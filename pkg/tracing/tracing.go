package tracing

import (
	"context"
	"github.com/opentracing/opentracing-go"
	"github.com/uber/jaeger-client-go"
	jaegercfg "github.com/uber/jaeger-client-go/config"
	jaegerlog "github.com/uber/jaeger-client-go/log"
	"github.com/uber/jaeger-lib/metrics"
	"io"
)

func MakeSpanGet(ctx context.Context, name string) (context.Context, opentracing.Span) {
	tracer := opentracing.GlobalTracer()

	var parentCtx opentracing.SpanContext
	parentSpan := opentracing.SpanFromContext(ctx)
	if parentSpan != nil {
		parentCtx = parentSpan.Context()
	}
	span := tracer.StartSpan(
		name,
		opentracing.ChildOf(parentCtx),
	)
	ctx = opentracing.ContextWithSpan(ctx, span)
	return ctx, span
}

func InitJaeger() io.Closer {
	cfg := jaegercfg.Configuration{
		ServiceName: "Radio_Count",
		Sampler: &jaegercfg.SamplerConfig{
			Type:  jaeger.SamplerTypeConst,
			Param: 1,
		},
		Reporter: &jaegercfg.ReporterConfig{
			LogSpans: true,
		},
	}

	jLogger := jaegerlog.StdLogger
	jMetricsFactory := metrics.NullFactory

	tracer, closer, _ := cfg.NewTracer(
		jaegercfg.Logger(jLogger),
		jaegercfg.Metrics(jMetricsFactory),
	)

	opentracing.SetGlobalTracer(tracer)
	return closer
}
