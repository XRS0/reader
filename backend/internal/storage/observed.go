package storage

import (
	"context"
	"io"
	"time"
)

type ObserveFunc func(operation, result string, elapsed time.Duration)

type ObservedStore struct {
	inner   ObjectStore
	observe ObserveFunc
}

func NewObserved(inner ObjectStore, observe ObserveFunc) *ObservedStore {
	return &ObservedStore{inner: inner, observe: observe}
}

func (s *ObservedStore) record(operation string, started time.Time, err error) {
	result := "ok"
	if err != nil {
		result = "error"
	}
	s.observe(operation, result, time.Since(started))
}

func (s *ObservedStore) EnsureBuckets(ctx context.Context, buckets ...string) (err error) {
	started := time.Now()
	defer func() { s.record("ensure_buckets", started, err) }()
	return s.inner.EnsureBuckets(ctx, buckets...)
}

func (s *ObservedStore) Ready(ctx context.Context, bucket string) (err error) {
	started := time.Now()
	defer func() { s.record("ready", started, err) }()
	return s.inner.Ready(ctx, bucket)
}

func (s *ObservedStore) Put(ctx context.Context, bucket, key string, body io.Reader, size int64, contentType string, metadata map[string]string) (err error) {
	started := time.Now()
	defer func() { s.record("put", started, err) }()
	return s.inner.Put(ctx, bucket, key, body, size, contentType, metadata)
}

func (s *ObservedStore) Get(ctx context.Context, bucket, key string, max int64) (data []byte, err error) {
	started := time.Now()
	defer func() { s.record("get", started, err) }()
	return s.inner.Get(ctx, bucket, key, max)
}

func (s *ObservedStore) Exists(ctx context.Context, bucket, key string) (exists bool, err error) {
	started := time.Now()
	defer func() { s.record("exists", started, err) }()
	return s.inner.Exists(ctx, bucket, key)
}

func (s *ObservedStore) Delete(ctx context.Context, bucket, key string) (err error) {
	started := time.Now()
	defer func() { s.record("delete", started, err) }()
	return s.inner.Delete(ctx, bucket, key)
}

func (s *ObservedStore) PresignGet(ctx context.Context, bucket, key string, ttl time.Duration) (url string, err error) {
	started := time.Now()
	defer func() { s.record("presign_get", started, err) }()
	return s.inner.PresignGet(ctx, bucket, key, ttl)
}
