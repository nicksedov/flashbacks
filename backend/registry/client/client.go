// Package client is the Go client for the Flashbacks registry facade.
package client

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Instance describes a registered service instance.
type Instance struct {
	Host       string `json:"host"`
	Port       int    `json:"port"`
	BasePath   string `json:"basePath,omitempty"`
	HealthPath string `json:"healthPath,omitempty"`
	Version    string `json:"version,omitempty"`
}

// Event is a change delivered by a watch stream.
type Event struct {
	Type  string `json:"type"`
	Key   string `json:"key,omitempty"`
	Value string `json:"value,omitempty"`
}

// Config configures a registry client. Endpoint is the facade base URL.
type Config struct {
	Endpoint    string
	ServiceName string
	InstanceID  string
	Token       string
	HTTPClient  *http.Client
	TTLSeconds  int64
	Logger      *slog.Logger
}

// Client talks to the registry facade.
type Client struct {
	baseURL     string
	serviceName string
	instanceID  string
	token       string
	httpClient  *http.Client
	ttl         int64
	logger      *slog.Logger

	mu            sync.Mutex
	leaseID       int64
	stopHeartbeat chan struct{}
}

// New creates a registry client.
func New(cfg Config) (*Client, error) {
	endpoint := strings.TrimRight(strings.TrimSpace(cfg.Endpoint), "/")
	if endpoint == "" {
		return nil, fmt.Errorf("registry endpoint is empty")
	}
	if _, err := url.Parse(endpoint); err != nil {
		return nil, fmt.Errorf("parse registry endpoint: %w", err)
	}
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{}
	}
	ttl := cfg.TTLSeconds
	if ttl <= 0 {
		ttl = 15
	}
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &Client{
		baseURL:     endpoint,
		serviceName: cfg.ServiceName,
		instanceID:  cfg.InstanceID,
		token:       cfg.Token,
		httpClient:  httpClient,
		ttl:         ttl,
		logger:      logger,
	}, nil
}

// Endpoint returns the facade base URL.
func (c *Client) Endpoint() string {
	return c.baseURL
}

// GetConfig returns all configuration values for a service.
func (c *Client) GetConfig(ctx context.Context, service string) (map[string]string, error) {
	var resp struct {
		Values map[string]string `json:"values"`
	}
	if err := c.do(ctx, http.MethodGet, "/registry/v1/config/"+escape(service), nil, &resp); err != nil {
		return nil, err
	}
	if resp.Values == nil {
		resp.Values = map[string]string{}
	}
	return resp.Values, nil
}

// SetConfig writes one configuration value.
func (c *Client) SetConfig(ctx context.Context, service, key, value string) error {
	return c.do(ctx, http.MethodPut, "/registry/v1/config/"+escape(service)+"/"+escape(key), map[string]string{"value": value}, nil)
}

// GetConfigValue returns one configuration value.
func (c *Client) GetConfigValue(ctx context.Context, service, key string) (string, error) {
	var resp struct {
		Value string `json:"value"`
	}
	if err := c.do(ctx, http.MethodGet, "/registry/v1/config/"+escape(service)+"/"+escape(key), nil, &resp); err != nil {
		return "", err
	}
	return resp.Value, nil
}

// PutSecret writes an encrypted secret value.
func (c *Client) PutSecret(ctx context.Context, service, key, value string) error {
	return c.do(ctx, http.MethodPut, "/registry/v1/secrets/"+escape(service)+"/"+escape(key), map[string]string{"value": value}, nil)
}

// GetSecret reads and decrypts a secret value.
func (c *Client) GetSecret(ctx context.Context, service, key string) (string, error) {
	var resp struct {
		Value string `json:"value"`
	}
	if err := c.do(ctx, http.MethodGet, "/registry/v1/secrets/"+escape(service)+"/"+escape(key), nil, &resp); err != nil {
		return "", err
	}
	return resp.Value, nil
}

// Register registers this client's service instance and starts its heartbeat.
func (c *Client) Register(ctx context.Context, instance Instance) error {
	if c.serviceName == "" || c.instanceID == "" {
		return fmt.Errorf("service name and instance id are required for registration")
	}
	var resp struct {
		LeaseID int64 `json:"leaseId"`
		TTL     int64 `json:"ttl"`
	}
	path := fmt.Sprintf("/registry/v1/services/%s/%s?ttl=%d", escape(c.serviceName), escape(c.instanceID), c.ttl)
	if err := c.do(ctx, http.MethodPut, path, instance, &resp); err != nil {
		return err
	}

	c.mu.Lock()
	if c.stopHeartbeat != nil {
		close(c.stopHeartbeat)
	}
	stop := make(chan struct{})
	c.stopHeartbeat = stop
	c.leaseID = resp.LeaseID
	c.mu.Unlock()

	go c.heartbeat(stop, resp.LeaseID)
	return nil
}

