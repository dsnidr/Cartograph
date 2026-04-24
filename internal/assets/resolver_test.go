package assets_test

import (
	"encoding/json"
	"image"
	"image/color"
	png "github.com/dsnidr/cartograph/internal/fastpng"
	"os"
	"path/filepath"
	"testing"

	"github.com/dsnidr/cartograph/internal/assets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResourceLocation(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected assets.ResourceLocation
		bsPath   string
		mPath    string
		tPath    string
	}{
		{
			name:  "parses namespaced location",
			input: "minecraft:stone",
			expected: assets.ResourceLocation{
				Namespace: "minecraft",
				Path:      "stone",
			},
			bsPath: "assets/minecraft/blockstates/stone.json",
			mPath:  "assets/minecraft/models/stone.json",
			tPath:  "assets/minecraft/textures/stone.png",
		},
		{
			name:  "defaults to minecraft namespace",
			input: "dirt",
			expected: assets.ResourceLocation{
				Namespace: "minecraft",
				Path:      "dirt",
			},
			bsPath: "assets/minecraft/blockstates/dirt.json",
			mPath:  "assets/minecraft/models/dirt.json",
			tPath:  "assets/minecraft/textures/dirt.png",
		},
		{
			name:  "handles custom namespaces",
			input: "create:block/cogwheel",
			expected: assets.ResourceLocation{
				Namespace: "create",
				Path:      "block/cogwheel",
			},
			bsPath: "assets/create/blockstates/block/cogwheel.json",
			mPath:  "assets/create/models/block/cogwheel.json",
			tPath:  "assets/create/textures/block/cogwheel.png",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loc := assets.ParseResourceLocation(tt.input)
			assert.Equal(t, tt.expected, loc)
			assert.Equal(t, tt.bsPath, loc.BlockstatePath())
			assert.Equal(t, tt.mPath, loc.ModelPath())
			assert.Equal(t, tt.tPath, loc.TexturePath())
		})
	}
}

func TestBlockStateVariant_UnmarshalJSON(t *testing.T) {
	t.Run("returns model from single object", func(t *testing.T) {
		data := []byte(`{"model": "minecraft:block/stone"}`)
		var v assets.BlockStateVariant
		err := json.Unmarshal(data, &v)
		require.NoError(t, err)
		assert.Equal(t, "minecraft:block/stone", v.Model)
	})

	t.Run("returns first model from array", func(t *testing.T) {
		data := []byte(`[{"model": "minecraft:block/stone_alt"}, {"model": "minecraft:block/stone"}]`)
		var v assets.BlockStateVariant
		err := json.Unmarshal(data, &v)
		require.NoError(t, err)
		assert.Equal(t, "minecraft:block/stone_alt", v.Model)
	})
}

func TestResolver_Resolve(t *testing.T) {
	// Setup a temporary directory with a mock asset structure
	tmpDir := t.TempDir()

	// 1. Create a blockstate
	bsDir := filepath.Join(tmpDir, "assets/minecraft/blockstates")
	require.NoError(t, os.MkdirAll(bsDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(bsDir, "oak_log.json"), []byte(`{
		"variants": {
			"axis=y": { "model": "minecraft:block/oak_log" }
		}
	}`), 0644))

	// 2. Create models (inheritance: oak_log -> cube_column)
	modelDir := filepath.Join(tmpDir, "assets/minecraft/models/block")
	require.NoError(t, os.MkdirAll(modelDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(modelDir, "oak_log.json"), []byte(`{
		"parent": "minecraft:block/cube_column",
		"textures": {
			"end": "minecraft:block/oak_log_top",
			"side": "minecraft:block/oak_log"
		}
	}`), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(modelDir, "cube_column.json"), []byte(`{
		"textures": {
			"up": "#end",
			"down": "#end"
		}
	}`), 0644))

	// 3. Create a texture
	texDir := filepath.Join(tmpDir, "assets/minecraft/textures/block")
	require.NoError(t, os.MkdirAll(texDir, 0755))

	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{255, 0, 0, 255})

	f, err := os.Create(filepath.Join(texDir, "oak_log_top.png"))
	require.NoError(t, err)
	require.NoError(t, png.Encode(f, img))
	f.Close()

	// 4. Resolve
	source := assets.NewDirectorySource(tmpDir)
	resolver := assets.NewResolver(source)
	ctx := t.Context()

	col, transparent, err := resolver.Resolve(ctx, "minecraft:oak_log")
	require.NoError(t, err)
	assert.Equal(t, uint8(255), col.R)
	assert.Equal(t, uint8(0), col.G)
	assert.Equal(t, uint8(0), col.B)
	assert.False(t, transparent)
}

