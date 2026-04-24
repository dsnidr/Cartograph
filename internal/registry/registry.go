// Package registry provides access to Minecraft block and biome colour definitions.
package registry

import (
	"context"
	"fmt"
	"image/color"
	"log/slog"
	"strings"
	"sync"

	"github.com/dsnidr/cartograph/pkg/stringcolour"
)

// ColourEntry stores the base colour and the biome tinting rule for a block.
type ColourEntry struct {
	Base color.RGBA
	Tint TintType
}

// Registry provides thread-safe access to block colours. It loads the embedded `vanilla_colours.jsonl` file
// containing vanilla colour definitions on startup and falls back to a deterministic hash-based colour generator
// for unknown blocks (e.g., modded blocks) to provide a consistent colour for each block.
type Registry struct {
	colours     sync.Map
	transparent sync.Map
	biomes      sync.Map
}

// New initializes a new Registry. It automatically loads the embedded vanilla colours.
func New(ctx context.Context) (*Registry, error) {
	r := &Registry{}

	vanillaColours, transparent, err := LoadVanillaColours(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load vanilla colours: %w", err)
	}

	for id, base := range vanillaColours {
		r.colours.Store(id, ColourEntry{
			Base: base,
			Tint: getTintTypeForBlock(id),
		})
	}
	for id := range transparent {
		r.transparent.Store(id, struct{}{})
	}
	slog.Debug("Loaded embedded vanilla block colours")

	vanillaBiomes, err := LoadVanillaBiomes(ctx)
	if err != nil {
		// We log a warning instead of failing because biomes are less critical than blocks
		slog.Warn("Failed to load vanilla biomes from embedded file", "error", err)
	} else {
		for id, b := range vanillaBiomes {
			r.biomes.Store(id, b)
		}
		slog.Debug("Loaded embedded vanilla biome colours")
	}

	return r, nil
}

// getTintTypeForBlock returns the appropriate biome tint rule for a block ID.
func getTintTypeForBlock(blockID string) TintType {
	// Simple heuristics based on common vanilla block IDs.
	// I could definitely improve this, but it's good enough for now.
	if strings.Contains(blockID, "leaves") || strings.Contains(blockID, "vine") {
		return TintFoliage
	}

	if strings.Contains(blockID, "grass") || strings.Contains(blockID, "fern") || strings.Contains(blockID, "sugar_cane") {
		return TintGrass
	}

	if strings.Contains(blockID, "water") || blockID == "minecraft:bubble_column" {
		return TintWater
	}

	return TintNone
}

// IsTransparent returns true if the block should be passed through by the raycaster.
func (r *Registry) IsTransparent(blockID string) bool {
	if _, ok := r.transparent.Load(blockID); ok {
		return true
	}

	// Heuristics for unknown/modded blocks
	lower := strings.ToLower(blockID)
	return strings.Contains(lower, "air") || strings.Contains(lower, "glass") || strings.Contains(lower, "pane") || strings.Contains(lower, "barrier")
}

// GetColour returns the base colour for a given block ID.
// Returns the colour entry and a boolean indicating if it was found in the pre-baked registry.
// If the block is not in the registry, it generates a deterministic fallback colour
// based on the hash of the block's name and returns false.
func (r *Registry) GetColour(blockID string) (ColourEntry, bool) {
	if v, ok := r.colours.Load(blockID); ok {
		return v.(ColourEntry), true
	}

	// Generate deterministic fallback colour
	hashColour := stringcolour.ToColour(blockID)
	entry := ColourEntry{
		Base: hashColour,
		Tint: getTintTypeForBlock(blockID),
	}

	// Cache the generated fallback to avoid rehashing
	r.colours.Store(blockID, entry)

	return entry, false
}

// RegisterColour allows manually adding or overriding a block's colour at runtime.
func (r *Registry) RegisterColour(blockID string, c color.RGBA, tint TintType) {
	r.colours.Store(blockID, ColourEntry{Base: c, Tint: tint})
}

// GetBiome returns the tint profile for a given biome ID.
func (r *Registry) GetBiome(biomeID string) BiomeColour {
	if v, ok := r.biomes.Load(biomeID); ok {
		return v.(BiomeColour)
	}

	return DefaultBiomeColour
}

// ApplyBiomeTint multiplies a base colour by a biome's specific tint colour.
func (r *Registry) ApplyBiomeTint(base color.RGBA, biomeName string, tintType TintType) color.RGBA {
	if tintType == TintNone {
		return base
	}

	biome := r.GetBiome(biomeName)

	var tint color.RGBA
	switch tintType {
	case TintGrass:
		tint = biome.Grass
	case TintFoliage:
		tint = biome.Foliage
	case TintWater:
		tint = biome.Water
	default:
		return base
	}

	// Apply Minecraft's multiplication logic: (Base * Tint) / 255
	return color.RGBA{
		R: uint8((uint16(base.R) * uint16(tint.R)) / 255),
		G: uint8((uint16(base.G) * uint16(tint.G)) / 255),
		B: uint8((uint16(base.B) * uint16(tint.B)) / 255),
		A: base.A,
	}
}
