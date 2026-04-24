package registry

import (
	"bytes"
	"image/color"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseJSONL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		input           string
		wantColours     map[string]color.RGBA
		wantTransparent map[string]struct{}
		wantErr         bool
	}{
		{
			name: "valid entries with namespace context",
			input: `{"$namespace":"minecraft"}
{"stone":[125,125,125,255,0]}
{"glass":[255,255,255,255,1]}
{"$namespace":"create"}
{"cogwheel":[150,120,80,255,0]}`,
			wantColours: map[string]color.RGBA{
				"minecraft:stone": {125, 125, 125, 255},
				"minecraft:glass": {255, 255, 255, 255},
				"create:cogwheel": {150, 120, 80, 255},
			},
			wantTransparent: map[string]struct{}{
				"minecraft:glass": {},
			},
			wantErr: false,
		},
		{
			name:            "missing namespace prefix errors out",
			input:           `{"stone":[125,125,125,255,0]}`,
			wantColours:     nil,
			wantTransparent: nil,
			wantErr:         true,
		},
		{
			name: "empty lines",
			input: `{"$namespace":"minecraft"}

{"stone":[125,125,125,255,0]}

{"dirt":[134,96,67,255,0]}`,
			wantColours: map[string]color.RGBA{
				"minecraft:stone": {125, 125, 125, 255},
				"minecraft:dirt":  {134, 96, 67, 255},
			},
			wantTransparent: map[string]struct{}{},
			wantErr:         false,
		},
		{
			name:            "invalid json",
			input:           `{"$namespace":"minecraft"}\n{"stone":[125,125,125,255,0}`,
			wantColours:     nil,
			wantTransparent: nil,
			wantErr:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotColours, gotTransparent, err := parseJSONL(bytes.NewBufferString(tt.input))
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.wantColours, gotColours)
				require.Equal(t, tt.wantTransparent, gotTransparent)
			}
		})
	}
}

func TestLoadVanillaColours(t *testing.T) {
	// This test relies on vanilla_colours.jsonl actually existing in the directory
	// during test execution due to go:embed constraints.
	colours, transparent, err := LoadVanillaColours(t.Context())
	require.NoError(t, err)
	require.NotEmpty(t, colours)
	require.NotNil(t, transparent)

	// Spot-check some known blocks that should always be present
	require.Contains(t, colours, "minecraft:stone")
	require.Contains(t, colours, "minecraft:dirt")
	require.Contains(t, colours, "minecraft:water")
}

func TestParseBiomesJSONL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		input      string
		wantBiomes map[string]BiomeColour
		wantErr    bool
	}{
		{
			name: "valid entries with namespace context",
			input: `{"$namespace":"minecraft"}
{"plains":{"grass":[145,189,89],"foliage":[119,171,47],"water":[63,118,228]}}
{"$namespace":"mod"}
{"custom":{"grass":[10,20,30],"foliage":[40,50,60],"water":[70,80,90]}}`,
			wantBiomes: map[string]BiomeColour{
				"minecraft:plains": {
					Grass:   color.RGBA{145, 189, 89, 255},
					Foliage: color.RGBA{119, 171, 47, 255},
					Water:   color.RGBA{63, 118, 228, 255},
				},
				"mod:custom": {
					Grass:   color.RGBA{10, 20, 30, 255},
					Foliage: color.RGBA{40, 50, 60, 255},
					Water:   color.RGBA{70, 80, 90, 255},
				},
			},
			wantErr: false,
		},
		{
			name:       "missing namespace errors out",
			input:      `{"plains":{"grass":[1,2,3],"foliage":[4,5,6],"water":[7,8,9]}}`,
			wantBiomes: nil,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotBiomes, err := parseBiomesJSONL(bytes.NewBufferString(tt.input))
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.wantBiomes, gotBiomes)
			}
		})
	}
}

func TestLoadVanillaBiomes(t *testing.T) {
	biomes, err := LoadVanillaBiomes(t.Context())
	require.NoError(t, err)
	require.NotEmpty(t, biomes)

	// Spot-check some known biomes
	require.Contains(t, biomes, "minecraft:plains")
	require.Contains(t, biomes, "minecraft:desert")
}
