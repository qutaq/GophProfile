package migrate_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/qutaq/GophProfile/internal/infra/migrate"
)

func TestUpInvalidDSN(t *testing.T) {
	err := migrate.Up("postgres://bad:bad@127.0.0.1:1/none?sslmode=disable")
	require.Error(t, err)
}

func TestVersionInvalidDSN(t *testing.T) {
	_, _, err := migrate.Version("postgres://bad:bad@127.0.0.1:1/none?sslmode=disable")
	require.Error(t, err)
}

func TestDownInvalidDSN(t *testing.T) {
	err := migrate.Down("postgres://bad:bad@127.0.0.1:1/none?sslmode=disable")
	require.Error(t, err)
}
