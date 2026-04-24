package nbt

import (
	"encoding/binary"
	"fmt"
	"io"
)

type bufferedReader interface {
	io.Reader
	io.ByteReader
}

type discarder interface {
	Discard(int) (int, error)
}

type decoder struct {
	// r is guaranteed to be a buffered reader to avoid hot-loop interface type assertions which cause
	// measurable CPU overhead when executed millions of times per chunk.
	r bufferedReader

	// dr is cached on init for the same reason: to optimize skipping.
	dr discarder

	pool       *StringPool
	slicePool  SlicePool
	tmp        [8]byte
	scratchBuf []byte
}

func (d *decoder) readByte() (byte, error) {
	return d.r.ReadByte()
}

func (d *decoder) readTagType() (TagType, error) {
	b, err := d.readByte()
	return TagType(b), err
}

func (d *decoder) readUint16() (uint16, error) {
	if _, err := io.ReadFull(d.r, d.tmp[:2]); err != nil {
		return 0, err
	}

	return binary.BigEndian.Uint16(d.tmp[:2]), nil
}

func (d *decoder) readInt16() (int16, error) {
	val, err := d.readUint16()
	
	return int16(val), err
}

func (d *decoder) readInt32() (int32, error) {
	if _, err := io.ReadFull(d.r, d.tmp[:4]); err != nil {
		return 0, err
	}

	
	return int32(binary.BigEndian.Uint32(d.tmp[:4])), nil
}

func (d *decoder) readInt64() (int64, error) {
	if _, err := io.ReadFull(d.r, d.tmp[:8]); err != nil {
		return 0, err
	}

	
	return int64(binary.BigEndian.Uint64(d.tmp[:8])), nil
}

// readString reads a NBT string, which is a uint16 length prefix followed by utf-8 bytes
func (d *decoder) readString() (string, error) {
	length, err := d.readUint16()
	if err != nil {
		return "", err
	}

	if length == 0 {
		return "", nil
	}

	if int(length) > cap(d.scratchBuf) {
		d.scratchBuf = make([]byte, length)
	}
	buf := d.scratchBuf[:length]

	if _, err := io.ReadFull(d.r, buf); err != nil {
		return "", err
	}

	if d.pool != nil {
		return d.pool.InternBytes(buf), nil
	}

	return string(buf), nil
}

// skipPayload advances the decoder past the payload of the given tag type without allocating any memory
// for its contents. Useful for skipping content we don't care about.
func (d *decoder) skipPayload(tagType TagType) error {
	var bytesToSkip int64

	if size := tagType.Size(); size > 0 {
		return d.skip(size)
	}

	switch tagType {
	case TagByteArray:
		length, err := d.readInt32()
		if err != nil {
			return err
		}
		if length < 0 {
			return fmt.Errorf("invalid NBT byte array length: %d", length)
		}
		bytesToSkip = int64(length)
	case TagString:
		length, err := d.readUint16()
		if err != nil {
			return err
		}
		bytesToSkip = int64(length)
	case TagList:
		return d.skipListPayload()
	case TagCompound:
		return d.skipCompoundPayload()
	case TagIntArray:
		length, err := d.readInt32()
		if err != nil {
			return err
		}
		if length < 0 {
			return fmt.Errorf("invalid NBT int array length: %d", length)
		}
		bytesToSkip = int64(length) * 4
	case TagLongArray:
		length, err := d.readInt32()
		if err != nil {
			return err
		}
		if length < 0 {
			return fmt.Errorf("invalid NBT long array length: %d", length)
		}
		bytesToSkip = int64(length) * 8
	}

	return d.skip(bytesToSkip)
}

