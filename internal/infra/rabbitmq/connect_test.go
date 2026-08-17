package rabbitmq_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/qutaq/GophProfile/internal/config"
	"github.com/qutaq/GophProfile/internal/infra/rabbitmq"
)

func TestConnectInvalidURL(t *testing.T) {
	_, err := rabbitmq.Connect(config.RabbitMQConfig{URL: "://bad"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "dial rabbitmq")
}

func TestDeclareExchangeInvalidURL(t *testing.T) {
	err := rabbitmq.DeclareExchange(config.RabbitMQConfig{
		URL:      "://bad",
		Exchange: "avatars.exchange",
	})
	require.Error(t, err)
}

func TestNewPublisherInvalidURL(t *testing.T) {
	_, err := rabbitmq.NewPublisher(rabbitmq.Config{
		URL:      "://bad",
		Exchange: "avatars.exchange",
	}, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "dial rabbitmq")
}

func TestNewConsumerInvalidURL(t *testing.T) {
	_, err := rabbitmq.NewConsumer(rabbitmq.Config{
		URL:      "://bad",
		Exchange: "avatars.exchange",
	}, nil, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "dial rabbitmq")
}
