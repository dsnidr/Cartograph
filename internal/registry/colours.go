package registry

import (
	"bufio"
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"image/color"
	"io"
)

//go:embed vanilla_colours.jsonl vanilla_biomes.jsonl
var vanillaColoursFS embed.FS

// LoadVanillaColours reads the embedded JSONL file and returns a map of block IDs to their average colours
// and a set of block IDs that should be considered transparent.
func LoadVanillaColours(_ context.Context) (map[string]color.RGBA, map[string]struct{}, error) {
	file, err := vanillaColoursFS.Open("vanilla_colours.jsonl")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open embedded vanilla colours: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	return parseJSONL(file)
}

// LoadVanillaBiomes reads the embedded JSONL file and returns a map of biome IDs to their tint profiles.
func LoadVanillaBiomes(_ context.Context) (map[string]BiomeColour, error) {
	file, err := vanillaColoursFS.Open("vanilla_biomes.jsonl")
	if err != nil {
		return nil, fmt.Errorf("failed to open embedded vanilla biomes: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	return parseBiomesJSONL(file)
}

func parseBiomesJSONL(r io.Reader) (map[string]BiomeColour, error) {
	biomes := make(map[string]BiomeColour)
	scanner := bufio.NewScanner(r)

	var currentNamespace string
	nsPrefix := []byte(`{"$namespace":`)

	var lineData map[string]struct {
		Grass   [3]uint8 `json:"grass"`
		Foliage [3]uint8 `json:"foliage"`
		Water   [3]uint8 `json:"water"`
	}

	for scanner.Scan() {
		line := scanner.Bytes()
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}

		if bytes.HasPrefix(line, nsPrefix) {
			var nsMap map[string]string
			if err := json.Unmarshal(line, &nsMap); err != nil {
				return nil, fmt.Errorf("failed to parse namespace directive: %w", err)
			}
			currentNamespace = nsMap["$namespace"]
			continue
		}

		if currentNamespace == "" {
			return nil, fmt.Errorf("encountered biome data before any $namespace directive")
		}

		clear(lineData)
		if err := json.Unmarshal(line, &lineData); err != nil {
			return nil, fmt.Errorf("failed to parse JSONL line: %w", err)
		}

		for path, data := range lineData {
			id := currentNamespace + ":" + path
			biomes[id] = BiomeColour{
				Grass:   color.RGBA{R: data.Grass[0], G: data.Grass[1], B: data.Grass[2], A: 255},
				Foliage: color.RGBA{R: data.Foliage[0], G: data.Foliage[1], B: data.Foliage[2], A: 255},
				Water:   color.RGBA{R: data.Water[0], G: data.Water[1], B: data.Water[2], A: 255},
			}
		}
	}

	return biomes, scanner.Err()
}

// parseJSONL reads a stream of JSON Lines, keeping track of namespace contexts.
// Lines can either be `{"$namespace": "minecraft"}` or `{"stone": [125,125,125,255,0]}`.
func parseJSONL(r io.Reader) (map[string]color.RGBA, map[string]struct{}, error) {
	colours := make(map[string]color.RGBA)
	transparent := make(map[string]struct{})
	scanner := bufio.NewScanner(r)

	var currentNamespace string
	nsPrefix := []byte(`{"$namespace":`)

	// A single buffer to avoid allocating maps every line.
	// We use json.RawMessage to defer decoding so we can cleanly check the structure type.
	var lineData map[string]json.RawMessage

	for scanner.Scan() {
		line := scanner.Bytes()
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}

		// Fast path for namespace directive
		if bytes.HasPrefix(line, nsPrefix) {
			var nsMap map[string]string
			if err := json.Unmarshal(line, &nsMap); err != nil {
				return nil, nil, fmt.Errorf("failed to parse namespace directive %q: %w", string(line), err)
			}
			if ns, ok := nsMap["$namespace"]; ok {
				currentNamespace = ns
			}
			continue
		}

		// Block data line
		if currentNamespace == "" {
			return nil, nil, fmt.Errorf("encountered block data before any $namespace directive: %s", string(line))
		}

		// clear lineData so values from previous lines don't leak into this one
		clear(lineData)
		if err := json.Unmarshal(line, &lineData); err != nil {
			return nil, nil, fmt.Errorf("failed to parse JSONL line %q: %w", string(line), err)
		}

		for path, rawData := range lineData {
			var data [5]uint8
			if err := json.Unmarshal(rawData, &data); err != nil {
				return nil, nil, fmt.Errorf("failed to unmarshal block colour array for %s: %w", path, err)
			}

			id := currentNamespace + ":" + path

			colours[id] = color.RGBA{
				R: data[0],
				G: data[1],
				B: data[2],
				A: data[3],
			}

			if data[4] == 1 {
				transparent[id] = struct{}{}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, nil, fmt.Errorf("error reading JSONL data: %w", err)
	}

	return colours, transparent, nil
}