func (d *decoder) skipListPayload() error {
	itemType, err := d.readByte()
	if err != nil {
		return err
	}

	count, err := d.readInt32()
	if err != nil {
		return err
	}

	if count < 0 {
		return fmt.Errorf("invalid NBT list count: %d", count)
	}

	// Fast path for skipping primitives
	if itemSize := TagType(itemType).Size(); itemSize > 0 {
		return d.skip(int64(count) * itemSize)
	}

	// Fallback for complex types (TagCompound, TagList, TagString, TagByteArray, etc)
	for range count {
		if err := d.skipPayload(TagType(itemType)); err != nil {
			return err
		}
	}
	return nil
}

func (d *decoder) skipCompoundPayload() error {
	for {
		innerType, err := d.readTagType()
		if err != nil {
			return err
		}

		if innerType == TagEnd {
			break
		}

		if err := d.skipString(); err != nil {
			return err
		}

		if err := d.skipPayload(innerType); err != nil {
			return err
		}
	}
	return nil
}

// skipString skips a NBT string without allocating any memory for it.
func (d *decoder) skipString() error {
	length, err := d.readUint16()
	if err != nil {
		return err
	}

	return d.skip(int64(length))
}

func (d *decoder) skip(n int64) error {
	if n <= 0 {
		return nil
	}

	// If the reader supports Discard (like bufio.Reader), use it as this is often much faster than CopyN into io.Discard.
	if d.dr != nil {
		_, err := d.dr.Discard(int(n))
		return err
	}

	_, err := io.CopyN(io.Discard, d.r, n)
	return err
}

func (d *decoder) parseSectionsList() ([]Section, error) {
	tagType, err := d.readTagType()
	if err != nil {
		return nil, err
	}

	count, err := d.readInt32()
	if err != nil {
		return nil, err
	}

	if count < 0 {
		return nil, fmt.Errorf("invalid NBT list count: %d", count)
	}

	// skip parsing non-compound tags as we don't need them.
	if tagType != TagCompound {
		for range count {
			if err := d.skipPayload(tagType); err != nil {
				return nil, err
			}
		}

		return []Section{}, nil
	}

	sections := make([]Section, 0, count)
	for range count {
		section, err := d.parseSection()
		if err != nil {
			return nil, err
		}
		sections = append(sections, section)
	}

	return sections, nil
}

func (d *decoder) parseSection() (Section, error) {
	section := Section{}

	for {
		innerType, err := d.readTagType()
		if err != nil {
			return section, err
		}

		if innerType == TagEnd {
			break
		}

		key, err := d.readString()
		if err != nil {
			return section, err
		}

		switch key {
		case "Y":
			yCoord, err := d.readByte()
			if err != nil {
				return section, err
			}

			
			section.Y = int8(yCoord)
		case "block_states":
			container, err := d.parsePaletteContainer(true)
			if err != nil {
				return section, fmt.Errorf("failed to parse block_states container: %w", err)
			}

			section.BlockStates = container
		case "biomes":
			container, err := d.parsePaletteContainer(false)
			if err != nil {
				return section, fmt.Errorf("failed to parse biomes container: %w", err)
			}

			section.Biomes = container
		default:
			// skip anything else
			if err := d.skipPayload(innerType); err != nil {
				return section, err
			}
		}
	}

	return section, nil
}

func (d *decoder) parsePaletteContainer(isBlock bool) (*PaletteContainer, error) {
	container := &PaletteContainer{}

	for {
		innerType, err := d.readTagType()
		if err != nil {
			return nil, err
		}

		if innerType == TagEnd {
			break
		}

		key, err := d.readString()
		if err != nil {
			return nil, err
		}

		switch key {
		case "data":
			if innerType != TagLongArray {
				// must be a LongArray
				return nil, d.skipPayload(innerType)
			}
			if err := d.parsePaletteData(container); err != nil {
				return nil, err
			}
		case "palette":
			if innerType != TagList {
				// must be a list
				return nil, d.skipPayload(innerType)
			}
			if err := d.parsePaletteList(container, isBlock); err != nil {
				return nil, err
			}
		default:
			// skip anything else
			if err := d.skipPayload(innerType); err != nil {
				return nil, err
			}
		}
	}

	return container, nil
}

