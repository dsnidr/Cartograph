// Package assets provides utilities for handling Minecraft assets like textures and colormaps.
package assets

import (
	"image"
	"image/color"

	_ "github.com/dsnidr/cartograph/internal/fastpng" // register optimized PNG decoder
)

// TransparencyThreshold defines the fraction of transparent pixels required
// to mark a block as transparent. 0.6 means >60% transparent.
const TransparencyThreshold = 0.6

// ComputeAverageColour returns the average non-transparent RGB colour for the provided image
// and a boolean indicating if the block should be considered transparent.
func ComputeAverageColour(img image.Image) (color.RGBA, bool) {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// Minecraft animated textures are typically 16x(16*N)
	if height > width && height%width == 0 {
		height = width
	}

	var rSum, gSum, bSum, count uint64
	var transparentCount uint64

	// Try to assert to concrete types to bypass interface dispatch overhead.
	switch cimg := img.(type) {
	case *image.RGBA:
		for y := bounds.Min.Y; y < bounds.Min.Y+height; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				c := cimg.RGBAAt(x, y)
				if c.A == 0 {
					transparentCount++
					continue
				}

				r, g, b, a := uint32(c.R), uint32(c.G), uint32(c.B), uint32(c.A)
				r |= r << 8
				g |= g << 8
				b |= b << 8
				a |= a << 8

				if a < 0xffff {
					r = (r * 0xffff) / a
					g = (g * 0xffff) / a
					b = (b * 0xffff) / a
				}

				rSum += uint64(r >> 8)
				gSum += uint64(g >> 8)
				bSum += uint64(b >> 8)
				count++
			}
		}
	case *image.NRGBA:
		for y := bounds.Min.Y; y < bounds.Min.Y+height; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				c := cimg.NRGBAAt(x, y)
				if c.A == 0 {
					transparentCount++
					continue
				}

				// NRGBA is already non-premultiplied.
				rSum += uint64(c.R)
				gSum += uint64(c.G)
				bSum += uint64(c.B)
				count++
			}
		}
	default:
		// Fallback for other image types
		for y := bounds.Min.Y; y < bounds.Min.Y+height; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				r, g, b, a := img.At(x, y).RGBA()

				if a == 0 {
					transparentCount++
					continue
				}

				// Un-premultiply if partially transparent
				if a < 0xffff {
					r = (r * 0xffff) / a
					g = (g * 0xffff) / a
					b = (b * 0xffff) / a
				}

				rSum += uint64(r >> 8)
				gSum += uint64(g >> 8)
				bSum += uint64(b >> 8)
				count++
			}
		}
	}

	total := uint64(width * height)
	isTransparent := float64(transparentCount) > float64(total)*TransparencyThreshold

	if count == 0 {
		return color.RGBA{0, 0, 0, 0}, true
	}

	return color.RGBA{
		R: uint8(rSum / count),
		G: uint8(gSum / count),
		B: uint8(bSum / count),
		A: 255,
	}, isTransparent
}
