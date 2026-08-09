//go:build tools

package tools

// Tooling and runtime dependencies pinned for the project.
// Install/update with: go mod tidy
import (
	_ "github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/minio/minio-go/v7"
	_ "github.com/rabbitmq/amqp091-go"
)
