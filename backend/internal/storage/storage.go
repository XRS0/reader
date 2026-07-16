package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/XRS0/reader/backend/internal/config"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

var ErrNotFound = errors.New("object not found")

type ObjectStore interface {
	EnsureBuckets(context.Context, ...string) error
	Ready(context.Context, string) error
	Put(context.Context, string, string, io.Reader, int64, string, map[string]string) error
	Get(context.Context, string, string, int64) ([]byte, error)
	Exists(context.Context, string, string) (bool, error)
	Delete(context.Context, string, string) error
	PresignGet(context.Context, string, string, time.Duration) (string, error)
}

type S3Store struct {
	client       *s3.Client
	uploader     *manager.Uploader
	presigner    *s3.PresignClient
	operationTTL time.Duration
	presignTTL   time.Duration
}

func NewS3(ctx context.Context, cfg config.S3) (*S3Store, error) {
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(cfg.Region), awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, "")), awsconfig.WithRetryMaxAttempts(cfg.MaxRetries))
	if err != nil {
		return nil, fmt.Errorf("load AWS configuration: %w", err)
	}
	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(strings.TrimRight(cfg.Endpoint, "/"))
		o.UsePathStyle = cfg.UsePathStyle
	})
	publicClient := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(strings.TrimRight(cfg.PublicEndpoint, "/"))
		o.UsePathStyle = cfg.UsePathStyle
	})
	return &S3Store{client: client, uploader: manager.NewUploader(client), presigner: s3.NewPresignClient(publicClient), operationTTL: cfg.OperationTTL, presignTTL: cfg.PresignTTL}, nil
}

func (s *S3Store) EnsureBuckets(ctx context.Context, buckets ...string) error {
	for _, bucket := range buckets {
		if !validBucket(bucket) {
			return fmt.Errorf("invalid bucket name %q", bucket)
		}
		opCtx, cancel := context.WithTimeout(ctx, s.operationTTL)
		_, err := s.client.HeadBucket(opCtx, &s3.HeadBucketInput{Bucket: aws.String(bucket)})
		cancel()
		if err == nil {
			continue
		}
		if !isNotFound(err) {
			return fmt.Errorf("head bucket %q: %w", bucket, err)
		}
		opCtx, cancel = context.WithTimeout(ctx, s.operationTTL)
		_, createErr := s.client.CreateBucket(opCtx, &s3.CreateBucketInput{Bucket: aws.String(bucket)})
		cancel()
		if createErr != nil {
			// API and worker may start together. If another process created the
			// bucket between HeadBucket and CreateBucket, a fresh HEAD confirms
			// that startup can safely continue.
			headCtx, headCancel := context.WithTimeout(ctx, s.operationTTL)
			_, headErr := s.client.HeadBucket(headCtx, &s3.HeadBucketInput{Bucket: aws.String(bucket)})
			headCancel()
			if headErr == nil {
				continue
			}
			return fmt.Errorf("ensure bucket %q: %w", bucket, createErr)
		}
	}
	return nil
}

func (s *S3Store) Ready(ctx context.Context, bucket string) error {
	if !validBucket(bucket) {
		return errors.New("invalid readiness bucket")
	}
	opCtx, cancel := context.WithTimeout(ctx, s.operationTTL)
	defer cancel()
	_, err := s.client.HeadBucket(opCtx, &s3.HeadBucketInput{Bucket: aws.String(bucket)})
	return err
}

func (s *S3Store) Put(ctx context.Context, bucket, key string, body io.Reader, size int64, contentType string, metadata map[string]string) error {
	if !validObject(bucket, key) {
		return errors.New("invalid object location")
	}
	if size < 0 {
		return errors.New("negative object size")
	}
	opCtx, cancel := context.WithTimeout(ctx, s.operationTTL)
	defer cancel()
	input := &s3.PutObjectInput{Bucket: aws.String(bucket), Key: aws.String(key), Body: body, ContentLength: aws.Int64(size), ContentType: aws.String(contentType), Metadata: metadata}
	if _, err := s.uploader.Upload(opCtx, input); err != nil {
		return fmt.Errorf("put s3://%s/%s: %w", bucket, key, err)
	}
	return nil
}