// Deregister stops the heartbeat and removes the registration.
func (c *Client) Deregister(ctx context.Context) error {
	c.mu.Lock()
	if c.stopHeartbeat != nil {
		close(c.stopHeartbeat)
		c.stopHeartbeat = nil
	}
	c.mu.Unlock()
	return c.do(ctx, http.MethodDelete, fmt.Sprintf("/registry/v1/services/%s/%s", escape(c.serviceName), escape(c.instanceID)), nil, nil)
}

// Discover returns the live instances of a service.
func (c *Client) Discover(ctx context.Context, service string) ([]Instance, error) {
	var resp struct {
		Instances []struct {
			ID string `json:"id"`
			Instance
		} `json:"instances"`
	}
	if err := c.do(ctx, http.MethodGet, "/registry/v1/services/"+escape(service), nil, &resp); err != nil {
		return nil, err
	}
	instances := make([]Instance, 0, len(resp.Instances))
	for _, item := range resp.Instances {
		instances = append(instances, item.Instance)
	}
	return instances, nil
}

// ResolvePeer returns a discovered peer base URL, falling back to bootstrap.
func (c *Client) ResolvePeer(ctx context.Context, service, bootstrap string) string {
	instances, err := c.Discover(ctx, service)
	if err != nil {
		c.logger.Warn("registry discovery failed; using bootstrap URL", "service", service, "error", err)
		return bootstrap
	}
	if len(instances) == 0 {
		return bootstrap
	}
	host := instances[0].Host
	if host == "" {
		host = "localhost"
	}
	return fmt.Sprintf("http://%s:%d", host, instances[0].Port)
}

// ResolveConfig merges defaults, registry values, and explicit env overrides.
// Precedence is env override > registry value > built-in default.
func (c *Client) ResolveConfig(ctx context.Context, service string, defaults map[string]string, lookupEnv func(string) (string, bool)) map[string]string {
	resolved := make(map[string]string, len(defaults))
	for key, value := range defaults {
		resolved[key] = value
	}

	remote, err := c.GetConfig(ctx, service)
	if err != nil {
		c.logger.Warn("registry config unavailable; using defaults and env", "service", service, "error", err)
	} else {
		for key, value := range remote {
			resolved[key] = value
		}
	}

	if lookupEnv != nil {
		for key := range resolved {
			if value, ok := lookupEnv(key); ok {
				resolved[key] = value
			}
		}
	}
	return resolved
}

// WatchConfig streams configuration changes for a service.
func (c *Client) WatchConfig(ctx context.Context, service string, fn func(Event)) error {
	return c.watch(ctx, "/registry/v1/config/"+escape(service)+"/watch", fn)
}

// WatchInstances streams registration changes for a service.
func (c *Client) WatchInstances(ctx context.Context, service string, fn func(Event)) error {
	return c.watch(ctx, "/registry/v1/services/"+escape(service)+"/watch", fn)
}

func (c *Client) watch(ctx context.Context, path string, fn func(Event)) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("create watch request: %w", err)
	}
	req.Header.Set("Accept", "text/event-stream")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("open watch stream: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		payload, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return &HTTPError{StatusCode: resp.StatusCode, Body: strings.TrimSpace(string(payload))}
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var event Event
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &event); err != nil {
			continue
		}
		fn(event)
	}
	if err := scanner.Err(); err != nil && ctx.Err() == nil {
		return fmt.Errorf("read watch stream: %w", err)
	}
	return nil
}

func (c *Client) heartbeat(stop <-chan struct{}, leaseID int64) {
	interval := time.Duration(c.ttl) * time.Second / 3
	if interval < time.Second {
		interval = time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			err := c.do(ctx, http.MethodPost, fmt.Sprintf("/registry/v1/services/%s/%s/heartbeat", escape(c.serviceName), escape(c.instanceID)), map[string]int64{"leaseId": leaseID}, nil)
			cancel()
			if err != nil {
				c.logger.Warn("registry heartbeat failed", "service", c.serviceName, "error", err)
			}
		}
	}
}

func (c *Client) do(ctx context.Context, method, path string, body any, out any) error {
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
	}

	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		reader = bytes.NewReader(payload)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("registry request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		payload, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return &HTTPError{StatusCode: resp.StatusCode, Body: strings.TrimSpace(string(payload))}
	}
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}
	return nil
}

func escape(segment string) string {
	return url.PathEscape(segment)
}

// HTTPError is returned when the facade responds with a non-2xx status.
type HTTPError struct {
	StatusCode int
	Body       string
}

// Error implements error.
func (e *HTTPError) Error() string {
	return fmt.Sprintf("registry returned %d: %s", e.StatusCode, e.Body)
}
