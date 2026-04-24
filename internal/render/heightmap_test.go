package render_test

import (
	"testing"

	"github.com/dsnidr/cartograph/internal/render"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildHeightmapFromSurface(t *testing.T) {
	t.Run("converts surface grid into accurate global heightmap", func(t *testing.T) {
		surface := new(render.SurfaceGrid)

		// Set a known solid block
		surface[0][0] = render.BlockHit{
			BlockName: "minecraft:stone",
			Y:         64,
			HasBlock:  true,
		}

		// Set another solid block at a different height
		surface[10][20] = render.BlockHit{
			BlockName: "minecraft:dirt",
			Y:         128,
			HasBlock:  true,
		}

		// Note: The rest of the surface is initialized with zero-values
		// where State == nil.

		hm := render.BuildHeightmapFromSurface(surface)
		require.NotNil(t, hm)

		// Assert populated coordinates copy the Y value correctly
		assert.Equal(t, 64, hm[0][0])
		assert.Equal(t, 128, hm[10][20])

		// Assert unpopulated/void coordinates fall back to -64 (minY)
		assert.Equal(t, -64, hm[1][1])
		assert.Equal(t, -64, hm[511][511])
	})
}
