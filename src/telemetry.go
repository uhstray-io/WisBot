package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.opentelemetry.io/otel/trace"

	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"

	sdkLog "go.opentelemetry.io/otel/sdk/log"
	sdkMetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdkTrace "go.opentelemetry.io/otel/sdk/trace"
)

// StartOTelService bootstraps the OpenTelemetry pipeline.
// If it does not return an error, make sure to call shutdown for proper cleanup.
func StartOTelService(ctx context.Context) {
	if !otelServiceEnabled {
		fmt.Println("OpenTelemetry service is disabled. Skipping initialization.")
		return
	}

	var shutdownFuncs []func(context.Context) error

	// shutdown calls cleanup functions registered via shutdownFuncs.
	// The errors from the calls are joined.
	// Each registered cleanup will be invoked once.
	shutdownHandler := func(ctx context.Context) error {
		var err error
		for _, fn := range shutdownFuncs {
			err = errors.Join(err, fn(ctx))
		}
		shutdownFuncs = nil
		return err
	}

	// Ensure cleanup at the end
	defer func() {
		err := shutdownHandler(ctx)
		if err != nil {
			fmt.Printf("Error shutting down OpenTelemetry: %v\n", err)
		}
	}()

	// handleErr calls shutdown for cleanup and makes sure that all errors are returned.
	handleErr := func(inErr error) {
		errors.Join(inErr, shutdownHandler(ctx))
	}

	// Set up propagator.
	prop := newPropagator()
	otel.SetTextMapPropagator(prop)

	// Set up trace provider.
	tracerProvider, err := newTraceProvider(ctx)
	if err != nil {
		handleErr(err)
		return
	}
	shutdownFuncs = append(shutdownFuncs, tracerProvider.Shutdown)
	otel.SetTracerProvider(tracerProvider)

	// Set up meter provider.
	meterProvider, err := newMeterProvider(ctx)
	if err != nil {
		handleErr(err)
		return
	}
	shutdownFuncs = append(shutdownFuncs, meterProvider.Shutdown)
	otel.SetMeterProvider(meterProvider)

	// Set up logger provider.
	loggerProvider, err := newLoggerProvider(ctx)
	if err != nil {
		handleErr(err)
		return
	}
	shutdownFuncs = append(shutdownFuncs, loggerProvider.Shutdown)
	global.SetLoggerProvider(loggerProvider)

}

// newPropagator creates a new propagator for trace context and baggage.
// We use the default TraceContext and Baggage propagators from OpenTelemetry.
func newPropagator() propagation.TextMapPropagator {
	return propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)
}

func newTraceProvider(ctx context.Context) (*sdkTrace.TracerProvider, error) {
	// stdoutTraceExporter, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
	// if err != nil {
	// 	return nil, err
	// }

	// Create OTLP trace exporter
	traceExporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(otelExporterOtlpEndpoint),
		otlptracegrpc.WithInsecure(), // For development; use TLS in production
	)
	if err != nil {
		return nil, err
	}

	traceProvider := sdkTrace.NewTracerProvider(
		// sdkTrace.WithSampler(sdkTrace.AlwaysSample()),
		sdkTrace.WithBatcher(traceExporter,
			sdkTrace.WithBatchTimeout(time.Second)),
		sdkTrace.WithResource(newResource()),
	)

	return traceProvider, nil
}

func newMeterProvider(ctx context.Context) (*sdkMetric.MeterProvider, error) {
	// stdoutMetricExporter, err := stdoutmetric.New(stdoutmetric.WithPrettyPrint())
	// if err != nil {
	// 	return nil, err
	// }

	// Create OTLP metric exporter
	metricExporter, err := otlpmetricgrpc.New(ctx,
		otlpmetricgrpc.WithEndpoint(otelExporterOtlpEndpoint),
		otlpmetricgrpc.WithInsecure(), // For development; use TLS in production
	)
	if err != nil {
		return nil, err
	}

	meterProvider := sdkMetric.NewMeterProvider(
		sdkMetric.WithReader(
			sdkMetric.NewPeriodicReader(metricExporter, sdkMetric.WithInterval(10*time.Second))),
		sdkMetric.WithResource(newResource()),
	)
	return meterProvider, nil
}

func newLoggerProvider(ctx context.Context) (*sdkLog.LoggerProvider, error) {
	// Use this for development purposes to log to stdout
	// stdoutLogExporter, err := stdoutlog.New(stdoutlog.WithPrettyPrint())
	// if err != nil {
	// 	return nil, err
	// }

	// Create OTLP log exporter
	logExporter, err := otlploggrpc.New(ctx,
		otlploggrpc.WithEndpoint(otelExporterOtlpEndpoint),
		otlploggrpc.WithInsecure(), // For development; use TLS in production
	)
	if err != nil {
		return nil, err
	}

	loggerProvider := sdkLog.NewLoggerProvider(
		sdkLog.WithProcessor(sdkLog.NewBatchProcessor(logExporter)),
		sdkLog.WithResource(newResource()),
	)
	return loggerProvider, nil
}

// newResource creates a resource with identifying information about this application.
// The purpose of this resource is to provide context for the telemetry data being sent.
// It includes the service name and deployment environment.
func newResource() *resource.Resource {
	// Parse deployment environment from OTEL_RESOURCE_ATTRIBUTES
	env := "development"
	if otelResourceAttrs != "" {
		parts := strings.Split(otelResourceAttrs, "=")
		if len(parts) >= 2 {
			env = parts[1]
		}
	}

	res, _ := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(otelServiceName),
			attribute.String("deployment.environment", env),
		),
	)
	return res
}

// StartSpan creates a new span with the given name and returns the span and context
func StartSpan(ctx context.Context, name string) (context.Context, trace.Span) {
	return otel.Tracer("wisbot").Start(ctx, name)
}

// LogEvent logs an event with the given severity and message, associating it with the current span
// This is a better approach than mixing log and span APIs directly
func LogEvent(ctx context.Context, severity log.Severity, message string, attrs ...attribute.KeyValue) {
	// Get the current span from context
	span := trace.SpanFromContext(ctx)

	// Create a logger
	logger := global.Logger("wisbot")

	// Create the log record
	record := log.Record{}
	record.SetTimestamp(time.Now())
	record.SetObservedTimestamp(time.Now())
	record.SetSeverity(severity)
	record.SetBody(log.StringValue(message))

	// Add the span context as an attribute to correlate logs with spans
	if span.SpanContext().IsValid() {
		record.AddAttributes(
			log.KeyValueFromAttribute(attribute.String("trace_id", span.SpanContext().TraceID().String())),
			log.KeyValueFromAttribute(attribute.String("span_id", span.SpanContext().SpanID().String())))
	}

	// Add user provided attributes
	for _, attr := range attrs {
		record.AddAttributes(log.KeyValueFromAttribute(attr))
	}

	// Also add events to the span itself for better correlation
	if span.IsRecording() {
		// Convert log attributes to trace attributes
		span.AddEvent(message, trace.WithAttributes())
	}

	// Emit the log
	logger.Emit(ctx, record)
}

// LogError logs an error and also records it on the current span
func LogError(ctx context.Context, err error, message string, attrs ...attribute.KeyValue) {
	if err == nil {
		return
	}

	span := trace.SpanFromContext(ctx)

	// Record error on the span
	span.RecordError(err)

	// Create error attributes
	errorAttrs := append(attrs, attribute.String("error", err.Error()))

	// Log the error
	LogEvent(ctx, log.SeverityError, fmt.Sprintf("%s: %v", message, err), errorAttrs...)
}
