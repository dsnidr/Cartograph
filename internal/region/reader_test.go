package region

import (
	"bytes"
	"compress/zlib"
	"testing"

	"github.com/dsnidr/cartograph/internal/nbt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockRegionFile generates a fake .mca file buffer
func mockRegionFile(t *testing.T, locations []chunkLocation, compression byte, payload []byte) *bytes.Reader {
	t.Helper()
	var buf bytes.Buffer

	// Write 4KiB header
	header := make([]byte, sectorBytes)
	for i, loc := range locations {
		if i >= maxChunksPerRegion {
			break
		}
		offsetIdx := i * chunkHeaderSize
		sectors := loc.Offset / sectorBytes
		header[offsetIdx] = byte(sectors >> 16)
		header[offsetIdx+1] = byte(sectors >> 8)
		header[offsetIdx+2] = byte(sectors)
		header[offsetIdx+3] = byte(loc.Length / sectorBytes)
	}
	buf.Write(header)

	for _, loc := range locations {
		if loc.Offset == 0 {
			continue
		}

		// pad to chunk offset
		padding := int(loc.Offset) - buf.Len()
		if padding > 0 && padding < 1<<20 {
			buf.Write(make([]byte, padding))
		}

		// payload length +1 extra byte to for the compression scheme
		length := uint32(len(payload) + 1)
		buf.Write([]byte{
			byte(length >> 24),
			byte(length >> 16),
			byte(length >> 8),
			byte(length),
		})

		buf.WriteByte(compression)
		buf.Write(payload)
	}

	return bytes.NewReader(buf.Bytes())
}

// createZlibNBT creates a valid NBT payload compressed with zlib
func createZlibNBT(t *testing.T, status string) []byte {
	t.Helper()
	var nbtBuf bytes.Buffer
	nbtBuf.Write([]byte{byte(nbt.TagCompound)})
	nbtBuf.Write([]byte{0, 0}) // empty name
	nbtBuf.Write([]byte{byte(nbt.TagString)})
	nbtBuf.Write([]byte{0, byte(len("Status"))})
	nbtBuf.WriteString("Status")
	nbtBuf.Write([]byte{0, byte(len(status))})
	nbtBuf.WriteString(status)
	nbtBuf.Write([]byte{byte(nbt.TagEnd)})

	var zlibBuf bytes.Buffer
	zw := zlib.NewWriter(&zlibBuf)
	_, err := zw.Write(nbtBuf.Bytes())
	require.NoError(t, err)
	require.NoError(t, zw.Close())

	return zlibBuf.Bytes()
}

func TestParse(t *testing.T) {
	pool := nbt.NewStringPool()
	validNBT := createZlibNBT(t, "minecraft:full")
	invalidNBT := createZlibNBT(t, "minecraft:empty")

	tests := []struct {
		name        string
		locations   []chunkLocation
		compression byte
		payload     []byte
		shortRead   bool
		wantErr     bool
		wantChunks  int
	}{
		{
			name: "returns empty region when header is all zeros",
			locations: []chunkLocation{
				{Offset: 0, Length: 0},
			},
			wantErr:    false,
			wantChunks: 0,
		},
		{
			name:      "fails when reader is too short for header",
			shortRead: true,
			wantErr:   true,
		},
		{
			name: "returns valid chunk",
			locations: []chunkLocation{
				{Offset: sectorBytes, Length: sectorBytes},
			},
			compression: compressionZlib,
			payload:     validNBT,
			wantErr:     false,
			wantChunks:  1,
		},
		{
			name: "skips chunk when status is not minecraft:full",
			locations: []chunkLocation{
				{Offset: sectorBytes, Length: sectorBytes},
			},
			compression: compressionZlib,
			payload:     invalidNBT,
			wantErr:     false,
			wantChunks:  0,
		},
		{
			name: "skips chunk with unsupported compression",
			locations: []chunkLocation{
				{Offset: sectorBytes, Length: sectorBytes},
			},
			compression: compressionGzip,
			payload:     validNBT,
			wantErr:     false,
			wantChunks:  0,
		},
		{
			name: "skips chunk with invalid zlib payload",
			locations: []chunkLocation{
				{Offset: sectorBytes, Length: sectorBytes},
			},
			compression: compressionZlib,
			payload:     []byte("not zlib data"),
			wantErr:     false,
			wantChunks:  0,
		},
		{
			name: "skips chunk with valid zlib but invalid NBT",
			locations: []chunkLocation{
				{Offset: sectorBytes, Length: sectorBytes},
			},
			compression: compressionZlib,
			payload: func() []byte {
				var b bytes.Buffer
				zw := zlib.NewWriter(&b)
				zw.Write([]byte("not NBT data"))
				zw.Close()
				return b.Bytes()
			}(),
			wantErr:    false,
			wantChunks: 0,
		},
		{
			name: "fails when seeking to chunk fails",
			locations: []chunkLocation{
				{Offset: 1 << 30, Length: sectorBytes}, // Large offset out of bounds of the actual byte buffer
			},
			compression: compressionZlib,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var r *bytes.Reader
			if tt.shortRead {
				r = bytes.NewReader(make([]byte, 100))
			} else {
				r = mockRegionFile(t, tt.locations, tt.compression, tt.payload)
			}

			region, err := Parse(t.Context(), r, pool)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, region)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, region)
			assert.Len(t, region.Chunks, tt.wantChunks)
		})
	}
}
