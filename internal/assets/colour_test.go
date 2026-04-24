package assets_test

import (
	"image"
	"image/color"
	"testing"

	"github.com/dsnidr/cartograph/internal/assets"
	"github.com/stretchr/testify/assert"
)

func TestComputeAverageColour(t *testing.T) {
	red := color.RGBA{255, 0, 0, 255}
	blue := color.RGBA{0, 0, 255, 255}
	trans := color.RGBA{0, 0, 0, 0}
	halfRed := color.RGBA{127, 0, 0, 127} // Used to test un-premultiplying (crazy word)

	tests := []struct {
		name     string
		setup    func() image.Image
		expected color.RGBA
	}{
		{
			name: "returns expected average for solid colour",
			setup: func() image.Image {
				img := image.NewRGBA(image.Rect(0, 0, 2, 2))
				for y := range 2 {
					for x := range 2 {
						img.Set(x, y, red)
					}
				}
				return img
			},
			expected: red,
		},
		{
			name: "ignores transparent pixels and averages the rest",
			setup: func() image.Image {
				img := image.NewRGBA(image.Rect(0, 0, 2, 2))
				img.Set(0, 0, red)
				img.Set(1, 0, red)
				img.Set(0, 1, red)
				img.Set(1, 1, trans)
				return img
			},
			expected: red,
		},
		{
			name: "returns transparent for fully transparent images",
			setup: func() image.Image {
				return image.NewRGBA(image.Rect(0, 0, 2, 2))
			},
			expected: trans,
		},
		{
			name: "handles animated vertical spritesheets by only reading the first frame",
			setup: func() image.Image {
				// 2x4 image, top 2x2 is red, bottom 2x2 is blue
				img := image.NewRGBA(image.Rect(0, 0, 2, 4))
				for y := range 2 {
					for x := range 2 {
						img.Set(x, y, red)
					}
				}
				for y := 2; y < 4; y++ {
					for x := range 2 {
						img.Set(x, y, blue)
					}
				}
				return img
			},
			expected: red,
		},
		{
			name: "un-premultiplies partially transparent pixels",
			setup: func() image.Image {
				img := image.NewRGBA(image.Rect(0, 0, 1, 1))
				img.Set(0, 0, halfRed)
				return img
			},
			expected: color.RGBA{R: 255, G: 0, B: 0, A: 255},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			img := tt.setup()
			avg, _ := assets.ComputeAverageColour(img)
			assert.Equal(t, tt.expected, avg)
		})
	}
}

func TestComputeAverageColour_Transparency(t *testing.T) {
	red := color.RGBA{255, 0, 0, 255}
	trans := color.RGBA{0, 0, 0, 0}

	tests := []struct {
		name                string
		setup               func() image.Image
		expectedTransparent bool
	}{
		{
			name: "returns false when mostly solid",
			setup: func() image.Image {
				img := image.NewRGBA(image.Rect(0, 0, 2, 2))
				img.Set(0, 0, red)
				img.Set(1, 0, red)
				img.Set(0, 1, red)
				img.Set(1, 1, trans) // 25% transparent
				return img
			},
			expectedTransparent: false,
		},
		{
			name: "returns true when mostly transparent (>60%)",
			setup: func() image.Image {
				// 3x3 grid = 9 pixels. 6/9 = 66.6% (>60%)
				img := image.NewRGBA(image.Rect(0, 0, 3, 3))
				img.Set(0, 0, red)
				img.Set(1, 0, red)
				img.Set(2, 0, red)
				// Remaining 6 are transparent
				return img
			},
			expectedTransparent: true,
		},
		{
			name: "returns false when slightly above 50% but below 60%",
			setup: func() image.Image {
				// 2x2 grid = 4 pixels. 2/4 = 50% (<= 60%)
				img := image.NewRGBA(image.Rect(0, 0, 2, 2))
				img.Set(0, 0, red)
				img.Set(1, 0, red)
				img.Set(0, 1, trans)
				img.Set(1, 1, trans)
				return img
			},
			expectedTransparent: false,
		},
		{
			name: "returns true for fully transparent",
			setup: func() image.Image {
				return image.NewRGBA(image.Rect(0, 0, 2, 2))
			},
			expectedTransparent: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			img := tt.setup()
			_, transparent := assets.ComputeAverageColour(img)
			assert.Equal(t, tt.expectedTransparent, transparent)
		})
	}
}
