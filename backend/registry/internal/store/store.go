// Package store defines the key-value surface used by the registry facade.
package store

import (
	"context"
	"fmt"
)

// EventType identifies a change delivered by a watch.
type EventType string

const (
	// EventPut reports a created or updated key.
	EventPut EventType = "put"
	// EventDelete reports a deleted key.
	EventDelete EventType = "delete"
	// EventReady signals that a watch stream is established.
	EventReady EventType = "ready"
)

// Event is a single key-value change delivered to a watcher.
type Event struct {
	Type  EventType `json:"type"`
	Key   string    `json:"key,omitempty"`
	Value string    `json:"value,omitempty"`
}

// Store is the KV surface used by the HTTP facade.
type Store interface {
	Put(ctx context.Context, key, value string) error
	Get(ctx context.Context, key string) (string, bool, error)
	Delete(ctx context.Context, key string) error
	List(ctx context.Context, prefix string) (map[string]string, error)
	PutWithLease(ctx context.Context, key, value string, ttlSeconds int64) (int64, error)
	KeepAliveOnce(ctx context.Context, leaseID int64) (int64, error)
	// LeaseOf returns the lease attached to a key and whether the key exists.
	// A key stored without a lease reports 0.
	LeaseOf(ctx context.Context, key string) (int64, bool, error)
	Watch(ctx context.Context, prefix string) (<-chan Event, error)
}

const rootPrefix = "/flashbacks"

// RootPrefix returns the namespace shared by all registry keys.
func RootPrefix() string {
	return rootPrefix + "/"
}

// ConfigPrefix returns the key prefix for a service's plaintext config.
func ConfigPrefix(service string) string {
	return fmt.Sprintf("%s/config/%s/", rootPrefix, service)
}

// ConfigKey returns the key for one configuration value.
func ConfigKey(service, key string) string {
	return ConfigPrefix(service) + key
}

// SecretsPrefix returns the key prefix for a service's encrypted secrets.
func SecretsPrefix(service string) string {
	return fmt.Sprintf("%s/secrets/%s/", rootPrefix, service)
}

// SecretKey returns the key for one encrypted secret.
func SecretKey(service, key string) string {
	return SecretsPrefix(service) + key
}

// ServicesPrefix returns the key prefix for a service's registrations.
func ServicesPrefix(service string) string {
	return fmt.Sprintf("%s/services/%s/", rootPrefix, service)
}

// ServiceKey returns the key for one service instance registration.
func ServiceKey(service, instance string) string {
	return ServicesPrefix(service) + instance
}
