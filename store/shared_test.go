package store

import (
	"context"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetConfig(t *testing.T) {
	originalRegion := os.Getenv(RegionEnvVar)
	require.NoError(t, os.Setenv(RegionEnvVar, "us-west-2"))
	if originalRegion != "" {
		defer func() { assert.NoError(t, os.Setenv(RegionEnvVar, originalRegion)) }()
	} else {
		defer func() { assert.NoError(t, os.Unsetenv(RegionEnvVar)) }()
	}

	config, region, err := getConfig(context.Background(), 3, aws.RetryModeStandard)

	assert.NoError(t, err)
	assert.Equal(t, "us-west-2", region)

	assert.Equal(t, 3, config.RetryMaxAttempts)
	assert.Equal(t, aws.RetryModeStandard, config.RetryMode)
}
