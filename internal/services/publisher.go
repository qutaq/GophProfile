package services

import (
	"context"

	"github.com/qutaq/GophProfile/internal/events"
)

// EventPublisher publishes avatar lifecycle events to the message broker.
type EventPublisher interface {
	PublishUpload(ctx context.Context, event events.AvatarUploadEvent) error
	PublishDelete(ctx context.Context, event events.AvatarDeleteEvent) error
}

// NoopPublisher is used in tests when broker is not needed.
type NoopPublisher struct{}

func (NoopPublisher) PublishUpload(context.Context, events.AvatarUploadEvent) error {
	return nil
}

func (NoopPublisher) PublishDelete(context.Context, events.AvatarDeleteEvent) error {
	return nil
}
