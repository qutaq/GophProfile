package rabbitmq

import (
	"context"
	"fmt"
	"log"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type MessageHandler func(ctx context.Context, body []byte, messageID string) error

type Consumer struct {
	conn     *amqp.Connection
	ch       *amqp.Channel
	exchange string
}

func NewConsumer(cfg Config) (*Consumer, error) {
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

	return &Consumer{conn: conn, ch: ch, exchange: cfg.Exchange}, nil
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

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case d, ok := <-deliveries:
				if !ok {
					return
				}
				c.handleDelivery(ctx, d, handler)
			}
		}
	}()

	return nil
}

const maxRetries = 5

func (c *Consumer) handleDelivery(ctx context.Context, d amqp.Delivery, handler MessageHandler) {
	msgID := d.MessageId
	if msgID == "" {
		msgID = fmt.Sprintf("generated-%d", time.Now().UnixNano())
	}

	err := retryWithBackoff(ctx, maxRetries, func() error {
		return handler(ctx, d.Body, msgID)
	})
	if err != nil {
		log.Printf("message %s failed after retries: %v", msgID, err)
		_ = d.Nack(false, false)
		return
	}
	_ = d.Ack(false)
}

func retryWithBackoff(ctx context.Context, attempts int, fn func() error) error {
	var err error
	for i := 0; i < attempts; i++ {
		if err = fn(); err == nil {
			return nil
		}
		if i == attempts-1 {
			break
		}
		backoff := time.Duration(1<<uint(i)) * 200 * time.Millisecond
		log.Printf("retry %d/%d after %s: %v", i+1, attempts, backoff, err)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
	}
	return err
}
