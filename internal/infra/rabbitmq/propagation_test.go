package rabbitmq

import (
	"context"
	"testing"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"

	"github.com/qutaq/GophProfile/internal/observability"
)

func TestAMQPHeaderCarrierInjectExtract(t *testing.T) {
	rec, cleanup := observability.NewTestTracer()
	t.Cleanup(cleanup)

	ctx, parent := observability.Tracer().Start(context.Background(), "parent")
	headers := amqp.Table{"x-message-key": "avatar-1"}
	otel.GetTextMapPropagator().Inject(ctx, amqpHeaderCarrier(headers))
	parent.End()

	raw, ok := headers["traceparent"].(string)
	require.True(t, ok)
	require.Contains(t, raw, parent.SpanContext().TraceID().String())

	extracted := otel.GetTextMapPropagator().Extract(context.Background(), amqpHeaderCarrier(headers))
	_, child := observability.StartSpanKind(extracted, "rabbitmq.consume", trace.SpanKindConsumer)
	child.End()

	consume := observability.EndedSpan(rec, "rabbitmq.consume")
	require.NotNil(t, consume)
	require.Equal(t, parent.SpanContext().TraceID(), consume.SpanContext().TraceID())
	require.Equal(t, parent.SpanContext().SpanID(), consume.Parent().SpanID())
}

func TestAMQPHeaderCarrierGetSetKeys(t *testing.T) {
	require.Equal(t, "", amqpHeaderCarrier(nil).Get("traceparent"))
	amqpHeaderCarrier(nil).Set("traceparent", "x")

	h := amqpHeaderCarrier{"traceparent": []byte("from-bytes"), "other": 1}
	require.Equal(t, "from-bytes", h.Get("traceparent"))
	require.Equal(t, "1", h.Get("other"))
	require.Equal(t, "", h.Get("missing"))

	h.Set("tracestate", "vendor=1")
	require.Equal(t, "vendor=1", h.Get("tracestate"))
	require.ElementsMatch(t, []string{"traceparent", "other", "tracestate"}, h.Keys())
}
