package main

import (
	"context"
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

// StartOTelService sets up OpenTelemetry tracing, metrics, and logging
func StartOTelService(ctx context.Context) {
	// Set up propagator
	prop := newPropagator()
	otel.SetTextMapPropagator(prop)

	// Setup Providers

	// Set up logger providers
	loggerProvider, err := newLoggerProvider(ctx)
	if err != nil {
		LogError(ctx, err, "Failed to create logger provider")
		return
	}
	global.SetLoggerProvider(loggerProvider)
	LogInfo(ctx, "Logger provider initialized successfully")

	// Set up trace provider
	tracerProvider, err := newTraceProvider(ctx)
	if err != nil {
		LogError(ctx, err, "Failed to create trace provider")
		return
	}
	otel.SetTracerProvider(tracerProvider)
	LogInfo(ctx, "Trace provider initialized successfully")

	// Set up meter provider
	meterProvider, err := newMeterProvider(ctx)
	if err != nil {
		LogError(ctx, err, "Failed to create meter provider")
		return
	}
	otel.SetMeterProvider(meterProvider)
	LogInfo(ctx, "Meter provider initialized successfully")
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
		span.AddEvent(message, trace.WithAttributes())
	}

	printRecordAndSpan(record)

	// Emit the log
	logger.Emit(ctx, record)
}

func printRecordAndSpan(record log.Record) {
	fmt.Printf("%v [%v] %s\n",
		record.Timestamp().Format("2006-01-02 15:04:05"),
		record.Severity().String(),
		record.Body().AsString(),
	)

	record.WalkAttributes(func(kv log.KeyValue) bool {
		if kv.Key == "trace_id" || kv.Key == "span_id" {
			return true
		}
		fmt.Printf("\t%v\n", kv.String())
		return true
	})
}

func LogInfo(ctx context.Context, message string, attrs ...attribute.KeyValue) {
	LogEvent(ctx, log.SeverityInfo, message, attrs...)
}

func LogWarning(ctx context.Context, message string, attrs ...attribute.KeyValue) {
	LogEvent(ctx, log.SeverityWarn, message, attrs...)
}

// LogError logs an error and also records it on the current span
func LogError(ctx context.Context, err error, message string, attrs ...attribute.KeyValue) {
	if err == nil {
		return
	}

	span := trace.SpanFromContext(ctx)
	span.RecordError(err)
	errorAttrs := append(attrs, attribute.String("error", err.Error()))

	LogEvent(ctx, log.SeverityError, fmt.Sprintf("%s: %v", message, err), errorAttrs...)
}

// PanicError logs an error and panics.
func PanicError(ctx context.Context, err error, message string, attrs ...attribute.KeyValue) {
	if err == nil {
		return
	}

	LogError(ctx, err, message, attrs...)
	panic(fmt.Sprintf("%s: %v", message, err))
}
