package registry_test

import (
	"image/color"
	"testing"

	"github.com/dsnidr/cartograph/internal/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegistry_GetColour(t *testing.T) {
	ctx := t.Context()
	reg, err := registry.New(ctx)
	require.NoError(t, err)

	t.Run("returns pre-baked colour if exists", func(t *testing.T) {
		// Register a manual override to be sure
		testColour := color.RGBA{1, 2, 3, 255}
		reg.RegisterColour("minecraft:test_block", testColour, registry.TintNone)

		entry, found := reg.GetColour("minecraft:test_block")
		assert.True(t, found)
		assert.Equal(t, testColour, entry.Base)
		assert.Equal(t, registry.TintNone, entry.Tint)
	})

	t.Run("generates fallback for unknown block", func(t *testing.T) {
		entry, found := reg.GetColour("mod:unknown_block")
		assert.False(t, found)
		assert.NotEqual(t, color.RGBA{}, entry.Base)
		assert.Equal(t, uint8(255), entry.Base.A)
	})

	t.Run("caches generated fallback", func(t *testing.T) {
		blockID := "mod:cached_block"
		entry1, found1 := reg.GetColour(blockID)
		assert.False(t, found1)

		entry2, found2 := reg.GetColour(blockID)
		assert.True(t, found2) // should be in cache now
		assert.Equal(t, entry1, entry2)
	})

	t.Run("applies tint heuristics", func(t *testing.T) {
		tests := []struct {
			id   string
			tint registry.TintType
		}{
			{"minecraft:oak_leaves", registry.TintFoliage},
			{"minecraft:grass_block", registry.TintGrass},
			{"minecraft:water", registry.TintWater},
			{"minecraft:stone", registry.TintNone},
		}

		for _, tt := range tests {
			entry, _ := reg.GetColour(tt.id)
			assert.Equal(t, tt.tint, entry.Tint, "incorrect tint for %s", tt.id)
		}
	})

	t.Run("returns true for transparent blocks", func(t *testing.T) {
		assert.True(t, reg.IsTransparent("minecraft:air"))
		assert.True(t, reg.IsTransparent("minecraft:glass"))
		assert.False(t, reg.IsTransparent("minecraft:oak_sapling"))
		assert.False(t, reg.IsTransparent("minecraft:stone"))
		assert.False(t, reg.IsTransparent("minecraft:oak_leaves"))
	})
}
