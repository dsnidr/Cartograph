// Package csd provides functionality for writing Cartograph Spatial Data (.csd) files.
package csd

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"

	"github.com/dsnidr/cartograph/internal/render"
	"github.com/klauspost/compress/zstd"
)

// Header contains the metadata for the spatial data dump.
type Header struct {
	Version      int      `json:"version"`
	Width        int      `json:"width"`
	Height       int      `json:"height"`
	Scale        int      `json:"scale"`
	BiomePalette []string `json:"biome_palette"`
}

// Write writes the spatial data dump to the provided writer.
func Write(w io.Writer, width, height, scale int, palette *render.BiomePalette, heightmap *render.FileBackedHeightmap, biomemap *render.FileBackedBiomeMap) (err error) {
	zw, err := zstd.NewWriter(w)
	if err != nil {
		return fmt.Errorf("create zstd writer: %w", err)
	}

	defer func() {
		zErr := zw.Close()
		if err == nil {
			err = zErr
		}
	}()

	header := Header{
		Version:      1,
		Width:        width,
		Height:       height,
		Scale:        scale,
		BiomePalette: palette.GetNames(),
	}

	headerBytes, err := json.Marshal(header)
	if err != nil {
		return fmt.Errorf("marshal header: %w", err)
	}

	// Write header length and header
	if err := binary.Write(zw, binary.LittleEndian, uint32(len(headerBytes))); err != nil {
		return fmt.Errorf("write header length: %w", err)
	}
	if _, err := zw.Write(headerBytes); err != nil {
		return fmt.Errorf("write header: %w", err)
	}

	// Write height data (int16), sourced from FileBackedHeightmap.
	heightRowBuf := make([]byte, width*2)
	for y := range height {
		row, err := heightmap.ReadSubHeightmap(0, y, width, 1)
		if err != nil {
			return fmt.Errorf("read heightmap row %d: %w", y, err)
		}

		for x := range width {
			// convert to absolute Y
			val := row.Gray16At(x, 0).Y
			heightY := int16(val) - 128
			binary.LittleEndian.PutUint16(heightRowBuf[x*2:], uint16(heightY))
		}

		if _, err := zw.Write(heightRowBuf); err != nil {
			return fmt.Errorf("write heights row %d: %w", y, err)
		}
	}

	// Write biomes (uint16)
	biomeRowBuf := make([]byte, width*2)
	for y := range height {
		row, err := biomemap.ReadSubBiomeMap(0, y, width, 1)
		if err != nil {
			return fmt.Errorf("read biomemap row %d: %w", y, err)
		}

		for x := range width {
			val := row.Gray16At(x, 0).Y
			binary.LittleEndian.PutUint16(biomeRowBuf[x*2:], val)
		}

		if _, err := zw.Write(biomeRowBuf); err != nil {
			return fmt.Errorf("write biomes row %d: %w", y, err)
		}
	}

	return nil
}
