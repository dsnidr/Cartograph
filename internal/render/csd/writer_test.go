package csd

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"image"
	"image/color"
	"path/filepath"
	"testing"

	"github.com/dsnidr/cartograph/internal/render"
	"github.com/klauspost/compress/zstd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWrite(t *testing.T) {
	t.Run("should write valid csd file with uint16 heights and biomes", func(t *testing.T) {
		tempDir := t.TempDir()

		width, height := 4, 4
		scale := 1

		palette := render.NewBiomePalette()
		palette.GetID("minecraft:plains")
		palette.GetID("minecraft:desert")

		hmPath := filepath.Join(tempDir, "heightmap.tmp")
		hm, err := render.NewFileBackedHeightmap(width, height, hmPath)
		require.NoError(t, err)
		defer hm.Close()

		bmPath := filepath.Join(tempDir, "biomemap.tmp")
		bm, err := render.NewFileBackedBiomeMap(width, height, bmPath)
		require.NoError(t, err)
		defer bm.Close()

		// Fill heightmap with some data (Y + 128)
		hmData := image.NewGray16(image.Rect(0, 0, width, height))
		for y := range height {
			for x := range width {
				hmData.SetGray16(x, y, color.Gray16{Y: 128}) // Y = 0
			}
		}
		// Set one specific height: Y = 64 -> Gray16 = 64 + 128 = 192
		hmData.SetGray16(0, 0, color.Gray16{Y: 192})
		require.NoError(t, hm.WriteSubHeightmap(0, 0, hmData))

		// Fill biomemap
		bmData := image.NewGray16(image.Rect(0, 0, width, height))
		bmData.SetGray16(0, 0, color.Gray16{Y: 1}) // plains (index 1)
		bmData.SetGray16(1, 1, color.Gray16{Y: 2}) // desert (index 2)
		require.NoError(t, bm.WriteSubBiomeMap(0, 0, bmData))

		buf := new(bytes.Buffer)
		err = Write(buf, width, height, scale, palette, hm, bm)
		require.NoError(t, err)

		// Verify output
		zr, err := zstd.NewReader(buf)
		require.NoError(t, err)
		defer zr.Close()

		var headerLen uint32
		err = binary.Read(zr, binary.LittleEndian, &headerLen)
		require.NoError(t, err)

		headerBytes := make([]byte, headerLen)
		_, err = zr.Read(headerBytes)
		require.NoError(t, err)

		var header Header
		err = json.Unmarshal(headerBytes, &header)
		require.NoError(t, err)

		assert.Equal(t, 1, header.Version)
		assert.Equal(t, width, header.Width)
		assert.Equal(t, height, header.Height)
		assert.Equal(t, []string{"", "minecraft:plains", "minecraft:desert"}, header.BiomePalette)

		// Read heights
		heights := make([]int16, width*height)
		err = binary.Read(zr, binary.LittleEndian, &heights)
		require.NoError(t, err)
		assert.Equal(t, int16(64), heights[0])
		assert.Equal(t, int16(0), heights[1])

		// Read biomes
		biomes := make([]uint16, width*height)
		err = binary.Read(zr, binary.LittleEndian, &biomes)
		require.NoError(t, err)
		assert.Equal(t, uint16(1), biomes[0]) // (0,0) -> plains (index 1)
		assert.Equal(t, uint16(2), biomes[5]) // (1,1) -> desert (index 2)
	})
}
