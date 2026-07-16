package observability

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/XRS0/reader/backend/internal/config"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/uptrace/bun"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.30.0"
)

type Metrics struct {
	Registry            *prometheus.Registry
	HTTPDuration        *prometheus.HistogramVec
	HTTPRequests        *prometheus.CounterVec
	DBDuration          *prometheus.HistogramVec
	StorageDuration     *prometheus.HistogramVec
	WorkerJobs          *prometheus.CounterVec
	ActiveSessions      prometheus.Gauge
	QueueDepth          prometheus.Gauge
	TranslationDuration *prometheus.HistogramVec
	TranslationCache    *prometheus.CounterVec
}

func NewMetrics() *Metrics {
	m := &Metrics{
		Registry: prometheus.NewRegistry(),
		HTTPDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name: "bookflow_http_request_duration_seconds",
				Help: "HTTP request duration.",
			},
			[]string{"method", "route", "status"},
		),
		HTTPRequests: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "bookflow_http_requests_total",
				Help: "HTTP request count.",
			},
			[]string{"method", "route", "status"},
		),
		DBDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name: "bookflow_database_query_duration_seconds",
				Help: "Database query duration.",
			},
			[]string{"operation"},
		),
		StorageDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name: "bookflow_storage_operation_duration_seconds",
				Help: "Object storage duration.",
			},
			[]string{"operation", "result"},
		),
		WorkerJobs: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "bookflow_worker_jobs_total",
				Help: "Worker job results.",
			},
			[]string{"type", "result"},
		),
		ActiveSessions: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "bookflow_reading_sessions_active",
			Help: "Active reading sessions.",
		}),
		QueueDepth: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "bookflow_jobs_queue_depth",
			Help: "Queued jobs.",
		}),
		TranslationDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name: "bookflow_translation_duration_seconds",
				Help: "Translation request duration.",
			},
			[]string{"type", "result"},
		),
		TranslationCache: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "bookflow_translation_cache_total",
				Help: "Translation cache outcomes.",
			},
			[]string{"result"},
		),
	}
	m.Registry.MustRegister(
		m.HTTPDuration,
		m.HTTPRequests,
		m.DBDuration,
		m.StorageDuration,
		m.WorkerJobs,
		m.ActiveSessions,
		m.QueueDepth,
		m.TranslationDuration,
		m.TranslationCache,
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
	return m
}

func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.Registry, promhttp.HandlerOpts{})
}

type queryStartedKey struct{}
type BunHook struct{ Metrics *Metrics }

func (h BunHook) BeforeQuery(ctx context.Context, _ *bun.QueryEvent) context.Context {
	return context.WithValue(ctx, queryStartedKey{}, time.Now())
}

func (h BunHook) AfterQuery(ctx context.Context, event *bun.QueryEvent) {
	start, ok := ctx.Value(queryStartedKey{}).(time.Time)
	if !ok {
		return
	}
	op := strings.ToLower(event.Operation())
	h.Metrics.DBDuration.WithLabelValues(op).Observe(time.Since(start).Seconds())
}

func NewLogger(environment string) *slog.Logger {
	level := slog.LevelInfo
	switch strings.ToLower(os.Getenv("LOG_LEVEL")) {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	opts := &slog.HandlerOptions{Level: level}
	var handler slog.Handler
	if strings.EqualFold(os.Getenv("LOG_FORMAT"), "text") {
		handler = slog.NewTextHandler(os.Stdout, opts)
	} else {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}
	return slog.New(handler).With("environment", environment)
}

func SetupTracing(ctx context.Context, c config.Config) (func(context.Context) error, error) {
	if c.Observability.OTLPEndpoint == "" || strings.EqualFold(os.Getenv("OTEL_ENABLED"), "false") {
		provider := sdktrace.NewTracerProvider()
		otel.SetTracerProvider(provider)
		return provider.Shutdown, nil
	}
	opts := []otlptracehttp.Option{otlptracehttp.WithEndpointURL(c.Observability.OTLPEndpoint)}
	if c.Observability.OTELInsecure {
		opts = append(opts, otlptracehttp.WithInsecure())
	}
	exporter, err := otlptracehttp.New(ctx, opts...)
	if err != nil {
		return nil, err
	}
	res, err := resource.Merge(
		resource.Default(),
		resource.NewSchemaless(
			semconv.ServiceName(c.ServiceName),
			semconv.DeploymentEnvironmentName(c.Environment),
		),
	)
	if err != nil {
		return nil, err
	}
	ratio := c.Observability.TraceRatio
	if ratio < 0 || ratio > 1 {
		ratio = .1
	}
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(ratio))),
	)
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	return provider.Shutdown, nil
}

func DiscardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
