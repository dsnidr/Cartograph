package nbt

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"slices"
)

const (
	// ChunkStatusFull indicates that a chunk was generated
	ChunkStatusFull = "minecraft:full"

	// DataVersion118 is the data version for Minecraft 1.18
	DataVersion118 = 2816
)

// SlicePool defines the interface for retrieving pre-allocated slices to avoid GC churn.
type SlicePool interface {
	GetInt64Slice() []int64
	PutInt64Slice([]int64)
	GetBlockStateSlice() []BlockState
	PutBlockStateSlice([]BlockState)
}

// ReadLevelVersion extracts the Minecraft version name from a level.dat reader.
func ReadLevelVersion(r io.Reader) (string, error) {
	br, ok := r.(bufferedReader)
	if !ok {
		br = bufio.NewReader(r)
	}
	dr, _ := br.(discarder)
	dec := &decoder{
		r:          br,
		dr:         dr,
		scratchBuf: make([]byte, 256),
	}

	rootType, err := dec.readTagType()
	if err != nil {
		return "", err
	}
	if rootType != TagCompound {
		return "", ErrNotCompound
	}

	// skip root name
	if err := dec.skipString(); err != nil {
		return "", err
	}

	return findVersionName(dec)
}

func findVersionName(dec *decoder) (string, error) {
	for {
		tagType, err := dec.readTagType()
		if err != nil {
			return "", err
		}
		if tagType == TagEnd {
			return "", fmt.Errorf("could not find Data tag")
		}

		tagName, err := dec.readString()
		if err != nil {
			return "", err
		}

		if tagName == "Data" && tagType == TagCompound {
			return findVersionInData(dec)
		}

		if err := dec.skipPayload(tagType); err != nil {
			return "", err
		}
	}
}

func findVersionInData(dec *decoder) (string, error) {
	for {
		tagType, err := dec.readTagType()
		if err != nil {
			return "", err
		}
		if tagType == TagEnd {
			return "", fmt.Errorf("could not find Version tag in Data")
		}

		tagName, err := dec.readString()
		if err != nil {
			return "", err
		}

		if tagName == "Version" && tagType == TagCompound {
			return findNameInVersion(dec)
		}

		if err := dec.skipPayload(tagType); err != nil {
			return "", err
		}
	}
}

func findNameInVersion(dec *decoder) (string, error) {
	for {
		tagType, err := dec.readTagType()
		if err != nil {
			return "", err
		}
		if tagType == TagEnd {
			return "", fmt.Errorf("could not find Name tag in Version")
		}

		tagName, err := dec.readString()
		if err != nil {
			return "", err
		}

		if tagName == "Name" && tagType == TagString {
			return dec.readString()
		}

		if err := dec.skipPayload(tagType); err != nil {
			return "", err
		}
	}
}

// ParseTargetedChunk reads from an NBT reader and extracts only the data needed for rendering.
func ParseTargetedChunk(ctx context.Context, r io.Reader, pool *StringPool, slicePool SlicePool) (*RegionChunk, error) {
	br, ok := r.(bufferedReader)
	if !ok {
		br = bufio.NewReader(r)
	}

	dr, _ := br.(discarder)

	dec := &decoder{
		r:          br,
		dr:         dr,
		pool:       pool,
		slicePool:  slicePool,
		scratchBuf: make([]byte, 256), // Start with a decent capacity to prevent initial grow allocations
	}

	rootType, err := dec.readTagType()
	if err != nil {
		return nil, fmt.Errorf("failed to read root tag type: %w", err)
	}
	if rootType != TagCompound {
		return nil, ErrNotCompound
	}

	// skip root compound name
	if err := dec.skipString(); err != nil {
		return nil, fmt.Errorf("failed to skip root compound name: %w", err)
	}

	chunk := new(RegionChunk)

	// Iterate through root compound tags
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		tagType, err := dec.readTagType()
		if err != nil {
			return nil, fmt.Errorf("failed to read chunk compound tag type: %w", err)
		}

		if tagType == TagEnd {
			break
		}

		tagName, err := dec.readString()
		if err != nil {
			return nil, fmt.Errorf("failed to read chunk compound tag name: %w", err)
		}

		switch tagName {
		case "Status":
			if tagType != TagString {
				return nil, fmt.Errorf("expected string for Status, got %v", tagType)
			}

			status, err := dec.readString()
			if err != nil {
				return nil, fmt.Errorf("failed to read Status string: %w", err)
			}

			chunk.Status = status
			if status != ChunkStatusFull {
				// Fail fast if this chunk isn't yet generated.
				// Clean up any slices we may have already parsed if sections came first.
				if slicePool != nil {
					for _, sec := range chunk.Sections {
						if sec.BlockStates != nil {
							slicePool.PutInt64Slice(sec.BlockStates.Data)
							slicePool.PutBlockStateSlice(sec.BlockStates.Palette)
						}
						if sec.Biomes != nil {
							slicePool.PutInt64Slice(sec.Biomes.Data)
							slicePool.PutBlockStateSlice(sec.Biomes.Palette)
						}
					}
				}
				return nil, nil
			}
		case "DataVersion":
			if tagType != TagInt {
				return nil, fmt.Errorf("expected int for DataVersion, got %v", tagType)
			}

			val, err := dec.readInt32()
			if err != nil {
				return nil, fmt.Errorf("failed to read DataVersion int: %w", err)
			}

			chunk.DataVersion = val
		case "xPos":
			if tagType != TagInt {
				return nil, fmt.Errorf("expected int for xPos, got %v", tagType)
			}

			val, err := dec.readInt32()
			if err != nil {
				return nil, fmt.Errorf("failed to read xPos int: %w", err)
			}

			chunk.XPos = val
		case "zPos":
			if tagType != TagInt {
				return nil, fmt.Errorf("expected int for zPos, got %v", tagType)
			}

			val, err := dec.readInt32()
			if err != nil {
				return nil, fmt.Errorf("failed to read zPos int: %w", err)
			}

			chunk.ZPos = val
		case "sections":
			if tagType != TagList {
				return nil, fmt.Errorf("expected list for sections, got %v", tagType)
			}

			sections, err := dec.parseSectionsList()
			if err != nil {
				return nil, fmt.Errorf("failed to parse sections list: %w", err)
			}

			slices.SortFunc(sections, func(a, b Section) int {
				return int(b.Y) - int(a.Y)
			})

			chunk.Sections = sections
		default:
			// Instantly skip unneeded tags like Entities or Heightmaps
			if err := dec.skipPayload(tagType); err != nil {
				return nil, fmt.Errorf("failed to skip payload for tag %s: %w", tagName, err)
			}
		}
	}

	return chunk, nil
}
