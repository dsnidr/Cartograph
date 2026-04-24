package assets

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// ErrNoTexture is returned when a suitable texture cannot be found for a block.
var ErrNoTexture = errors.New("no suitable texture found")

// Resolver determines the top-facing average colour of a block by resolving its blockstate and models.
type Resolver struct {
	source        Source
	mu            sync.Mutex
	grassMap      *Colormap
	foliageMap    *Colormap
	grassMapErr   error
	foliageMapErr error
}

// NewResolver creates a new Resolver using the provided Source.
func NewResolver(source Source) *Resolver {
	return &Resolver{source: source}
}

// BiomeData stores the calculated colours for a biome.
type BiomeData struct {
	Grass   color.RGBA
	Foliage color.RGBA
	Water   color.RGBA
}

// ResolveBiome determines the average colours for a given biome ID.
func (r *Resolver) ResolveBiome(ctx context.Context, biomeID string) (BiomeData, error) {
	loc := ParseResourceLocation(biomeID)
	path := fmt.Sprintf("data/%s/worldgen/biome/%s.json", loc.Namespace, loc.Path)

	rc, err := r.source.Open(ctx, path)
	if err != nil {
		return BiomeData{}, err
	}
	defer func() {
		_ = rc.Close()
	}()

	var biome struct {
		Temperature float32 `json:"temperature"`
		Downfall    float32 `json:"downfall"`
		Effects     struct {
			GrassColour   *mcColour `json:"grass_color"`
			FoliageColour *mcColour `json:"foliage_color"`
			WaterColour   mcColour  `json:"water_color"`
		} `json:"effects"`
	}

	if err := json.NewDecoder(rc).Decode(&biome); err != nil {
		return BiomeData{}, err
	}

	data := BiomeData{
		Water: biome.Effects.WaterColour.RGBA(),
	}

	// Resolve grass colour
	if biome.Effects.GrassColour != nil {
		data.Grass = biome.Effects.GrassColour.RGBA()
	} else {
		r.mu.Lock()
		if r.grassMap == nil && r.grassMapErr == nil {
			r.grassMap, r.grassMapErr = LoadColormap(ctx, r.source, "assets/minecraft/textures/colormap/grass.png")
		}
		gMap := r.grassMap
		gErr := r.grassMapErr
		r.mu.Unlock()

		if gErr != nil {
			return BiomeData{}, fmt.Errorf("failed to load grass colormap: %w", gErr)
		}
		data.Grass = gMap.GetColour(biome.Temperature, biome.Downfall)
	}

	// Resolve foliage colour
	if biome.Effects.FoliageColour != nil {
		data.Foliage = biome.Effects.FoliageColour.RGBA()
	} else {
		r.mu.Lock()
		if r.foliageMap == nil && r.foliageMapErr == nil {
			r.foliageMap, r.foliageMapErr = LoadColormap(ctx, r.source, "assets/minecraft/textures/colormap/foliage.png")
		}
		fMap := r.foliageMap
		fErr := r.foliageMapErr
		r.mu.Unlock()

		if fErr != nil {
			return BiomeData{}, fmt.Errorf("failed to load foliage colormap: %w", fErr)
		}
		data.Foliage = fMap.GetColour(biome.Temperature, biome.Downfall)
	}

	return data, nil
}

type mcColour int

func (c *mcColour) UnmarshalJSON(data []byte) error {
	if data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		if strings.HasPrefix(s, "#") {
			var val int
			if _, err := fmt.Sscanf(s, "#%x", &val); err != nil {
				return err
			}
			*c = mcColour(val)
			return nil
		}
		if val, err := strconv.ParseInt(s, 10, 64); err == nil {
			*c = mcColour(val)
			return nil
		}
	}

	var val int
	if err := json.Unmarshal(data, &val); err != nil {
		return err
	}

	*c = mcColour(val)
	return nil
}

func (c mcColour) RGBA() color.RGBA {
	val := int(c)

	return color.RGBA{
		R: uint8((val >> 16) & 0xFF),
		G: uint8((val >> 8) & 0xFF),
		B: uint8(val & 0xFF),
		A: 255,
	}
}

