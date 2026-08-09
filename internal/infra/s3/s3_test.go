package s3_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/qutaq/GophProfile/internal/config"
	infras3 "github.com/qutaq/GophProfile/internal/infra/s3"
)

func TestNewClient(t *testing.T) {
	client, err := infras3.NewClient(config.S3Config{
		Endpoint:        "localhost:9000",
		AccessKeyID:     "minioadmin",
		SecretAccessKey: "minioadmin",
		Bucket:          "avatars",
		UseSSL:          false,
		Region:          "us-east-1",
	})
	require.NoError(t, err)
	require.NotNil(t, client)
}

func TestEnsureBucketAndListBucketsUnreachable(t *testing.T) {
	client, err := infras3.NewClient(config.S3Config{
		Endpoint:        "127.0.0.1:1",
		AccessKeyID:     "minioadmin",
		SecretAccessKey: "minioadmin",
		Bucket:          "avatars",
		UseSSL:          false,
		Region:          "us-east-1",
	})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err = infras3.EnsureBucket(ctx, client, "avatars")
	require.Error(t, err)
	require.Contains(t, err.Error(), "check bucket")

	_, err = infras3.ListBuckets(ctx, client)
	require.Error(t, err)
	require.Contains(t, err.Error(), "list buckets")
}
