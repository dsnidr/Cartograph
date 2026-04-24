package render_test

import (
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"

	"github.com/dsnidr/cartograph/internal/render"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileBackedImage(t *testing.T) {
	t.Run("creates manages and cleans up temp file", func(t *testing.T) {
		tempDir := t.TempDir()
		tempPath := filepath.Join(tempDir, "test_canvas.tmp")

		canvas, err := render.NewFileBackedImage(10, 10, tempPath)
		require.NoError(t, err)
		require.NotNil(t, canvas)

		// File should exist and be truncated to exact size
		stat, err := os.Stat(tempPath)
		require.NoError(t, err)
		assert.Equal(t, int64(10*10*4), stat.Size())

		// Test property getters
		assert.Equal(t, color.RGBAModel, canvas.ColorModel())
		assert.Equal(t, image.Rect(0, 0, 10, 10), canvas.Bounds())

		// Test cleanup
		err = canvas.Close()
		require.NoError(t, err)

		_, err = os.Stat(tempPath)
		assert.True(t, os.IsNotExist(err), "expected file to be deleted")
	})

	t.Run("returns error when write is out of bounds", func(t *testing.T) {
		tempDir := t.TempDir()
		canvas, err := render.NewFileBackedImage(10, 10, filepath.Join(tempDir, "bounds.tmp"))
		require.NoError(t, err)
		defer canvas.Close()

		img := image.NewRGBA(image.Rect(0, 0, 5, 5))

		err = canvas.WriteSubImage(8, 8, img) // This will push the 5x5 image out of the 10x10 bounds
		assert.ErrorContains(t, err, "write out of bounds")
	})

	t.Run("writes and reads pixels correctly", func(t *testing.T) {
		tempDir := t.TempDir()
		canvas, err := render.NewFileBackedImage(20, 20, filepath.Join(tempDir, "rw.tmp"))
		require.NoError(t, err)
		defer canvas.Close()

		img := image.NewRGBA(image.Rect(0, 0, 2, 2))

		// Fill a 2x2 square with red
		red := color.RGBA{255, 0, 0, 255}
		img.Set(0, 0, red)
		img.Set(1, 0, red)
		img.Set(0, 1, red)
		img.Set(1, 1, red)

		// Write the 2x2 red square at an offset
		err = canvas.WriteSubImage(5, 5, img)
		require.NoError(t, err)

		// Reading inside the red box should return red
		assert.Equal(t, red, canvas.At(5, 5))
		assert.Equal(t, red, canvas.At(6, 6))

		// Reading outside the red box should return transparent/black (default empty bytes)
		assert.Equal(t, color.RGBA{0, 0, 0, 0}, canvas.At(0, 0))
		assert.Equal(t, color.RGBA{0, 0, 0, 0}, canvas.At(10, 10))

		// Reading out of bounds entirely should return transparent without panicking
		assert.Equal(t, color.RGBA{}, canvas.At(50, 50))
	})
}
