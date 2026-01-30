// cmd/server/main.go
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/viper"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	"github.com/SyedDaiam9101/policy-service/internal/cache"
	"github.com/SyedDaiam9101/policy-service/internal/handler"
	"github.com/SyedDaiam9101/policy-service/internal/inference"
	"github.com/SyedDaiam9101/policy-service/internal/metrics"
	"github.com/SyedDaiam9101/policy-service/internal/middleware"
	pb "github.com/SyedDaiam9101/policy-service/proto/plannerpb"
)

const serviceName = "policy-service"

func main() {
	// Parse command-line flags
	port := flag.Int("port", 0, "gRPC server port (default: 50051)")
	modelPath := flag.String("model", "", "Path to ONNX model file (default: policy_cpu.onnx)")
	redisAddr := flag.String("redis", "", "Redis address (default: localhost:6379)")
	metricsPort := flag.Int("metrics", 0, "Prometheus metrics port (default: 9100)")
	configFile := flag.String("config", "", "Path to config file (optional)")
	useMock := flag.Bool("mock", false, "Use mock inference engine (for testing)")
	flag.Parse()

	// Load configuration from file and environment
	loadConfig(*configFile, *port, *modelPath, *redisAddr, *metricsPort, *useMock)

	// Read final configuration
	cfg := getConfig()

	log.Printf("Starting %s...", serviceName)
	log.Printf("Configuration: port=%d, model=%s, redis=%s, metrics=%d, otel=%v",
		cfg.Port, cfg.Model, cfg.Redis, cfg.MetricsPort, cfg.OTELEnabled)

	// Initialize OpenTelemetry tracer
	var tracerShutdown func(context.Context) error
	if cfg.OTELEnabled {
		var err error
		tracerShutdown, err = initTracer(cfg.OTELEndpoint)
		if err != nil {
			log.Printf("Warning: Failed to initialize tracer: %v", err)
		} else {
			log.Printf("OpenTelemetry tracing enabled (endpoint: %s)", cfg.OTELEndpoint)
		}
	}

	// Load inference engine
	var infer inference.InferenceEngine
	if cfg.UseMock {
		log.Printf("Using mock inference engine")
		infer = inference.NewMock()
	} else {
		log.Printf("Loading ONNX model from %s...", cfg.Model)
		var err error
		infer, err = inference.New(cfg.Model)
		if err != nil {
			log.Fatalf("Failed to load ONNX model: %v", err)
		}
		log.Printf("ONNX model loaded successfully")
	}
	defer infer.Close()

	// Initialize Redis cache (optional)
	var cacheClient *cache.Cache
	if cfg.Redis != "" {
		log.Printf("Connecting to Redis at %s...", cfg.Redis)
		var err error
		cacheClient, err = cache.New(cfg.Redis)
		if err != nil {
			log.Printf("Warning: Failed to connect to Redis: %v (continuing without cache)", err)
		} else {
			defer cacheClient.Close()
			log.Printf("Redis connected successfully")
		}
	}

	// Create gRPC health server
	healthServer := health.NewServer()

	// Start HTTP server for metrics and health checks
	httpServer := startHTTPServer(cfg.MetricsPort, healthServer)

	// Build interceptor chain
	interceptors := []grpc.UnaryServerInterceptor{
		middleware.UnaryRequestIDInterceptor(),
		middleware.UnaryMetricsInterceptor(),
	}

	// Add OpenTelemetry interceptor if enabled
	if cfg.OTELEnabled {
		interceptors = append(interceptors, otelgrpc.UnaryServerInterceptor())
	}

	// Create gRPC server with interceptors
	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(interceptors...),
	)

	// Register PathPlanner service
	h := handler.New(infer, cacheClient)
	pb.RegisterPathPlannerServer(grpcServer, h)

	// Register health service
	healthpb.RegisterHealthServer(grpcServer, healthServer)

	// Enable server reflection for debugging
	reflection.Register(grpcServer)

	// Start listening
	addr := fmt.Sprintf(":%d", cfg.Port)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Failed to listen on %s: %v", addr, err)
	}

	// Set health status to serving
	healthServer.SetServingStatus(serviceName, healthpb.HealthCheckResponse_SERVING)
	healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING) // Overall health
	metrics.SetHealthy()

	// Setup graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		log.Printf("Received signal %v, shutting down gracefully...", sig)

		// Set health to not serving
		healthServer.SetServingStatus(serviceName, healthpb.HealthCheckResponse_NOT_SERVING)
		healthServer.SetServingStatus("", healthpb.HealthCheckResponse_NOT_SERVING)
		metrics.SetUnhealthy()

		// Give time for load balancers to detect unhealthy status
		time.Sleep(5 * time.Second)

		// Shutdown gRPC server
		grpcServer.GracefulStop()

		// Shutdown HTTP server
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		httpServer.Shutdown(ctx)

		// Shutdown tracer
		if tracerShutdown != nil {
			tracerShutdown(ctx)
		}
	}()

	log.Printf("gRPC server listening on %s", addr)
	log.Printf("%s is ready to accept requests", serviceName)

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}

	log.Printf("Server shutdown complete")
}

