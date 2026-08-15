package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/qutaq/GophProfile/internal/observability"
)

type MessageHandler func(ctx context.Context, body []byte, messageID string) error

type Consumer struct {
	conn     *amqp.Connection
	ch       *amqp.Channel
	exchange string
	log      *slog.Logger
}

func NewConsumer(cfg Config, logger *slog.Logger) (*Consumer, error) {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	logger = logger.With("component", "rabbitmq-consumer")

	conn, err := amqp.Dial(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("dial rabbitmq: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("open channel: %w", err)
	}

	if err := declareTopology(ch, cfg.Exchange); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, err
	}

	if err := ch.Qos(1, 0, false); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, fmt.Errorf("qos: %w", err)
	}

	return &Consumer{conn: conn, ch: ch, exchange: cfg.Exchange, log: logger}, nil
}

func (c *Consumer) Close() error {
	var err error
	if c.ch != nil {
		err = c.ch.Close()
	}
	if c.conn != nil {
		if cerr := c.conn.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}
	return err
}

func (c *Consumer) Consume(ctx context.Context, queue string, handler MessageHandler) error {
	deliveries, err := c.ch.Consume(queue, "", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("consume %s: %w", queue, err)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case d, ok := <-deliveries:
			if !ok {
				return nil
			}
			// Finish the in-flight message even after shutdown is requested.
			c.handleDelivery(context.WithoutCancel(ctx), queue, d, handler)
		}
	}
}

const maxRetries = 5

func (c *Consumer) handleDelivery(ctx context.Context, queue string, d amqp.Delivery, handler MessageHandler) {
	msgID := d.MessageId
	if msgID == "" {
		msgID = fmt.Sprintf("generated-%d", time.Now().UnixNano())
	}

	headers := d.Headers
	if headers == nil {
		headers = amqp.Table{}
	}
	ctx = otel.GetTextMapPropagator().Extract(ctx, amqpHeaderCarrier(headers))

	ctx, span := observability.StartSpanKind(ctx, "rabbitmq.consume", trace.SpanKindConsumer,
		attribute.String("messaging.system", "rabbitmq"),
		attribute.String("messaging.destination", queue),
		attribute.String("messaging.operation", "consume"),
		attribute.String("messaging.message_id", msgID),
	)
	defer span.End()

	observability.ConsumerInFlightInc()
	defer observability.ConsumerInFlightDec()

	err := retryWithBackoff(ctx, c.log, maxRetries, func() error {
		return handler(ctx, d.Body, msgID)
	})
	if err != nil {
		observability.RecordError(span, err)
		observability.ObserveConsume(queue, observability.StatusError)
		attrs := append([]any{"queue", queue, "message_id", msgID, "err", err}, eventLogAttrs(d.Body)...)
		c.log.ErrorContext(ctx, "message failed", attrs...)
		_ = d.Nack(false, false)
		return
	}
	observability.ObserveConsume(queue, observability.StatusSuccess)
	_ = d.Ack(false)
}

func retryWithBackoff(ctx context.Context, logger *slog.Logger, attempts int, fn func() error) error {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	var err error
	for i := 0; i < attempts; i++ {
		if err = fn(); err == nil {
			return nil
		}
		if i == attempts-1 {
			break
		}
		backoff := time.Duration(1<<uint(i)) * 200 * time.Millisecond
		logger.WarnContext(ctx, "retrying message handler",
			"attempt", i+1,
			"max_attempts", attempts,
			"backoff", backoff.String(),
			"err", err,
		)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
	}
	return err
}

func eventLogAttrs(body []byte) []any {
	var meta struct {
		AvatarID string `json:"avatar_id"`
		UserID   string `json:"user_id"`
	}
	if json.Unmarshal(body, &meta) != nil {
		return nil
	}
	attrs := make([]any, 0, 4)
	if meta.AvatarID != "" {
		attrs = append(attrs, "avatar_id", meta.AvatarID)
	}
	if meta.UserID != "" {
		attrs = append(attrs, "user_id", meta.UserID)
	}
	return attrs
}
