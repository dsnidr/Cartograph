package pipeline_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dsnidr/cartograph/internal/nbt"
	"github.com/dsnidr/cartograph/internal/pipeline"
	"github.com/dsnidr/cartograph/internal/registry"
	"github.com/dsnidr/cartograph/internal/render"
)

func TestNewOrchestrator(t *testing.T) {
	t.Run("returns initialized orchestrator", func(t *testing.T) {
		sp := nbt.NewStringPool()
		ctx := context.Background()
		outDir := "test-out"
		scale := 1
		mode := pipeline.OutputModeTiles
		renderer := render.NewRenderer(render.RendererConfig{})

		br, err := registry.New(ctx)
		require.NoError(t, err)

		orc := pipeline.NewOrchestrator(sp, br, outDir, scale, mode, renderer)
		require.NotNil(t, orc)

		assert.Same(t, sp, orc.StringPool)
		assert.Same(t, br, orc.BlockRegistry)
		assert.Equal(t, outDir, orc.OutputDir)
		assert.Equal(t, scale, orc.Scale)
		assert.Equal(t, mode, orc.OutputMode)
		assert.Same(t, renderer, orc.Renderer)
	})
}

func TestStartWorkers(t *testing.T) {
	t.Run("returns early when context is canceled", func(t *testing.T) {
		sp := nbt.NewStringPool()
		br, err := registry.New(context.Background())
		require.NoError(t, err)
		renderer := render.NewRenderer(render.RendererConfig{})
		orc := pipeline.NewOrchestrator(sp, br, "out", 0, pipeline.OutputModeTiles, renderer)

		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		jobs := make(chan pipeline.Job, 1)
		jobs <- pipeline.Job{RegionPath: "dummy.mca"}
		close(jobs)

		results := orc.StartWorkers(ctx, 1, jobs)

		resultCount := 0
		for range results {
			resultCount++
		}

		assert.Equal(t, 0, resultCount)
	})
}
