package store

import (
	"context"
	"fmt"

	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// EtcdStore implements Store on top of an etcd client.
type EtcdStore struct {
	client *clientv3.Client
}

// NewEtcdStore wraps an etcd client.
func NewEtcdStore(client *clientv3.Client) *EtcdStore {
	return &EtcdStore{client: client}
}

// Put stores a value.
func (s *EtcdStore) Put(ctx context.Context, key, value string) error {
	if _, err := s.client.Put(ctx, key, value); err != nil {
		return fmt.Errorf("put %q: %w", key, err)
	}
	return nil
}

// Get returns a value and whether the key exists.
func (s *EtcdStore) Get(ctx context.Context, key string) (string, bool, error) {
	resp, err := s.client.Get(ctx, key)
	if err != nil {
		return "", false, fmt.Errorf("get %q: %w", key, err)
	}
	if len(resp.Kvs) == 0 {
		return "", false, nil
	}
	return string(resp.Kvs[0].Value), true, nil
}

// Delete removes a key.
func (s *EtcdStore) Delete(ctx context.Context, key string) error {
	if _, err := s.client.Delete(ctx, key); err != nil {
		return fmt.Errorf("delete %q: %w", key, err)
	}
	return nil
}

// List returns every key-value pair under a prefix.
func (s *EtcdStore) List(ctx context.Context, prefix string) (map[string]string, error) {
	resp, err := s.client.Get(ctx, prefix, clientv3.WithPrefix())
	if err != nil {
		return nil, fmt.Errorf("list %q: %w", prefix, err)
	}
	values := make(map[string]string, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		values[string(kv.Key)] = string(kv.Value)
	}
	return values, nil
}

// PutWithLease stores a value attached to a new lease and returns its ID.
func (s *EtcdStore) PutWithLease(ctx context.Context, key, value string, ttlSeconds int64) (int64, error) {
	if ttlSeconds <= 0 {
		ttlSeconds = 15
	}
	lease, err := s.client.Grant(ctx, ttlSeconds)
	if err != nil {
		return 0, fmt.Errorf("grant lease: %w", err)
	}
	if _, err := s.client.Put(ctx, key, value, clientv3.WithLease(lease.ID)); err != nil {
		_, _ = s.client.Revoke(context.Background(), lease.ID)
		return 0, fmt.Errorf("put with lease: %w", err)
	}
	return int64(lease.ID), nil
}

// KeepAliveOnce refreshes a lease and returns its new TTL.
func (s *EtcdStore) KeepAliveOnce(ctx context.Context, leaseID int64) (int64, error) {
	resp, err := s.client.KeepAliveOnce(ctx, clientv3.LeaseID(leaseID))
	if err != nil {
		return 0, fmt.Errorf("keepalive lease %d: %w", leaseID, err)
	}
	return resp.TTL, nil
}

// Watch streams events for a prefix until the context is cancelled.
func (s *EtcdStore) Watch(ctx context.Context, prefix string) (<-chan Event, error) {
	watchChan := s.client.Watch(ctx, prefix, clientv3.WithPrefix(), clientv3.WithCreatedNotify())
	events := make(chan Event)
	go func() {
		defer close(events)
		for response := range watchChan {
			if err := response.Err(); err != nil {
				return
			}
			if response.Created {
				select {
				case events <- Event{Type: EventReady}:
				case <-ctx.Done():
					return
				}
				continue
			}
			for _, event := range response.Events {
				var eventType EventType
				switch event.Type {
				case mvccpb.PUT:
					eventType = EventPut
				case mvccpb.DELETE:
					eventType = EventDelete
				default:
					continue
				}
				select {
				case events <- Event{Type: eventType, Key: string(event.Kv.Key), Value: string(event.Kv.Value)}:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	return events, nil
}
