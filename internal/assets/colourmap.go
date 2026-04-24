package assets

import (
	"context"
	"image"
	"image/color"

	_ "github.com/dsnidr/cartograph/internal/fastpng" // Register PNG decoder
)

// Colormap represents a Minecraft colour map image (e.g., grass.png, foliage.png).
type Colormap struct {
	img image.Image
}

// LoadColormap loads a Colormap from the given source and path.
func LoadColormap(ctx context.Context, source Source, path string) (*Colormap, error) {
	rc, err := source.Open(ctx, path)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rc.Close()
	}()

	img, _, err := image.Decode(rc)
	if err != nil {
		return nil, err
	}

	return &Colormap{img: img}, nil
}

// GetColour returns the colour for a given temperature and downfall from the colormap.
func (c *Colormap) GetColour(temperature, downfall float32) color.RGBA {
	// Clamp temperature and downfall to [0.0, 1.0]
	t := clamp(temperature, 0.0, 1.0)
	d := clamp(downfall, 0.0, 1.0) * t

	bounds := c.img.Bounds()

	// Calculate pixel coordinates dynamically based on image bounds
	x := bounds.Min.X + int((1.0-t)*float32(bounds.Dx()-1))
	y := bounds.Min.Y + int((1.0-d)*float32(bounds.Dy()-1))

	// Bounds check
	if x < bounds.Min.X {
		x = bounds.Min.X
	} else if x >= bounds.Max.X {
		x = bounds.Max.X - 1
	}
	if y < bounds.Min.Y {
		y = bounds.Min.Y
	} else if y >= bounds.Max.Y {
		y = bounds.Max.Y - 1
	}

	r, g, b, a := c.img.At(x, y).RGBA()
	return color.RGBA{
		R: uint8(r >> 8),
		G: uint8(g >> 8),
		B: uint8(b >> 8),
		A: uint8(a >> 8),
	}
}

func clamp(v, minVal, maxVal float32) float32 {
	if v < minVal {
		return minVal
	}
	if v > maxVal {
		return maxVal
	}
	return v
}
