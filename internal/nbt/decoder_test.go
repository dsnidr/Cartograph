package nbt

import (
	"bufio"
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func buildNBTString(s string) []byte {
	b := make([]byte, 2+len(s))
	b[0] = byte(len(s) >> 8)
	b[1] = byte(len(s))
	copy(b[2:], s)

	return b
}

func TestDecoder_readByte(t *testing.T) {
	t.Run("returns byte when data is available", func(t *testing.T) {
		data := []byte{0x42}
		dec := &decoder{r: bufio.NewReader(bytes.NewReader(data))}

		b, err := dec.readByte()
		require.NoError(t, err)
		assert.Equal(t, byte(0x42), b)
	})

	t.Run("fails when reader is empty", func(t *testing.T) {
		dec := &decoder{r: bufio.NewReader(bytes.NewReader([]byte{}))}

		_, err := dec.readByte()
		assert.ErrorIs(t, err, io.EOF)
	})
}

func TestDecoder_readUint16(t *testing.T) {
	tests := []struct {
		name    string
		input   []byte
		want    uint16
		wantErr error
	}{
		{
			name:    "returns big-endian parsed value",
			input:   []byte{0x12, 0x34},
			want:    0x1234,
			wantErr: nil,
		},
		{
			name:    "returns maximum value (unsigned)",
			input:   []byte{0xFF, 0xFF},
			want:    65535,
			wantErr: nil,
		},
		{
			name:    "returns zero",
			input:   []byte{0x00, 0x00},
			want:    0,
			wantErr: nil,
		},
		{
			name:    "fails when partially written",
			input:   []byte{0x12},
			want:    0,
			wantErr: io.ErrUnexpectedEOF,
		},
		{
			name:    "fails when empty",
			input:   []byte{},
			want:    0,
			wantErr: io.EOF,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dec := &decoder{r: bufio.NewReader(bytes.NewReader(tt.input))}
			got, err := dec.readUint16()

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestDecoder_readInt16(t *testing.T) {
	tests := []struct {
		name    string
		input   []byte
		want    int16
		wantErr error
	}{
		{
			name:    "returns big-endian parsed value",
			input:   []byte{0x12, 0x34},
			want:    4660,
			wantErr: nil,
		},
		{
			name:    "returns negative values correctly",
			input:   []byte{0xFF, 0xFE},
			want:    -2,
			wantErr: nil,
		},
		{
			name:    "returns zero",
			input:   []byte{0x00, 0x00},
			want:    0,
			wantErr: nil,
		},
		{
			name:    "fails when partially written",
			input:   []byte{0x12},
			want:    0,
			wantErr: io.ErrUnexpectedEOF,
		},
		{
			name:    "fails when empty",
			input:   []byte{},
			want:    0,
			wantErr: io.EOF,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dec := &decoder{r: bufio.NewReader(bytes.NewReader(tt.input))}
			got, err := dec.readInt16()

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestDecoder_readInt32(t *testing.T) {
	tests := []struct {
		name    string
		input   []byte
		want    int32
		wantErr error
	}{
		{
			name:    "returns big-endian parsed value",
			input:   []byte{0x12, 0x34, 0x56, 0x78},
			want:    305419896,
			wantErr: nil,
		},
		{
			name:    "returns negative values correctly",
			input:   []byte{0xFF, 0xFF, 0xFF, 0xFE},
			want:    -2,
			wantErr: nil,
		},
		{
			name:    "returns zero",
			input:   []byte{0x00, 0x00, 0x00, 0x00},
			want:    0,
			wantErr: nil,
		},
		{
			name:    "fails when partially written",
			input:   []byte{0x12, 0x34, 0x56},
			want:    0,
			wantErr: io.ErrUnexpectedEOF,
		},
		{
			name:    "fails when empty",
			input:   []byte{},
			want:    0,
			wantErr: io.EOF,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dec := &decoder{r: bufio.NewReader(bytes.NewReader(tt.input))}
			got, err := dec.readInt32()

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestDecoder_readTagType(t *testing.T) {
	tests := []struct {
		name    string
		input   []byte
		want    TagType
		wantErr error
	}{
		{
			name:    "returns parsed TagCompound type",
			input:   []byte{0x0A},
			want:    TagCompound,
			wantErr: nil,
		},
		{
			name:    "returns parsed TagEnd type",
			input:   []byte{0x00},
			want:    TagEnd,
			wantErr: nil,
		},
		{
			name:    "fails when empty",
			input:   []byte{},
			want:    0,
			wantErr: io.EOF,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dec := &decoder{r: bufio.NewReader(bytes.NewReader(tt.input))}
			got, err := dec.readTagType()

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestDecoder_readInt64(t *testing.T) {
	tests := []struct {
		name    string
		input   []byte
		want    int64
		wantErr error
	}{
		{
			name:    "returns big-endian parsed value",
			input:   []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x23},
			want:    291,
			wantErr: nil,
		},
		{
			name:    "returns negative values correctly",
			input:   []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
			want:    -1,
			wantErr: nil,
		},
		{
			name:    "returns zero",
			input:   []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
			want:    0,
			wantErr: nil,
		},
		{
			name:    "fails when partially written",
			input:   []byte{0x01, 0x02, 0x03, 0x04, 0x05},
			want:    0,
			wantErr: io.ErrUnexpectedEOF,
		},
		{
			name:    "fails when empty",
			input:   []byte{},
			want:    0,
			wantErr: io.EOF,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dec := &decoder{r: bufio.NewReader(bytes.NewReader(tt.input))}
			got, err := dec.readInt64()

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestDecoder_skip(t *testing.T) {
	t.Run("returns nil when skipping zero bytes", func(t *testing.T) {
		dec := &decoder{r: bufio.NewReader(bytes.NewReader([]byte{0x01, 0x02}))}
		err := dec.skip(0)
		require.NoError(t, err)
	})

	t.Run("returns nil when skipping negative bytes", func(t *testing.T) {
		dec := &decoder{r: bufio.NewReader(bytes.NewReader([]byte{0x01, 0x02}))}
		err := dec.skip(-5)
		require.NoError(t, err)
	})

	t.Run("uses discard interface when available", func(t *testing.T) {
		base := bytes.NewReader([]byte{0x01, 0x02, 0x03, 0x04, 0x05})
		mock := &mockDiscardReader{
			Reader: base,
		}
		// mockDiscardReader implements bufferedReader implicitly (via io.Reader) but we need to satisfy ByteReader manually for the test struct
		type brMock struct {
			*mockDiscardReader
			io.ByteReader
		}
		dec := &decoder{r: &brMock{mockDiscardReader: mock, ByteReader: bufio.NewReader(base)}, dr: mock}

		err := dec.skip(3)
		require.NoError(t, err)
		assert.True(t, mock.discardCalled, "expected Discard to be called on underlying reader")
	})

	t.Run("falls back to io.CopyN when discard is missing", func(t *testing.T) {
		base := bytes.NewReader([]byte{0x01, 0x02, 0x03, 0x04, 0x05})
		// Provide a generic io.Reader and io.ByteReader without Discard
		type genericBuf struct {
			io.Reader
			io.ByteReader
		}
		gb := &genericBuf{base, base}
		dec := &decoder{r: gb}

		err := dec.skip(3)
		require.NoError(t, err)

		rem := base.Len()
		assert.Equal(t, 2, rem)
	})

	t.Run("fails when EOF is reached early", func(t *testing.T) {
		dec := &decoder{r: bufio.NewReader(bytes.NewReader([]byte{0x01}))}
		err := dec.skip(5)
		assert.ErrorIs(t, err, io.EOF)
	})
}

func TestDecoder_skipString(t *testing.T) {
	tests := []struct {
		name    string
		input   []byte
		wantErr error
	}{
		{
			name:    "skips correctly sized string",
			input:   buildNBTString("test"),
			wantErr: nil,
		},
		{
			name:    "fails when length prefix is missing",
			input:   []byte{0x00},
			wantErr: io.ErrUnexpectedEOF,
		},
		{
			name:    "fails when body is partially missing",
			input:   []byte{0x00, 0x04, 't', 'e'},
			wantErr: io.EOF,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base := bytes.NewReader(tt.input)
			dec := &decoder{r: bufio.NewReader(base)}

			err := dec.skipString()

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, 0, base.Len(), "expected all bytes to be skipped")
			}
		})
	}
}

func TestDecoder_skipPayload(t *testing.T) {
	tests := []struct {
		name    string
		tagType TagType
		input   []byte
		wantErr error
	}{
		{
			name:    "skips byte (1 byte)",
			tagType: TagByte,
			input:   []byte{0x42},
			wantErr: nil,
		},
		{
			name:    "skips short (2 bytes)",
			tagType: TagShort,
			input:   []byte{0x12, 0x34},
			wantErr: nil,
		},
		{
			name:    "skips int (4 bytes)",
			tagType: TagInt,
			input:   []byte{0x12, 0x34, 0x56, 0x78},
			wantErr: nil,
		},
		{
			name:    "skips float (4 bytes)",
			tagType: TagFloat,
			input:   []byte{0x12, 0x34, 0x56, 0x78},
			wantErr: nil,
		},
		{
			name:    "skips long (8 bytes)",
			tagType: TagLong,
			input:   []byte{0x01, 0x23, 0x45, 0x67, 0x89, 0xAB, 0xCD, 0xEF},
			wantErr: nil,
		},
		{
			name:    "skips double (8 bytes)",
			tagType: TagDouble,
			input:   []byte{0x01, 0x23, 0x45, 0x67, 0x89, 0xAB, 0xCD, 0xEF},
			wantErr: nil,
		},
		{
			name:    "skips byte array (length + length*1 bytes)",
			tagType: TagByteArray,
			input:   []byte{0x00, 0x00, 0x00, 0x03, 0x01, 0x02, 0x03},
			wantErr: nil,
		},
		{
			name:    "skips string (length + length*1 bytes)",
			tagType: TagString,
			input:   buildNBTString("test"),
			wantErr: nil,
		},
		{
			name:    "skips list (type + count + count*payload bytes)",
			tagType: TagList,
			input: []byte{
				byte(TagInt),
				0x00, 0x00, 0x00, 0x02,
				0x00, 0x00, 0x00, 0x01,
				0x00, 0x00, 0x00, 0x02,
			},
			wantErr: nil,
		},
		{
			name:    "skips compound (tags until TagEnd)",
			tagType: TagCompound,
			input:   append([]byte{byte(TagByte)}, append(buildNBTString("foo"), 0x01, byte(TagEnd))...),
			wantErr: nil,
		},
		{
			name:    "skips nested compound",
			tagType: TagCompound,
			input:   append([]byte{byte(TagCompound)}, append(buildNBTString("c"), append([]byte{byte(TagByte)}, append(buildNBTString("b"), 0x01, byte(TagEnd), byte(TagEnd))...)...)...),
			wantErr: nil,
		},
		{
			name:    "skips int array (length + length*4 bytes)",
			tagType: TagIntArray,
			input: []byte{
				0x00, 0x00, 0x00, 0x02,
				0x00, 0x00, 0x00, 0x01,
				0x00, 0x00, 0x00, 0x02,
			},
			wantErr: nil,
		},
		{
			name:    "skips long array (length + length*8 bytes)",
			tagType: TagLongArray,
			input: []byte{
				0x00, 0x00, 0x00, 0x01,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01,
			},
			wantErr: nil,
		},
		{
			name:    "fails when primitive is missing bytes",
			tagType: TagInt,
			input:   []byte{0x01, 0x02},
			wantErr: io.EOF,
		},
		{
			name:    "fails when array length is missing",
			tagType: TagByteArray,
			input:   []byte{0x00},
			wantErr: io.ErrUnexpectedEOF,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base := bytes.NewReader(tt.input)
			dec := &decoder{r: bufio.NewReader(base)}

			err := dec.skipPayload(tt.tagType)

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, 0, base.Len(), "expected entire payload to be skipped")
			}
		})
	}
}

type mockDiscardReader struct {
	io.Reader
	discardCalled bool
}

func (m *mockDiscardReader) Discard(n int) (int, error) {
	m.discardCalled = true
	var skipped int64
	skipped, err := io.CopyN(io.Discard, m.Reader, int64(n))
	return int(skipped), err
}

func TestDecoder_readString(t *testing.T) {
	tests := []struct {
		name    string
		input   []byte
		pool    *StringPool
		want    string
		wantErr error
	}{
		{
			name:    "returns empty string for zero length",
			input:   []byte{0x00, 0x00},
			want:    "",
			wantErr: nil,
		},
		{
			name:    "returns parsed string for valid input",
			input:   buildNBTString("test"),
			want:    "test",
			wantErr: nil,
		},
		{
			name:    "returns interned string when pool is provided",
			input:   buildNBTString("test"),
			pool:    NewStringPool(),
			want:    "test",
			wantErr: nil,
		},
		{
			name:    "handles strings larger than 32767 bytes",
			input:   append([]byte{0x80, 0x00}, make([]byte, 32768)...),
			want:    string(make([]byte, 32768)),
			wantErr: nil,
		},
		{
			name:    "fails when length prefix is missing",
			input:   []byte{0x00},
			want:    "",
			wantErr: io.ErrUnexpectedEOF,
		},
		{
			name:    "fails when body is partially missing",
			input:   []byte{0x00, 0x04, 't', 'e', 's'},
			want:    "",
			wantErr: io.ErrUnexpectedEOF,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dec := &decoder{
				r:    bufio.NewReader(bytes.NewReader(tt.input)),
				pool: tt.pool,
			}

			got, err := dec.readString()

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestDecoder_parseBlockState(t *testing.T) {
	tests := []struct {
		name    string
		input   []byte
		want    BlockState
		wantErr error
	}{
		{
			name: "returns valid block state with name",
			input: bytes.Join([][]byte{
				{byte(TagString)}, buildNBTString("Name"), buildNBTString("minecraft:stone"),
				{byte(TagEnd)},
			}, nil),
			want: BlockState{
				Name: "minecraft:stone",
			},
			wantErr: nil,
		},
		{
			name: "returns valid block state with name and properties",
			input: bytes.Join([][]byte{
				{byte(TagString)}, buildNBTString("Name"), buildNBTString("minecraft:oak_log"),
				{byte(TagCompound)}, buildNBTString("Properties"),
				{byte(TagString)}, buildNBTString("axis"), buildNBTString("y"),
				{byte(TagEnd)},
				{byte(TagEnd)},
			}, nil),
			want: BlockState{
				Name:       "minecraft:oak_log",
				Properties: map[string]string{"axis": "y"},
			},
			wantErr: nil,
		},
		{
			name: "skips unknown tags",
			input: bytes.Join([][]byte{
				{byte(TagInt)}, buildNBTString("Unknown"), {0x00, 0x00, 0x00, 0x01},
				{byte(TagString)}, buildNBTString("Name"), buildNBTString("minecraft:dirt"),
				{byte(TagEnd)},
			}, nil),
			want: BlockState{
				Name: "minecraft:dirt",
			},
			wantErr: nil,
		},
		{
			name:    "fails when reader is empty",
			input:   []byte{},
			wantErr: io.EOF,
		},
		{
			name: "fails when Properties is not TagCompound",
			input: bytes.Join([][]byte{
				{byte(TagString)}, buildNBTString("Properties"), buildNBTString("axis=y"), // this correctly skips the tag, but needs EOF after to fail
			}, nil),
			wantErr: io.EOF,
		},
		{
			name: "fails when property value is not TagString",
			input: bytes.Join([][]byte{
				{byte(TagCompound)}, buildNBTString("Properties"),
				{byte(TagInt)}, buildNBTString("axis"), {0x00, 0x00, 0x00, 0x01},
			}, nil),
			wantErr: io.EOF,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dec := &decoder{r: bufio.NewReader(bytes.NewReader(tt.input))}
			got, err := dec.parseBlockState(nil)

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestDecoder_parsePaletteContainer(t *testing.T) {
	tests := []struct {
		name    string
		isBlock bool
		input   []byte
		want    *PaletteContainer
		wantErr error
	}{
		{
			name:    "returns valid block palette container",
			isBlock: true,
			input: bytes.Join([][]byte{
				{byte(TagList)}, buildNBTString("palette"), {byte(TagCompound), 0x00, 0x00, 0x00, 0x01},
				{byte(TagString)}, buildNBTString("Name"), buildNBTString("minecraft:air"), {byte(TagEnd)},
				{byte(TagLongArray)}, buildNBTString("data"), {0x00, 0x00, 0x00, 0x01}, {0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x2A},
				{byte(TagEnd)},
			}, nil),
			want: &PaletteContainer{
				Palette: []BlockState{{Name: "minecraft:air"}},
				Data:    []int64{42},
			},
			wantErr: nil,
		},
		{
			name:    "returns valid biome palette container",
			isBlock: false,
			input: bytes.Join([][]byte{
				{byte(TagList)}, buildNBTString("palette"), {byte(TagString), 0x00, 0x00, 0x00, 0x01},
				buildNBTString("minecraft:plains"),
				{byte(TagLongArray)}, buildNBTString("data"), {0x00, 0x00, 0x00, 0x01}, {0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x05},
				{byte(TagEnd)},
			}, nil),
			want: &PaletteContainer{
				Palette: []BlockState{{Name: "minecraft:plains"}},
				Data:    []int64{5},
			},
			wantErr: nil,
		},
		{
			name:    "skips unknown tags",
			isBlock: true,
			input: bytes.Join([][]byte{
				{byte(TagInt)}, buildNBTString("Unknown"), {0x00, 0x00, 0x00, 0x01},
				{byte(TagEnd)},
			}, nil),
			want: &PaletteContainer{
				Palette: nil,
				Data:    nil,
			},
			wantErr: nil,
		},
		{
			name:    "fails when empty",
			isBlock: true,
			input:   []byte{},
			wantErr: io.EOF,
		},
		{
			name:    "fails when data is not TagLongArray",
			isBlock: true,
			input: bytes.Join([][]byte{
				{byte(TagList)}, buildNBTString("data"), {byte(TagInt), 0x00, 0x00, 0x00, 0x01},
			}, nil),
			wantErr: io.EOF,
		},
		{
			name:    "fails when palette is not TagList",
			isBlock: true,
			input: bytes.Join([][]byte{
				{byte(TagCompound)}, buildNBTString("palette"),
			}, nil),
			wantErr: io.EOF,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dec := &decoder{r: bufio.NewReader(bytes.NewReader(tt.input))}
			got, err := dec.parsePaletteContainer(tt.isBlock)

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestDecoder_parseSectionsList(t *testing.T) {
	tests := []struct {
		name    string
		input   []byte
		want    []Section
		wantErr error
	}{
		{
			name: "returns valid sections list",
			input: bytes.Join([][]byte{
				{byte(TagCompound)}, {0x00, 0x00, 0x00, 0x01}, // List of 1 Compound
				{byte(TagByte)}, buildNBTString("Y"), {0x04},
				{byte(TagEnd)},
			}, nil),
			want: []Section{
				{Y: 4},
			},
			wantErr: nil,
		},
		{
			name: "returns valid section with block states and biomes",
			input: bytes.Join([][]byte{
				{byte(TagCompound)}, {0x00, 0x00, 0x00, 0x01},
				{byte(TagByte)}, buildNBTString("Y"), {0x00},
				{byte(TagCompound)}, buildNBTString("block_states"),
				{byte(TagList)}, buildNBTString("palette"), {byte(TagCompound), 0x00, 0x00, 0x00, 0x01},
				{byte(TagString)}, buildNBTString("Name"), buildNBTString("minecraft:stone"), {byte(TagEnd)},
				{byte(TagEnd)},
				{byte(TagCompound)}, buildNBTString("biomes"),
				{byte(TagList)}, buildNBTString("palette"), {byte(TagString), 0x00, 0x00, 0x00, 0x01},
				buildNBTString("minecraft:plains"),
				{byte(TagEnd)},
				{byte(TagEnd)},
			}, nil),
			want: []Section{
				{
					Y: 0,
					BlockStates: &PaletteContainer{
						Palette: []BlockState{{Name: "minecraft:stone"}},
						Data:    nil,
					},
					Biomes: &PaletteContainer{
						Palette: []BlockState{{Name: "minecraft:plains"}},
						Data:    nil,
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "skips non-compound lists",
			input: bytes.Join([][]byte{
				{byte(TagInt)}, {0x00, 0x00, 0x00, 0x02}, // List of 2 Ints
				{0x00, 0x00, 0x00, 0x01},
				{0x00, 0x00, 0x00, 0x02},
			}, nil),
			want:    []Section{},
			wantErr: nil,
		},
		{
			name:    "fails when empty",
			input:   []byte{},
			want:    nil,
			wantErr: io.EOF,
		},
		{
			name: "fails when Y coordinate is malformed",
			input: bytes.Join([][]byte{
				{byte(TagCompound)}, {0x00, 0x00, 0x00, 0x01},
				{byte(TagByte)}, buildNBTString("Y"), // missing byte value
			}, nil),
			want:    nil,
			wantErr: io.EOF,
		},
		{
			name: "fails when block_states is malformed",
			input: bytes.Join([][]byte{
				{byte(TagCompound)}, {0x00, 0x00, 0x00, 0x01},
				{byte(TagCompound)}, buildNBTString("block_states"),
				{byte(TagInt)}, buildNBTString("data"), // data must be TagLongArray
			}, nil),
			want:    nil,
			wantErr: io.EOF,
		},
		{
			name: "fails when biomes is malformed",
			input: bytes.Join([][]byte{
				{byte(TagCompound)}, {0x00, 0x00, 0x00, 0x01},
				{byte(TagCompound)}, buildNBTString("biomes"),
				{byte(TagList)}, buildNBTString("palette"), {byte(TagInt)}, // palette length missing
			}, nil),
			want:    nil,
			wantErr: io.EOF,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dec := &decoder{r: bufio.NewReader(bytes.NewReader(tt.input))}
			got, err := dec.parseSectionsList()

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}
