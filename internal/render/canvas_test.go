package render

import (
	"image"
	"image/color"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileBackedBiomeMap(t *testing.T) {
	t.Run("should write and read sub biomemap correctly", func(t *testing.T) {
		tempDir := t.TempDir()
		path := filepath.Join(tempDir, "biomes.tmp")

		canvasWidth, canvasHeight := 10, 10
		bm, err := NewFileBackedBiomeMap(canvasWidth, canvasHeight, path)
		require.NoError(t, err)

		defer func() {
			err := bm.Close()
			assert.NoError(t, err)
		}()

		// Write a 2x2 submap
		sub := image.NewGray16(image.Rect(0, 0, 2, 2))
		sub.SetGray16(0, 0, color.Gray16{Y: 10})
		sub.SetGray16(1, 0, color.Gray16{Y: 20})
		sub.SetGray16(0, 1, color.Gray16{Y: 30})
		sub.SetGray16(1, 1, color.Gray16{Y: 40})

		err = bm.WriteSubBiomeMap(2, 2, sub)
		require.NoError(t, err)

		// Read it back
		readSub, err := bm.ReadSubBiomeMap(2, 2, 2, 2)
		require.NoError(t, err)

		assert.Equal(t, uint16(10), readSub.Gray16At(0, 0).Y)
		assert.Equal(t, uint16(20), readSub.Gray16At(1, 0).Y)
		assert.Equal(t, uint16(30), readSub.Gray16At(0, 1).Y)
		assert.Equal(t, uint16(40), readSub.Gray16At(1, 1).Y)
	})
}
