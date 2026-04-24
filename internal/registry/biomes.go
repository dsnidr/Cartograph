package registry

import "image/color"

// BiomeColour holds the tint colours applied to grass, foliage, and water in a specific biome.
type BiomeColour struct {
	Grass   color.RGBA
	Foliage color.RGBA
	Water   color.RGBA
}

// DefaultBiomeColour is the fallback tint profile used when a biome is unrecognized.
var DefaultBiomeColour = BiomeColour{
	Grass:   color.RGBA{R: 145, G: 189, B: 89, A: 255}, // Plains Grass
	Foliage: color.RGBA{R: 119, G: 171, B: 47, A: 255}, // Plains Foliage
	Water:   color.RGBA{R: 63, G: 118, B: 228, A: 255}, // Plains Water
}

// TintType represents the type of biome tint to apply.
type TintType int

const (
	// TintNone indicates no biome tinting should be applied.
	TintNone TintType = iota

	// TintGrass indicates the block should be tinted with the biome's grass colour.
	TintGrass

	// TintFoliage indicates the block should be tinted with the biome's foliage colour.
	TintFoliage

	// TintWater indicates the block should be tinted with the biome's water colour.
	TintWater
)
