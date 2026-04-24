package render

import (
	"image/color"
	"testing"

	"github.com/dsnidr/cartograph/internal/nbt"
	"github.com/dsnidr/cartograph/internal/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderer_resolveColour(t *testing.T) {
	renderer := NewRenderer(RendererConfig{DebugMissingBlocks: true})
	ctx := t.Context()
	br, err := registry.New(ctx)
	require.NoError(t, err)

	// Inject deterministic mock colours for the tests so they don't depend on the real embedded JSONL.
	br.RegisterColour("minecraft:stone", color.RGBA{R: 125, G: 125, B: 125, A: 255}, registry.TintNone)
	br.RegisterColour("minecraft:water", color.RGBA{R: 63, G: 118, B: 228, A: 255}, registry.TintWater)
	br.RegisterColour("minecraft:grass_block", color.RGBA{R: 145, G: 189, B: 89, A: 255}, registry.TintGrass)

	tests := []struct {
		name      string
		block     *nbt.BlockState
		biomeName string
		expected  color.RGBA
	}{
		{
			name: "returns direct mapped colour from registry",
			block: &nbt.BlockState{
				Name: "minecraft:stone",
			},
			biomeName: "minecraft:plains",
			expected:  color.RGBA{R: 125, G: 125, B: 125, A: 255},
		},
		{
			name: "overrides colour with water tint when block is waterlogged",
			block: &nbt.BlockState{
				Name: "minecraft:oak_stairs",
				Properties: map[string]string{
					"waterlogged": "true",
				},
			},
			biomeName: "minecraft:plains",
			// Should return biome-tinted water colour, not oak wood
			expected: br.ApplyBiomeTint(color.RGBA{R: 63, G: 118, B: 228, A: 255}, "minecraft:plains", registry.TintWater),
		},
		{
			name: "applies grass tinting based on biome",
			block: &nbt.BlockState{
				Name: "minecraft:grass_block",
			},
			biomeName: "minecraft:desert",
			// Grass base colour tinted for desert
			expected: br.ApplyBiomeTint(color.RGBA{R: 145, G: 189, B: 89, A: 255}, "minecraft:desert", registry.TintGrass),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hit := &BlockHit{
				BlockName: tt.block.Name,
				BiomeName: tt.biomeName,
			}
			if tt.block.HasProperty("waterlogged", "true") {
				hit.IsWaterlogged = true
			}
			result := renderer.resolveColour(hit, br)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRenderer_renderDownsampled(t *testing.T) {
	t.Run("averages block colours while skipping nil states", func(t *testing.T) {
		renderer := NewRenderer(RendererConfig{})
		ctx := t.Context()
		br, err := registry.New(ctx)
		require.NoError(t, err)

		// Inject deterministic mock colours for the downsampling math check.
		br.RegisterColour("minecraft:stone", color.RGBA{R: 125, G: 125, B: 125, A: 255}, registry.TintNone)
		br.RegisterColour("minecraft:dirt", color.RGBA{R: 150, G: 108, B: 74, A: 255}, registry.TintNone)

		surface := new(SurfaceGrid)

		// Set up a 2x2 area to test 2:1 downsampling
		// Block 1: Stone (125, 125, 125)
		surface[0][0] = BlockHit{
			BlockName: "minecraft:stone",
			Y:         10,
			BiomeName: "minecraft:plains",
			HasBlock:  true,
		}

		// Block 2: Dirt (150, 108, 74)
		surface[1][0] = BlockHit{
			BlockName: "minecraft:dirt",
			Y:         10,
			BiomeName: "minecraft:plains",
			HasBlock:  true,
		}

		// Block 3: Stone (125, 125, 125)
		surface[0][1] = BlockHit{
			BlockName: "minecraft:stone",
			Y:         10,
			BiomeName: "minecraft:plains",
			HasBlock:  true,
		}

		// Block 4: zero-value (representing void / air)
		// surface[1][1] is implicitly zero-value (HasBlock = false)

		// Run 2:1 downsampling
		img, _ := renderer.ExtractBaseSurface(surface, br, Scale2x2)

		// Get the single output pixel that represents this 2x2 area
		outColour := img.RGBAAt(0, 0)

		// Calculate expected average of the 3 solid blocks, completely ignoring the nil block
		// R: (125 + 150 + 125) / 3 = 400 / 3 = 133
		// G: (125 + 108 + 125) / 3 = 358 / 3 = 119
		// B: (125 + 74 + 125) / 3 = 324 / 3 = 108
		// A: 255
		expected := color.RGBA{R: 133, G: 119, B: 108, A: 255}

		assert.Equal(t, expected, outColour)
	})

	t.Run("returns transparent pixel when all blocks in area are nil", func(t *testing.T) {
		renderer := NewRenderer(RendererConfig{})
		ctx := t.Context()
		surface := new(SurfaceGrid)

		br, err := registry.New(ctx)
		require.NoError(t, err)

		// Run 2:1 downsampling on entirely empty grid
		img, _ := renderer.ExtractBaseSurface(surface, br, Scale2x2)

		outColour := img.RGBAAt(0, 0)
		expected := color.RGBA{0, 0, 0, 0}

		assert.Equal(t, expected, outColour)
	})
}
