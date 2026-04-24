package pipeline_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dsnidr/cartograph/internal/pipeline"
)

func TestDiscoverRegions(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "region-discovery-test")
	require.NoError(t, err, "failed to create temp dir")
	defer os.RemoveAll(tempDir)

	fileNames := []string{
		"r.0.0.mca",
		"r.-1.2.mca",
		"r.3.-4.mca",
		"not_a_region.txt",
	}

	for _, name := range fileNames {
		path := filepath.Join(tempDir, name)
		err := os.WriteFile(path, []byte("dummy data"), 0644)
		require.NoError(t, err, "failed to create dummy file")
	}

	t.Run("returns valid jobs and bounds", func(t *testing.T) {
		ctx := t.Context()
		jobs, total, bounds, err := pipeline.DiscoverRegions(ctx, tempDir)
		
		require.NoError(t, err)
		assert.Equal(t, 3, total)

		assert.Equal(t, -1, bounds.MinX)
		assert.Equal(t, 3, bounds.MaxX)
		assert.Equal(t, -4, bounds.MinZ)
		assert.Equal(t, 2, bounds.MaxZ)

		jobCount := 0
		for range jobs {
			jobCount++
		}
		assert.Equal(t, 3, jobCount)
	})

	t.Run("fails when context is canceled", func(t *testing.T) {
		canceledCtx, cancel := context.WithCancel(t.Context())
		cancel()

		jobs, total, _, err := pipeline.DiscoverRegions(canceledCtx, tempDir)
		
		require.Error(t, err)
		assert.ErrorIs(t, err, context.Canceled)
		assert.Equal(t, 0, total)

		jobCount := 0
		for range jobs {
			jobCount++
		}
		assert.Equal(t, 0, jobCount)
	})
}
