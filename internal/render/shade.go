package render

import (
	"image"
	"image/color"
	"math"
)

const (
	// ShadeSlopeMultiplier controls how aggressively slopes are shaded.
	ShadeSlopeMultiplier = 0.12

	// ShadeMaxLighten is the maximum multiplier applied to sunny slopes.
	ShadeMaxLighten = 1.35

	// ShadeMaxDarken is the minimum multiplier applied to shaded slopes.
	ShadeMaxDarken = 0.40

	// CastShadowMultiplier is the shadow multiplier applied when a block is occluded from the sun.
	CastShadowMultiplier = 0.65

	// HeightOffset is added to Y coordinates when storing in Gray16 to avoid negative values
	HeightOffset = 128

	// ShadowRaySteps is the number of steps to raymarch for shadows
	ShadowRaySteps = 15
)

// ApplyReliefShading calculates and applies topographical shading and cast shadows to a color based on a heightmap.
func ApplyReliefShading(base color.RGBA, x, z int, hm *image.Gray16) color.RGBA {
	currentHeight := int(hm.Gray16At(x, z).Y) - HeightOffset
	factor := 1.0

	// 1. Elevation-Based Shading (Global Height)
	// Sea level is 62. Normalize around there to slightly darken deep areas and lighten peaks.
	elevationDiff := float64(currentHeight - 62)
	// subtle global elevation gradient (e.g. +/- 10% max)
	elevationFactor := 1.0 + (elevationDiff * 0.001)
	if elevationFactor < 0.8 {
		elevationFactor = 0.8
	} else if elevationFactor > 1.2 {
		elevationFactor = 1.2
	}
	factor *= elevationFactor

	// 2. Smooth Light Direction & Slope Shading
	// Central difference slope calculation.
	// Make sure we do not read outside the padded heightmap array
	dx, dz := 0, 0

	if z > 0 && z < hm.Bounds().Dy()-1 {
		dz = int(hm.Gray16At(x, z+1).Y) - int(hm.Gray16At(x, z-1).Y)
	}
	if x > 0 && x < hm.Bounds().Dx()-1 {
		dx = int(hm.Gray16At(x+1, z).Y) - int(hm.Gray16At(x-1, z).Y)
	}

	// Because we measure slope over 2 blocks, we halve the difference to normalize it back to a 1-block gradient.
	diff := float64(dx+dz) / 2.0

	if diff > 0 {
		slope := 1.0 + (diff * ShadeSlopeMultiplier)
		if slope > ShadeMaxLighten {
			slope = ShadeMaxLighten
		}
		factor *= slope
	} else if diff < 0 {
		slope := 1.0 + (diff * ShadeSlopeMultiplier)
		if slope < ShadeMaxDarken {
			slope = ShadeMaxDarken
		}
		factor *= slope
	}

	// 3. Cast Shadows (Raymarch towards the NW Sun)
	// Sun vector is roughly (-1, -1, +1) (45 degrees up to the Northwest)
	inShadow := false
	for step := 1; step <= ShadowRaySteps; step++ {
		nx, nz := x-step, z-step

		// If raycast goes out of the loaded padded tile, stop to avoid array out-of-bounds
		if nx < 0 || nz < 0 || nx >= hm.Bounds().Dx() || nz >= hm.Bounds().Dy() {
			break
		}

		// The ray from the sun slopes downwards. If a block in that direction is higher than the ray, we are occluded.
		rayHeight := currentHeight + step
		if int(hm.Gray16At(nx, nz).Y)-HeightOffset > rayHeight {
			inShadow = true
			break
		}
	}

	if inShadow {
		factor *= CastShadowMultiplier
	}

	factor = math.Round(factor*32.0) / 32.0

	r := float64(base.R) * factor
	g := float64(base.G) * factor
	b := float64(base.B) * factor

	// Ensure clamped to uint8 bounds (0-255)
	return color.RGBA{
		R: clampUint8(r),
		G: clampUint8(g),
		B: clampUint8(b),
		A: base.A,
	}
}

// clampUint8 ensures a float64 value safely fits within a uint8 without overflowing
func clampUint8(v float64) uint8 {
	if v > 255 {
		return 255
	}

	if v < 0 {
		return 0
	}

	// We mathematically round (v+0.5) to fix floating point truncation
	return uint8(v + 0.5)
}
