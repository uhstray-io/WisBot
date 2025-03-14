package main

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// StartSpan creates a new span with the given name and returns the span and context
func StartSpan(ctx context.Context, name string) (context.Context, trace.Span) {
	return otel.Tracer("wisbot").Start(ctx, name)
}

// GetTracer returns a named tracer from the global provider
func GetTracer(name string) trace.Tracer {
	return otel.Tracer(name)
}

// GetMetric returns a named meter from the global provider
func GetMetric(name string) metric.Meter {
	return otel.Meter(name)
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
