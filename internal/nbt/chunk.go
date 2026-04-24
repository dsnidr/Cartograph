// Package nbt contains logic for decoding and parsing Minecraft's NBT data.
package nbt

import (
	"fmt"
	"math/bits"
	"strings"
)

// RegionChunk represents a single parsed chunk from a region file.
type RegionChunk struct {
	DataVersion int32
	XPos        int32
	ZPos        int32
	Status      string
	Sections    []Section
}

// Section represents a 16x16x16 cubic section within a chunk.
type Section struct {
	Y           int8
	BlockStates *PaletteContainer // will be nil if section is empty
	Biomes      *PaletteContainer
}

// PaletteContainer holds bit-packed data and its associated palette for blocks or biomes.
type PaletteContainer struct {
	Palette []BlockState
	Data    []int64
}

// GetStateIndex returns the integer index of the state in the palette at the given linear index.
func (pc *PaletteContainer) GetStateIndex(index int, bitsPerEntry int) int {
	if len(pc.Palette) == 0 {
		return -1
	}

	if len(pc.Data) == 0 || len(pc.Palette) == 1 {
		return 0
	}

	if bitsPerEntry <= 0 {
		return 0
	}

	entriesPerLong := 64 / bitsPerEntry
	longIndex := index / entriesPerLong

	if longIndex < 0 || longIndex >= len(pc.Data) {
		return 0
	}

	bitOffset := (index % entriesPerLong) * bitsPerEntry
	mask := (1 << bitsPerEntry) - 1

	paletteIndex := int((pc.Data[longIndex] >> bitOffset) & int64(mask))

	if paletteIndex >= len(pc.Palette) {
		return 0
	}

	return paletteIndex
}

// GetState returns the BlockState at the specific linear index. It handles the 1.16+ bit-packing math.
func (pc *PaletteContainer) GetState(index int, bitsPerEntry int) *BlockState {
	palIdx := pc.GetStateIndex(index, bitsPerEntry)
	if palIdx < 0 {
		return nil
	}
	return &pc.Palette[palIdx]
}

// GetBlock returns the block state at local section coordinates (0-15).
func (s *Section) GetBlock(x, y, z int) *BlockState {
	if s.BlockStates == nil || len(s.BlockStates.Palette) == 0 {
		return nil
	}

	// Calculate bits per entry (minimum of 4 for blocks).
	bitsPerEntry := max(bits.Len(uint(len(s.BlockStates.Palette)-1)), 4)

	// YZX ordering for blocks
	index := (y * 16 * 16) + (z * 16) + x

	return s.BlockStates.GetState(index, bitsPerEntry)
}

// GetBlockIndex returns the palette index of the block at local section coordinates (0-15).
func (s *Section) GetBlockIndex(x, y, z int) int {
	if s.BlockStates == nil || len(s.BlockStates.Palette) == 0 {
		return -1
	}

	bitsPerEntry := max(bits.Len(uint(len(s.BlockStates.Palette)-1)), 4)
	index := (y * 16 * 16) + (z * 16) + x

	return s.BlockStates.GetStateIndex(index, bitsPerEntry)
}

// GetBiome returns the biome state at local section coordinates (0-15). In 1.18+, biomes are stored
// in a 4x4x4 grid per section.
func (s *Section) GetBiome(x, y, z int) *BlockState {
	if s.Biomes == nil || len(s.Biomes.Palette) == 0 {
		return nil
	}

	bitsPerEntry := bits.Len(uint(len(s.Biomes.Palette) - 1))

	// Convert 16x16x16 local block coordinates into 4x4x4 biome grid coordinates.
	biomeX := x / 4
	biomeY := y / 4
	biomeZ := z / 4

	// YZX ordering
	index := (biomeY * 4 * 4) + (biomeZ * 4) + biomeX

	return s.Biomes.GetState(index, bitsPerEntry)
}

// BlockState represents a specific block or biome state, with an interned name and properties.
type BlockState struct {
	Name       string
	Properties map[string]string
}

// HasProperty returns true if the block state has a property matching the given key and value.
func (b *BlockState) HasProperty(key, value string) bool {
	if b.Properties == nil {
		return false
	}

	return b.Properties[key] == value
}

// String provides a highly readable output for debugging block states.
func (b *BlockState) String() string {
	if len(b.Properties) == 0 {
		return b.Name
	}

	props := make([]string, 0, len(b.Properties))
	for k, v := range b.Properties {
		props = append(props, fmt.Sprintf("%s=%s", k, v))
	}

	return fmt.Sprintf("%s[%s]", b.Name, strings.Join(props, ","))
}
