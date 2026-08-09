package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/qutaq/GophProfile/internal/events"
)

type Publisher struct {
	conn     *amqp.Connection
	ch       *amqp.Channel
	exchange string
}

func NewPublisher(cfg Config) (*Publisher, error) {
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

	return &Publisher{conn: conn, ch: ch, exchange: cfg.Exchange}, nil
}

type Config struct {
	URL      string
	Exchange string
}

func (p *Publisher) Close() error {
	var err error
	if p.ch != nil {
		err = p.ch.Close()
	}
	if p.conn != nil {
		if cerr := p.conn.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}
	return err
}

func (p *Publisher) PublishUpload(ctx context.Context, event events.AvatarUploadEvent) error {
	return p.publish(ctx, events.RoutingKeyUploaded, event.AvatarID, event)
}

func (p *Publisher) PublishDelete(ctx context.Context, event events.AvatarDeleteEvent) error {
	return p.publish(ctx, events.RoutingKeyDeleted, event.AvatarID, event)
}

func (p *Publisher) publish(ctx context.Context, routingKey, messageKey string, body any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	msgID := uuid.NewString()
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(1<<uint(attempt-1)) * 100 * time.Millisecond):
			}
		}

		err = p.ch.PublishWithContext(ctx,
			p.exchange,
			routingKey,
			false,
			false,
			amqp.Publishing{
				ContentType:  "application/json",
				DeliveryMode: amqp.Persistent,
				MessageId:    msgID,
				Timestamp:    time.Now().UTC(),
				Type:         routingKey,
				Body:         payload,
				Headers: amqp.Table{
					"x-message-key": messageKey,
				},
			},
		)
		if err == nil {
			return nil
		}
		lastErr = err
	}
	return fmt.Errorf("publish %s: %w", routingKey, lastErr)
}

func declareTopology(ch *amqp.Channel, exchange string) error {
	if err := ch.ExchangeDeclare(exchange, "topic", true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare exchange: %w", err)
	}

	queues := []struct {
		name       string
		routingKey string
	}{
		{events.QueueUpload, events.RoutingKeyUploaded},
		{events.QueueDelete, events.RoutingKeyDeleted},
	}

	for _, q := range queues {
		if _, err := ch.QueueDeclare(q.name, true, false, false, false, nil); err != nil {
			return fmt.Errorf("declare queue %s: %w", q.name, err)
		}
		if err := ch.QueueBind(q.name, q.routingKey, exchange, false, nil); err != nil {
			return fmt.Errorf("bind queue %s: %w", q.name, err)
		}
	}
	return nil
}
