package nbt

import (
	"bytes"
	"context"
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func encInt32(v int32) []byte {
	return binary.BigEndian.AppendUint32(nil, uint32(v))
}

func encLong(v int64) []byte {
	return binary.BigEndian.AppendUint64(nil, uint64(v))
}

func TestParseTargetedChunk(t *testing.T) {
	pool := NewStringPool()

	tests := []struct {
		name    string
		input   []byte
		want    *RegionChunk
		wantErr bool
	}{
		{
			name: "returns chunk when status is full",
			input: bytes.Join([][]byte{
				{byte(TagCompound)}, buildNBTString(""),
				{byte(TagString)}, buildNBTString("Status"), buildNBTString("minecraft:full"),
				{byte(TagInt)}, buildNBTString("DataVersion"), encInt32(DataVersion118),
				{byte(TagInt)}, buildNBTString("xPos"), encInt32(1),
				{byte(TagInt)}, buildNBTString("zPos"), encInt32(2),
				{byte(TagList)}, buildNBTString("sections"), {byte(TagCompound)}, encInt32(1),
				{byte(TagByte)}, buildNBTString("Y"), {0x04},
				{byte(TagEnd)},
				{byte(TagEnd)},
			}, nil),
			want: &RegionChunk{
				Status:      "minecraft:full",
				DataVersion: DataVersion118,
				XPos:        1,
				ZPos:        2,
				Sections: []Section{
					{Y: 4},
				},
			},
		},
		{
			name: "returns nil when status is not full",
			input: bytes.Join([][]byte{
				{byte(TagCompound)}, buildNBTString(""),
				{byte(TagString)}, buildNBTString("Status"), buildNBTString("minecraft:empty"),
				{byte(TagEnd)},
			}, nil),
			want: nil,
		},
		{
			name: "fails when root is not compound",
			input: bytes.Join([][]byte{
				{byte(TagInt)}, buildNBTString("Status"), encInt32(1),
			}, nil),
			wantErr: true,
		},
		{
			name: "fails when Status is not a string",
			input: bytes.Join([][]byte{
				{byte(TagCompound)}, buildNBTString(""),
				{byte(TagInt)}, buildNBTString("Status"), encInt32(1),
				{byte(TagEnd)},
			}, nil),
			wantErr: true,
		},
		{
			name: "fails when DataVersion is not an int",
			input: bytes.Join([][]byte{
				{byte(TagCompound)}, buildNBTString(""),
				{byte(TagString)}, buildNBTString("DataVersion"), buildNBTString("1.18"),
				{byte(TagEnd)},
			}, nil),
			wantErr: true,
		},
		{
			name: "fails when xPos is not an int",
			input: bytes.Join([][]byte{
				{byte(TagCompound)}, buildNBTString(""),
				{byte(TagString)}, buildNBTString("Status"), buildNBTString("minecraft:full"),
				{byte(TagString)}, buildNBTString("xPos"), buildNBTString("1"),
				{byte(TagEnd)},
			}, nil),
			wantErr: true,
		},
		{
			name: "fails when zPos is not an int",
			input: bytes.Join([][]byte{
				{byte(TagCompound)}, buildNBTString(""),
				{byte(TagString)}, buildNBTString("Status"), buildNBTString("minecraft:full"),
				{byte(TagString)}, buildNBTString("zPos"), buildNBTString("2"),
				{byte(TagEnd)},
			}, nil),
			wantErr: true,
		},
		{
			name: "fails when sections is not a list",
			input: bytes.Join([][]byte{
				{byte(TagCompound)}, buildNBTString(""),
				{byte(TagString)}, buildNBTString("Status"), buildNBTString("minecraft:full"),
				{byte(TagCompound)}, buildNBTString("sections"), {byte(TagEnd)},
				{byte(TagEnd)},
			}, nil),
			wantErr: true,
		},
		{
			name: "skips unneeded tags",
			input: bytes.Join([][]byte{
				{byte(TagCompound)}, buildNBTString(""),
				{byte(TagString)}, buildNBTString("Status"), buildNBTString("minecraft:full"),
				{byte(TagLong)}, buildNBTString("InhabitedTime"), encLong(12345),
				{byte(TagEnd)},
			}, nil),
			want: &RegionChunk{
				Status: "minecraft:full",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseTargetedChunk(t.Context(), bytes.NewReader(tt.input), pool, nil)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestParseTargetedChunk_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	input := bytes.Join([][]byte{
		{byte(TagCompound)}, buildNBTString(""),
		{byte(TagString)}, buildNBTString("Status"), buildNBTString("minecraft:full"),
		{byte(TagEnd)},
	}, nil)

	_, err := ParseTargetedChunk(ctx, bytes.NewReader(input), nil, nil)
	assert.ErrorIs(t, err, context.Canceled)
}