// Config holds the merged configuration
type Config struct {
	Port        int
	MetricsPort int
	Model       string
	Redis       string
	OTELEnabled bool
	OTELEndpoint string
	UseMock     bool
}

func loadConfig(configFile string, port int, model, redis string, metricsPort int, useMock bool) {
	v := viper.GetViper()

	// Set defaults
	v.SetDefault("port", 50051)
	v.SetDefault("metrics_port", 9100)
	v.SetDefault("model", "policy_cpu.onnx")
	v.SetDefault("redis", "localhost:6379")
	v.SetDefault("otel_enabled", false)
	v.SetDefault("otel_endpoint", "")
	v.SetDefault("use_mock", false)

	// Environment variables
	v.SetEnvPrefix("POLICY_SERVICE")
	v.AutomaticEnv()

	// Check for OTEL standard env var
	if endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"); endpoint != "" {
		v.Set("otel_endpoint", endpoint)
		v.Set("otel_enabled", true)
	}

	// Config file
	if configFile != "" {
		v.SetConfigFile(configFile)
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath(".")
		v.AddConfigPath("/etc/policy-service/")
	}

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			log.Printf("Warning: Error reading config file: %v", err)
		}
	} else {
		log.Printf("Using config file: %s", v.ConfigFileUsed())
	}

	// Override with flags if provided
	if port > 0 {
		v.Set("port", port)
	}
	if model != "" {
		v.Set("model", model)
	}
	if redis != "" {
		v.Set("redis", redis)
	}
	if metricsPort > 0 {
		v.Set("metrics_port", metricsPort)
	}
	if useMock {
		v.Set("use_mock", true)
	}
}

func getConfig() Config {
	v := viper.GetViper()
	return Config{
		Port:         v.GetInt("port"),
		MetricsPort:  v.GetInt("metrics_port"),
		Model:        v.GetString("model"),
		Redis:        v.GetString("redis"),
		OTELEnabled:  v.GetBool("otel_enabled"),
		OTELEndpoint: v.GetString("otel_endpoint"),
		UseMock:      v.GetBool("use_mock"),
	}
}

func startHTTPServer(port int, healthServer *health.Server) *http.Server {
	mux := http.NewServeMux()

	// Prometheus metrics endpoint
	mux.Handle("/metrics", promhttp.Handler())

	// Health check endpoint
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		resp, err := healthServer.Check(r.Context(), &healthpb.HealthCheckRequest{})
		if err != nil || resp.Status != healthpb.HealthCheckResponse_SERVING {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("Service Unavailable"))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Readiness check (same as healthz for now)
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		resp, err := healthServer.Check(r.Context(), &healthpb.HealthCheckRequest{})
		if err != nil || resp.Status != healthpb.HealthCheckResponse_SERVING {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("Not Ready"))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Ready"))
	})

	addr := fmt.Sprintf(":%d", port)
	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	go func() {
		log.Printf("HTTP server listening on %s (metrics, health)", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	return server
}

func initTracer(endpoint string) (func(context.Context) error, error) {
	var exporter sdktrace.SpanExporter
	var err error

	if endpoint != "" {
		// For now, use stdout exporter as OTLP requires more setup
		// In production, use: otlptrace.New(ctx, otlptracegrpc.NewClient(...))
		log.Printf("Note: Using stdout trace exporter (OTLP endpoint: %s)", endpoint)
		exporter, err = stdouttrace.New(stdouttrace.WithPrettyPrint())
	} else {
		exporter, err = stdouttrace.New(stdouttrace.WithPrettyPrint())
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create trace exporter: %w", err)
	}

	// Create resource with service information
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion("1.0.0"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create tracer provider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)

	// Set global tracer provider
	otel.SetTracerProvider(tp)

	return tp.Shutdown, nil
}