func (d *decoder) parsePaletteData(container *PaletteContainer) error {
	length, err := d.readInt32()
	if err != nil {
		return err
	}

	if length < 0 {
		return fmt.Errorf("invalid NBT long array length: %d", length)
	}

	if d.slicePool != nil {
		pooledSlice := d.slicePool.GetInt64Slice()
		if int(length) > cap(pooledSlice) {
			pooledSlice = make([]int64, 0, length)
		}
		container.Data = pooledSlice[:length]
	} else {
		container.Data = make([]int64, length)
	}

	byteLen := int(length) * 8
	if byteLen > cap(d.scratchBuf) {
		d.scratchBuf = make([]byte, byteLen)
	}
	buf := d.scratchBuf[:byteLen]

	if _, err := io.ReadFull(d.r, buf); err != nil {
		return err
	}

	for i := 0; i < int(length); i++ {
		
		container.Data[i] = int64(binary.BigEndian.Uint64(buf[i*8 : i*8+8]))
	}
	return nil
}

func (d *decoder) parsePaletteList(container *PaletteContainer, isBlock bool) error {
	listType, err := d.readTagType()
	if err != nil {
		return err
	}

	length, err := d.readInt32()
	if err != nil {
		return err
	}

	if length < 0 {
		return fmt.Errorf("invalid NBT list count: %d", length)
	}

	if d.slicePool != nil {
		pooledSlice := d.slicePool.GetBlockStateSlice()
		if int(length) > cap(pooledSlice) {
			pooledSlice = make([]BlockState, 0, length)
		}
		container.Palette = pooledSlice[:0]
	} else {
		container.Palette = make([]BlockState, 0, length)
	}

	for range length {
		if isBlock {
			// Block palettes are lists of Compounds
			if listType != TagCompound {
				if err := d.skipPayload(listType); err != nil {
					return err
				}
				continue
			}

			var existing map[string]string
			if len(container.Palette) < cap(container.Palette) {
				existing = container.Palette[:len(container.Palette)+1][len(container.Palette)].Properties
			}

			state, err := d.parseBlockState(existing)
			if err != nil {
				return err
			}
			container.Palette = append(container.Palette, state)
		} else {
			// Biome palettes are lists of Strings
			if listType != TagString {
				if err := d.skipPayload(listType); err != nil {
					return err
				}
				continue
			}

			name, err := d.readString()
			if err != nil {
				return err
			}
			container.Palette = append(container.Palette, BlockState{Name: name})
		}
	}
	return nil
}

func (d *decoder) parseBlockState(existing map[string]string) (BlockState, error) {
	state := BlockState{}

	for {
		innerType, err := d.readTagType()
		if err != nil {
			return state, err
		}

		if innerType == TagEnd {
			break
		}

		key, err := d.readString()
		if err != nil {
			return state, err
		}

		switch key {
		case "Name":
			name, err := d.readString()
			if err != nil {
				return state, err
			}
			state.Name = name
		case "Properties":
			if innerType != TagCompound {
				if err := d.skipPayload(innerType); err != nil {
					return state, err
				}
				continue
			}

			if existing != nil {
				state.Properties = existing
			} else {
				state.Properties = make(map[string]string)
			}

			if err := d.parseBlockProperties(&state); err != nil {
				return state, err
			}
		default:
			if err := d.skipPayload(innerType); err != nil {
				return state, err
			}
		}
	}

	return state, nil
}

func (d *decoder) parseBlockProperties(state *BlockState) error {
	for {
		propType, err := d.readTagType()
		if err != nil {
			return err
		}
		if propType == TagEnd {
			break
		}

		propKey, err := d.readString()
		if err != nil {
			return err
		}

		// property values are always strings
		if propType != TagString {
			if err := d.skipPayload(propType); err != nil {
				return err
			}
			continue
		}

		propValue, err := d.readString()
		if err != nil {
			return err
		}

		state.Properties[propKey] = propValue
	}
	return nil
}
