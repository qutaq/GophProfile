package rabbitmq

import (
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/qutaq/GophProfile/internal/config"
)

func Connect(cfg config.RabbitMQConfig) (*amqp.Connection, error) {
	conn, err := amqp.Dial(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("dial rabbitmq: %w", err)
	}
	return conn, nil
}

// DeclareExchange ensures exchange and queues used by the service exist.
func DeclareExchange(cfg config.RabbitMQConfig) error {
	conn, err := Connect(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("open channel: %w", err)
	}
	defer ch.Close()

	return declareTopology(ch, cfg.Exchange)
}
