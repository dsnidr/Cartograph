package nbt

// TagType represents an NBT tag type byte.
type TagType byte

// Size returns the fixed byte size of the primitive tag type, or 0 if the tag has a variable size.
func (t TagType) Size() int64 {
	switch t {
	case TagByte:
		return 1
	case TagShort:
		return 2
	case TagInt, TagFloat:
		return 4
	case TagLong, TagDouble:
		return 8
	default:
		return 0
	}
}

// Minecraft NBT tag type bytes
const (
	TagEnd TagType = iota
	TagByte
	TagShort
	TagInt
	TagLong
	TagFloat
	TagDouble
	TagByteArray
	TagString
	TagList
	TagCompound
	TagIntArray
	TagLongArray
)
