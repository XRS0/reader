//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/XRS0/reader/backend/internal/config"
	"github.com/XRS0/reader/backend/internal/storage"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestRustFSS3ObjectLifecycle(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "rustfs/rustfs:1.0.0-beta.4",
			ExposedPorts: []string{"9000/tcp"},
			Env: map[string]string{
				"RUSTFS_VOLUMES":        "/data",
				"RUSTFS_ADDRESS":        "0.0.0.0:9000",
				"RUSTFS_CONSOLE_ENABLE": "false",
				"RUSTFS_ACCESS_KEY":     "bookflow-integration-access",
				"RUSTFS_SECRET_KEY":     "bookflow-integration-secret-at-least-32-characters",
			},
			WaitingFor: wait.ForHTTP("/health").
				WithPort("9000/tcp").
				WithStartupTimeout(90 * time.Second),
		},
		Started: true,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		terminateCtx, terminateCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer terminateCancel()
		require.NoError(t, container.Terminate(terminateCtx, testcontainers.StopTimeout(time.Second)))
	})

	host, err := container.Host(ctx)
	require.NoError(t, err)
	port, err := container.MappedPort(ctx, "9000/tcp")
	require.NoError(t, err)
	endpoint := fmt.Sprintf("http://%s:%s", host, port.Port())
	store, err := storage.NewS3(ctx, config.S3{
		Endpoint:       endpoint,
		PublicEndpoint: endpoint,
		Region:         "us-east-1",
		AccessKey:      "bookflow-integration-access",
		SecretKey:      "bookflow-integration-secret-at-least-32-characters",
		UsePathStyle:   true,
		OperationTTL:   15 * time.Second,
		PresignTTL:     5 * time.Minute,
		MaxRetries:     3,
	})
	require.NoError(t, err)

	buckets := []string{"books-original", "books-content", "books-assets", "books-covers", "user-exports"}
	require.NoError(t, store.EnsureBuckets(ctx, buckets...))
	require.NoError(t, store.EnsureBuckets(ctx, buckets...), "bucket initialization must be idempotent")
	for _, bucket := range buckets {
		require.NoError(t, store.Ready(ctx, bucket))
	}

	payload := []byte("BookFlow RustFS integration payload")
	key := "users/integration/books/example/original/content.txt"
	require.NoError(t, store.Put(
		ctx,
		"books-original",
		key,
		bytes.NewReader(payload),
		int64(len(payload)),
		"text/plain; charset=utf-8",
		map[string]string{"sha256": "integration-fixture"},
	))

	exists, err := store.Exists(ctx, "books-original", key)
	require.NoError(t, err)
	require.True(t, exists)
	downloaded, err := store.Get(ctx, "books-original", key, int64(len(payload)))
	require.NoError(t, err)
	require.Equal(t, payload, downloaded)
	_, err = store.Get(ctx, "books-original", key, int64(len(payload)-1))
	require.Error(t, err, "bounded reads must reject oversized objects")

	presigned, err := store.PresignGet(ctx, "books-original", key, time.Hour)
	require.NoError(t, err)
	parsed, err := url.Parse(presigned)
	require.NoError(t, err)
	require.Equal(t, host, parsed.Hostname())
	require.Equal(t, "/books-original/"+key, parsed.Path)
	require.NotEmpty(t, parsed.Query().Get("X-Amz-Signature"))
	require.NotEmpty(t, parsed.Query().Get("X-Amz-Expires"))
	require.Equal(t, "300", parsed.Query().Get("X-Amz-Expires"), "presigned lifetime must be capped by configured PresignTTL")

	require.NoError(t, store.Delete(ctx, "books-original", key))
	exists, err = store.Exists(ctx, "books-original", key)
	require.NoError(t, err)
	require.False(t, exists)
	_, err = store.Get(ctx, "books-original", key, int64(len(payload)))
	require.True(t, errors.Is(err, storage.ErrNotFound), "deleted objects must map to storage.ErrNotFound: %v", err)
}