func (s *S3Store) Get(ctx context.Context, bucket, key string, maxBytes int64) (data []byte, err error) {
	if !validObject(bucket, key) || maxBytes <= 0 {
		return nil, errors.New("invalid object request")
	}
	opCtx, cancel := context.WithTimeout(ctx, s.operationTTL)
	defer cancel()
	out, err := s.client.GetObject(opCtx, &s3.GetObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)})
	if err != nil {
		if isNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get s3://%s/%s: %w", bucket, key, err)
	}
	defer func() {
		if closeErr := out.Body.Close(); err == nil && closeErr != nil {
			data = nil
			err = fmt.Errorf("close object body: %w", closeErr)
		}
	}()
	if out.ContentLength != nil && *out.ContentLength > maxBytes {
		return nil, fmt.Errorf("object exceeds %d bytes", maxBytes)
	}
	data, err = io.ReadAll(io.LimitReader(out.Body, maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read object: %w", err)
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("object exceeds %d bytes", maxBytes)
	}
	return data, nil
}

func (s *S3Store) Exists(ctx context.Context, bucket, key string) (bool, error) {
	if !validObject(bucket, key) {
		return false, errors.New("invalid object location")
	}
	opCtx, cancel := context.WithTimeout(ctx, s.operationTTL)
	defer cancel()
	_, err := s.client.HeadObject(opCtx, &s3.HeadObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)})
	if err != nil {
		if isNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
func (s *S3Store) Delete(ctx context.Context, bucket, key string) error {
	if !validObject(bucket, key) {
		return errors.New("invalid object location")
	}
	opCtx, cancel := context.WithTimeout(ctx, s.operationTTL)
	defer cancel()
	_, err := s.client.DeleteObject(opCtx, &s3.DeleteObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)})
	return err
}
func (s *S3Store) PresignGet(ctx context.Context, bucket, key string, ttl time.Duration) (string, error) {
	if !validObject(bucket, key) || ttl <= 0 {
		return "", errors.New("invalid presign request")
	}
	if s.presignTTL > 0 && ttl > s.presignTTL {
		ttl = s.presignTTL
	}
	opCtx, cancel := context.WithTimeout(ctx, s.operationTTL)
	defer cancel()
	out, err := s.presigner.PresignGetObject(opCtx, &s3.GetObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)}, func(o *s3.PresignOptions) { o.Expires = ttl })
	if err != nil {
		return "", err
	}
	if _, err := url.ParseRequestURI(out.URL); err != nil {
		return "", err
	}
	return out.URL, nil
}

func validBucket(v string) bool {
	return len(v) >= 3 && len(v) <= 63 && !strings.ContainsAny(v, "/\\\x00")
}
func validObject(bucket, key string) bool {
	if !validBucket(bucket) || key == "" || len(key) > 1024 || strings.HasPrefix(key, "/") || strings.ContainsRune(key, '\x00') {
		return false
	}
	for _, part := range strings.Split(strings.ReplaceAll(key, "\\", "/"), "/") {
		if part == ".." {
			return false
		}
	}
	return true
}
func isNotFound(err error) bool {
	v := strings.ToLower(err.Error())
	return strings.Contains(v, "notfound") || strings.Contains(v, "nosuchkey") || strings.Contains(v, "no such key") || strings.Contains(v, "status code: 404") || strings.Contains(v, "statuscode: 404")
}

// MemoryStore is intentionally limited to deterministic unit tests.
type MemoryStore struct{ Objects map[string][]byte }

func NewMemoryStore() *MemoryStore                                    { return &MemoryStore{Objects: map[string][]byte{}} }
func (m *MemoryStore) EnsureBuckets(context.Context, ...string) error { return nil }
func (m *MemoryStore) Ready(context.Context, string) error            { return nil }
func (m *MemoryStore) Put(_ context.Context, b, k string, r io.Reader, size int64, _ string, _ map[string]string) error {
	data, err := io.ReadAll(io.LimitReader(r, size+1))
	if err != nil {
		return err
	}
	if int64(len(data)) != size {
		return errors.New("size mismatch")
	}
	m.Objects[b+"/"+k] = bytes.Clone(data)
	return nil
}
func (m *MemoryStore) Get(_ context.Context, b, k string, max int64) ([]byte, error) {
	v, ok := m.Objects[b+"/"+k]
	if !ok {
		return nil, ErrNotFound
	}
	if int64(len(v)) > max {
		return nil, errors.New("too large")
	}
	return bytes.Clone(v), nil
}
func (m *MemoryStore) Exists(_ context.Context, b, k string) (bool, error) {
	_, ok := m.Objects[b+"/"+k]
	return ok, nil
}
func (m *MemoryStore) Delete(_ context.Context, b, k string) error {
	delete(m.Objects, b+"/"+k)
	return nil
}
func (m *MemoryStore) PresignGet(_ context.Context, b, k string, _ time.Duration) (string, error) {
	if _, ok := m.Objects[b+"/"+k]; !ok {
		return "", ErrNotFound
	}
	return "https://storage.invalid/" + b + "/" + url.PathEscape(k), nil
}