func TestResolver_InfiniteRecursion(t *testing.T) {
	tmpDir := t.TempDir()
	modelDir := filepath.Join(tmpDir, "assets/minecraft/models/block")
	require.NoError(t, os.MkdirAll(modelDir, 0755))

	// Create a cycle: #a -> #b, #b -> #a
	require.NoError(t, os.WriteFile(filepath.Join(modelDir, "recursive.json"), []byte(`{
		"textures": {
			"top": "#a",
			"a": "#b",
			"b": "#a"
		}
	}`), 0644))

	// Need a blockstate to point to it
	bsDir := filepath.Join(tmpDir, "assets/minecraft/blockstates")
	require.NoError(t, os.MkdirAll(bsDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(bsDir, "recursive.json"), []byte(`{
		"variants": {
			"": { "model": "minecraft:block/recursive" }
		}
	}`), 0644))

	source := assets.NewDirectorySource(tmpDir)
	resolver := assets.NewResolver(source)
	ctx := t.Context()

	_, _, err := resolver.Resolve(ctx, "minecraft:recursive")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "less than 10 variable redirections")
}

func TestResolver_ResolveBiome(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. Setup explicit biome colours
	biomeDir := filepath.Join(tmpDir, "data/minecraft/worldgen/biome")
	require.NoError(t, os.MkdirAll(biomeDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(biomeDir, "explicit.json"), []byte(`{
		"temperature": 0.8,
		"downfall": 0.4,
		"effects": {
			"grass_color": 16711680,
			"foliage_color": "#00ff00",
			"water_color": 255
		}
	}`), 0644))

	// 2. Setup biome using colourmaps
	require.NoError(t, os.WriteFile(filepath.Join(biomeDir, "mapped.json"), []byte(`{
		"temperature": 1.0,
		"downfall": 0.0,
		"effects": {
			"water_color": "#ffffff"
		}
	}`), 0644))

	// 3. Create colourmaps
	cmapDir := filepath.Join(tmpDir, "assets/minecraft/textures/colormap")
	require.NoError(t, os.MkdirAll(cmapDir, 0755))

	grassImg := image.NewRGBA(image.Rect(0, 0, 256, 256))
	grassImg.Set(0, 255, color.RGBA{100, 150, 200, 255}) // (1.0 - t)*255 = 0, (1.0 - d*t)*255 = 255
	f1, _ := os.Create(filepath.Join(cmapDir, "grass.png"))
	png.Encode(f1, grassImg)
	f1.Close()

	foliageImg := image.NewRGBA(image.Rect(0, 0, 256, 256))
	foliageImg.Set(0, 255, color.RGBA{50, 100, 150, 255})
	f2, _ := os.Create(filepath.Join(cmapDir, "foliage.png"))
	png.Encode(f2, foliageImg)
	f2.Close()

	source := assets.NewDirectorySource(tmpDir)
	resolver := assets.NewResolver(source)
	ctx := t.Context()

	t.Run("resolves explicit colours (int and hex)", func(t *testing.T) {
		data, err := resolver.ResolveBiome(ctx, "minecraft:explicit")
		require.NoError(t, err)
		assert.Equal(t, color.RGBA{R: 255, G: 0, B: 0, A: 255}, data.Grass)
		assert.Equal(t, color.RGBA{R: 0, G: 255, B: 0, A: 255}, data.Foliage)
		assert.Equal(t, color.RGBA{R: 0, G: 0, B: 255, A: 255}, data.Water)
	})

	t.Run("resolves colours using colourmaps", func(t *testing.T) {
		data, err := resolver.ResolveBiome(ctx, "minecraft:mapped")
		require.NoError(t, err)
		assert.Equal(t, color.RGBA{R: 100, G: 150, B: 200, A: 255}, data.Grass)
		assert.Equal(t, color.RGBA{R: 50, G: 100, B: 150, A: 255}, data.Foliage)
		assert.Equal(t, color.RGBA{R: 255, G: 255, B: 255, A: 255}, data.Water)
	})

	t.Run("fails when biome file is missing", func(t *testing.T) {
		_, err := resolver.ResolveBiome(ctx, "minecraft:missing")
		assert.Error(t, err)
	})
}