// Resolve returns the average colour and transparency for a block ID by traversing blockstates and models.
func (r *Resolver) Resolve(ctx context.Context, blockID string) (color.RGBA, bool, error) {
	loc := ParseResourceLocation(blockID)
	bs, err := LoadBlockState(ctx, r.source, loc)
	if err != nil {
		return color.RGBA{}, false, wrapError(err, blockID, loc.BlockstatePath(), PhaseLoadBlockState, "valid blockstate JSON")
	}

	modelID := ""
	if len(bs.Variants) > 0 {
		// Prefer the default variant or just pick the first available.
		if v, ok := bs.Variants[""]; ok {
			modelID = v.Model
		} else if v, ok := bs.Variants["normal"]; ok {
			modelID = v.Model
		} else {
			// Sort the variants to ensure deterministic selection
			keys := make([]string, 0, len(bs.Variants))
			for k := range bs.Variants {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			modelID = bs.Variants[keys[0]].Model
		}
	} else if len(bs.Multipart) > 0 {
		modelID = bs.Multipart[0].Apply.Model
	}

	if modelID == "" {
		return color.RGBA{}, false, newError(blockID, loc.BlockstatePath(), PhaseFindModelVariant, "variants or multipart definitions")
	}

	texLoc, err := r.resolveModelTexture(ctx, blockID, modelID)
	if err != nil {
		return color.RGBA{}, false, err
	}

	rc, err := r.source.Open(ctx, texLoc.TexturePath())
	if err != nil {
		return color.RGBA{}, false, wrapError(err, blockID, texLoc.TexturePath(), PhaseReadTexture, "readable PNG file")
	}
	defer func() {
		_ = rc.Close()
	}()

	img, _, err := image.Decode(rc)
	if err != nil {
		return color.RGBA{}, false, wrapError(err, blockID, texLoc.TexturePath(), PhaseDecodeTexture, "valid image format")
	}

	c, transparent := ComputeAverageColour(img)
	return c, transparent, nil
}

func (r *Resolver) resolveModelTexture(ctx context.Context, rootBlockID, modelID string) (ResourceLocation, error) {
	textures := make(map[string]string)
	current := modelID

	// Traverse inheritance chain and collect textures
	for current != "" {
		loc := ParseResourceLocation(current)
		m, err := LoadModel(ctx, r.source, loc)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				if len(textures) == 0 {
					return ResourceLocation{}, wrapError(err, rootBlockID, loc.ModelPath(), PhaseLoadModel, "initial model file")
				}
				break
			}
			return ResourceLocation{}, wrapError(err, rootBlockID, loc.ModelPath(), PhaseLoadModel, "valid model JSON")
		}

		for k, v := range m.Textures {
			if _, ok := textures[k]; !ok {
				switch val := v.(type) {
				case string:
					textures[k] = val
				case map[string]any:
					// Handle modern Minecraft (1.20+) render-type-hint format where texture is an object
					if sprite, ok := val["sprite"].(string); ok {
						textures[k] = sprite
					}
				}
			}
		}
		current = m.Parent
	}

	// Pick the best candidate key for a top-down map
	// We added pane/edge specific mappings for stained glass and panes.
	candidates := []string{"up", "top", "all", "texture", "particle", "pane", "edge"}
	var found string
	for _, c := range candidates {
		if v, ok := textures[c]; ok {
			found = v
			break
		}
	}

	if found == "" {
		expectedMsg := fmt.Sprintf("keys %v, got %v", candidates, mapKeys(textures))
		return ResourceLocation{}, wrapError(ErrNoTexture, rootBlockID, modelID, PhaseSelectCandidate, expectedMsg)
	}

	// Resolve texture variables (e.g., `#all` -> `minecraft:block/stone`)
	const maxAttempts = 10
	for range maxAttempts {
		if !strings.HasPrefix(found, "#") {
			return ParseResourceLocation(found), nil
		}

		next, ok := textures[found[1:]]
		if !ok {
			return ResourceLocation{}, newError(rootBlockID, modelID, PhaseResolveVariables, fmt.Sprintf("variable %s to be defined", found))
		}

		found = next
	}

	return ResourceLocation{}, newError(rootBlockID, modelID, PhaseResolveVariables, fmt.Sprintf("less than %d variable redirections", maxAttempts))
}

func mapKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// BlockState represents the contents of a blockstate JSON file
type BlockState struct {
	Variants  map[string]BlockStateVariant `json:"variants"`
	Multipart []BlockStateMultipart        `json:"multipart"`
}

// BlockStateVariant represents a single variant in a BlockState
type BlockStateVariant struct {
	Model string `json:"model"`
}

// UnmarshalJSON handles both single-object variants and weighted-array variants.
func (v *BlockStateVariant) UnmarshalJSON(data []byte) error {
	if data[0] == '[' {
		var arr []struct {
			Model string `json:"model"`
		}
		if err := json.Unmarshal(data, &arr); err != nil {
			return err
		}
		if len(arr) > 0 {
			v.Model = arr[0].Model
		}
		return nil
	}

	type alias BlockStateVariant
	return json.Unmarshal(data, (*alias)(v))
}

// BlockStateMultipart represents a conditional model part in a multipart blockstate.
type BlockStateMultipart struct {
	Apply BlockStateVariant `json:"apply"`
}

// Model represents the contents of a model JSON file.
type Model struct {
	Parent   string         `json:"parent"`
	Textures map[string]any `json:"textures"`
}

// ResourceLocation parses "namespace:path" strings.
type ResourceLocation struct {
	Namespace string
	Path      string
}

// ParseResourceLocation parses "namespace:path" strings into a ResourceLocation.
func ParseResourceLocation(raw string) ResourceLocation {
	parts := strings.Split(raw, ":")
	if len(parts) == 2 {
		return ResourceLocation{Namespace: parts[0], Path: parts[1]}
	}

	return ResourceLocation{Namespace: "minecraft", Path: parts[0]}
}

// BlockstatePath returns the path to the blockstate JSON file for this ResourceLocation.
func (r ResourceLocation) BlockstatePath() string {
	return fmt.Sprintf("assets/%s/blockstates/%s.json", r.Namespace, r.Path)
}

// ModelPath returns the path to the model JSON file for this ResourceLocation.
func (r ResourceLocation) ModelPath() string {
	return fmt.Sprintf("assets/%s/models/%s.json", r.Namespace, r.Path)
}

// TexturePath returns the path to the texture PNG file for this ResourceLocation.
func (r ResourceLocation) TexturePath() string {
	return fmt.Sprintf("assets/%s/textures/%s.png", r.Namespace, r.Path)
}

// LoadBlockState helper to read and parse from an Source
func LoadBlockState(ctx context.Context, s Source, loc ResourceLocation) (*BlockState, error) {
	rc, err := s.Open(ctx, loc.BlockstatePath())
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rc.Close()
	}()

	var bs BlockState
	if err := json.NewDecoder(rc).Decode(&bs); err != nil {
		return nil, err
	}

	return &bs, nil
}

// LoadModel helper to read and parse from an Source
func LoadModel(ctx context.Context, s Source, loc ResourceLocation) (*Model, error) {
	rc, err := s.Open(ctx, loc.ModelPath())
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rc.Close()
	}()

	var m Model
	if err := json.NewDecoder(rc).Decode(&m); err != nil {
		return nil, err
	}

	return &m, nil
}
