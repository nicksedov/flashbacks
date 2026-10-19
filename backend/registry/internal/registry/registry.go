// Package registry starts and owns the embedded etcd instance.
package registry

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"go.etcd.io/etcd/client/pkg/v3/transport"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/server/v3/embed"

	"github.com/flashbacks/registry/internal/config"
)

// Embedded wraps a running single-node etcd instance and its client.
type Embedded struct {
	etcd   *embed.Etcd
	client *clientv3.Client
	url    string
}

// Start launches the embedded etcd server and returns a connected client.
func Start(ctx context.Context, cfg config.Config) (*Embedded, error) {
	if err := os.MkdirAll(cfg.DataDir, 0o700); err != nil {
		return nil, fmt.Errorf("create registry data dir: %w", err)
	}

	clientURL, err := url.Parse(cfg.ClientURL)
	if err != nil {
		return nil, fmt.Errorf("parse client url: %w", err)
	}
	peerURL, err := url.Parse(cfg.PeerURL)
	if err != nil {
		return nil, fmt.Errorf("parse peer url: %w", err)
	}

	econf := embed.NewConfig()
	econf.Dir = cfg.DataDir
	econf.LogLevel = "error"
	econf.Logger = "zap"
	econf.Name = "default"
	econf.ListenClientUrls = []url.URL{*clientURL}
	econf.AdvertiseClientUrls = []url.URL{*clientURL}
	econf.ListenPeerUrls = []url.URL{*peerURL}
	econf.AdvertisePeerUrls = []url.URL{*peerURL}
	econf.InitialCluster = econf.Name + "=" + cfg.PeerURL

	var tlsInfo *transport.TLSInfo
	if cfg.TLSCertFile != "" {
		info := transport.TLSInfo{
			CertFile:      cfg.TLSCertFile,
			KeyFile:       cfg.TLSKeyFile,
			TrustedCAFile: cfg.TLSCAFile,
		}
		econf.ClientTLSInfo = info
		econf.PeerTLSInfo = info
		tlsInfo = &info
	}

	etcd, err := embed.StartEtcd(econf)
	if err != nil {
		return nil, fmt.Errorf("start embedded etcd: %w", err)
	}

	select {
	case <-etcd.Server.ReadyNotify():
	case <-time.After(cfg.ReadyTimeout):
		etcd.Server.Stop()
		return nil, fmt.Errorf("embedded etcd not ready within %s", cfg.ReadyTimeout)
	case <-ctx.Done():
		etcd.Server.Stop()
		return nil, ctx.Err()
	}

	clientConfig := clientv3.Config{
		Endpoints:   []string{cfg.ClientURL},
		DialTimeout: 5 * time.Second,
	}
	if tlsInfo != nil {
		tlsConfig, err := tlsInfo.ClientConfig()
		if err != nil {
			etcd.Close()
			return nil, fmt.Errorf("build etcd client TLS config: %w", err)
		}
		clientConfig.TLS = tlsConfig
	}

	cli, err := clientv3.New(clientConfig)
	if err != nil {
		etcd.Close()
		return nil, fmt.Errorf("connect embedded etcd: %w", err)
	}

	if cfg.AuthEnabled {
		if err := enableAuth(ctx, cli, cfg.RootPassword); err != nil {
			_ = cli.Close()
			etcd.Close()
			return nil, err
		}
		_ = cli.Close()

		authenticated := clientConfig
		authenticated.Username = "root"
		authenticated.Password = cfg.RootPassword
		cli, err = clientv3.New(authenticated)
		if err != nil {
			etcd.Close()
			return nil, fmt.Errorf("connect embedded etcd with auth: %w", err)
		}
	}

	return &Embedded{etcd: etcd, client: cli, url: cfg.ClientURL}, nil
}

// Client returns the etcd client.
func (e *Embedded) Client() *clientv3.Client {
	return e.client
}

// URL returns the etcd client endpoint.
func (e *Embedded) URL() string {
	return e.url
}

// Close stops the client and the embedded server.
func (e *Embedded) Close() error {
	var firstErr error
	if e.client != nil {
		if err := e.client.Close(); err != nil {
			firstErr = err
		}
	}
	if e.etcd != nil {
		e.etcd.Close()
	}
	return firstErr
}

func enableAuth(ctx context.Context, cli *clientv3.Client, password string) error {
	status, err := cli.AuthStatus(ctx)
	if err == nil && status.Enabled {
		return nil
	}
	if _, err := cli.UserAdd(ctx, "root", password); err != nil && !alreadyExists(err) {
		return fmt.Errorf("create etcd root user: %w", err)
	}
	if _, err := cli.RoleAdd(ctx, "root"); err != nil && !alreadyExists(err) {
		return fmt.Errorf("create etcd root role: %w", err)
	}
	if _, err := cli.UserGrantRole(ctx, "root", "root"); err != nil {
		return fmt.Errorf("grant etcd root role: %w", err)
	}
	if _, err := cli.AuthEnable(ctx); err != nil && !alreadyExists(err) {
		return fmt.Errorf("enable etcd auth: %w", err)
	}
	return nil
}

func alreadyExists(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "already exists")
}
