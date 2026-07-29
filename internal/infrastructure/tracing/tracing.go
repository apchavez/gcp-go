// Package tracing wires OpenTelemetry to Cloud Trace - the GCP equivalent of the AWS
// sibling's X-Ray (see src/infra/tracing.ts's captureAWSClient wrapper) and the Azure
// sibling's Application Insights instrumentation (azure-monitor-opentelemetry). Same
// distributed-tracing story across all three clouds, different backend/exporter.
package tracing

import (
	"context"
	"fmt"

	texporter "github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.41.0"
)

// Init configures the global OpenTelemetry TracerProvider to export spans to Cloud Trace
// for the given GCP project, tagging every span with serviceName (api/worker/confirm) so
// the three Cloud Run services show up as distinct services in Cloud Trace, the same way
// each Lambda/Function does in its AWS/Azure sibling. Returns a shutdown func that flushes
// pending spans - callers should defer it.
func Init(ctx context.Context, gcpProjectID, serviceName string) (func(context.Context) error, error) {
	exporter, err := texporter.New(texporter.WithProjectID(gcpProjectID))
	if err != nil {
		return nil, fmt.Errorf("tracing: creating Cloud Trace exporter: %w", err)
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
			semconv.CloudProviderGCP,
		),
	)
	if err != nil {
		return nil, fmt.Errorf("tracing: building resource: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tp.Shutdown, nil
}

// tracer is shared by every resilience-guarded infra call (Firestore/Pub-Sub/Cloud SQL) -
// mirrors captureAWSClient's "instrument the client once, not each call site" approach.
// Safe to use before Init runs: the global TracerProvider defaults to a no-op implementation.
var tracer = otel.Tracer("github.com/apchavez/gcp-go")

// StartSpan starts a child span named after the infra component (e.g. "firestore-state-repo")
// making the call - used by internal/infrastructure/resilience.Resilience.Run.
func StartSpan(ctx context.Context, name string) (context.Context, func(err error)) {
	ctx, span := tracer.Start(ctx, name)
	return ctx, func(err error) {
		if err != nil {
			span.RecordError(err)
		}
		span.End()
	}
}
