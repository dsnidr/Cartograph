package render_test

import (
	"testing"

	"github.com/dsnidr/cartograph/internal/nbt"
	"github.com/dsnidr/cartograph/internal/registry"
	"github.com/dsnidr/cartograph/internal/render"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createPalette(name string) *nbt.PaletteContainer {
	return &nbt.PaletteContainer{
		Palette: []nbt.BlockState{{Name: name}},
	}
}

func TestRaycastColumn(t *testing.T) {
	tests := []struct {
		name           string
		sections       []nbt.Section
		expectedBlock  string
		expectedHeight int
		expectedBiome  string
	}{
		{
			name: "returns first solid block when raycast hits immediately",
			sections: []nbt.Section{
				{
					Y:           4,
					BlockStates: createPalette("minecraft:stone"),
					Biomes:      createPalette("minecraft:plains"),
				},
			},
			expectedBlock:  "minecraft:stone",
			expectedHeight: (4 * 16) + 15,
			expectedBiome:  "minecraft:plains",
		},
		{
			name: "skips transparent blocks and returns solid block below",
			sections: []nbt.Section{
				{
					Y:           1,
					BlockStates: createPalette("minecraft:air"),
				},
				{
					Y:           0,
					BlockStates: createPalette("minecraft:dirt"),
					Biomes:      createPalette("minecraft:desert"),
				},
			},
			expectedBlock:  "minecraft:dirt",
			expectedHeight: 15,
			expectedBiome:  "minecraft:desert",
		},
		{
			name: "skips empty sections entirely",
			sections: []nbt.Section{
				{
					Y:           2,
					BlockStates: nil,
				},
				{
					Y:           1,
					BlockStates: createPalette("minecraft:grass_block"),
					Biomes:      createPalette("minecraft:forest"),
				},
			},
			expectedBlock:  "minecraft:grass_block",
			expectedHeight: (1 * 16) + 15,
			expectedBiome:  "minecraft:forest",
		},
		{
			name: "returns default fallback biome when missing biome data",
			sections: []nbt.Section{
				{
					Y:           0,
					BlockStates: createPalette("minecraft:stone"),
					Biomes:      nil,
				},
			},
			expectedBlock:  "minecraft:stone",
			expectedHeight: 15,
			expectedBiome:  "minecraft:plains",
		},
		{
			name: "returns nil block and minY when falling into void",
			sections: []nbt.Section{
				{
					Y:           0,
					BlockStates: createPalette("minecraft:air"),
				},
			},
			expectedBlock:  "",
			expectedHeight: -64,
			expectedBiome:  "minecraft:plains",
		},
	}

	reg, err := registry.New(t.Context())
	require.NoError(t, err)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Precompute transparencies and water
			transparencies := make([][]bool, len(tt.sections))
			isWater := make([][]bool, len(tt.sections))
			for i, sec := range tt.sections {
				if sec.BlockStates != nil {
					trans := make([]bool, len(sec.BlockStates.Palette))
					water := make([]bool, len(sec.BlockStates.Palette))
					allTransparent := true
					for j, state := range sec.BlockStates.Palette {
						w := state.Name == "minecraft:water"
						water[j] = w
						
						t := reg.IsTransparent(state.Name) || w
						trans[j] = t
						if !t {
							allTransparent = false
						}
					}
					isWater[i] = water
					if !allTransparent {
						transparencies[i] = trans
					} else {
						transparencies[i] = nil
					}
				}
			}

			block, height, biome, _ := render.RaycastColumn(tt.sections, transparencies, isWater, 5, 5, -1)

			if tt.expectedBlock == "" {
				assert.Nil(t, block)
			} else {
				assert.NotNil(t, block)
				assert.Equal(t, tt.expectedBlock, block.Name)
			}

			assert.Equal(t, tt.expectedHeight, height)
			assert.Equal(t, tt.expectedBiome, biome)
		})
	}
}
