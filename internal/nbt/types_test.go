package nbt

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTagType_Size(t *testing.T) {
	tests := []struct {
		tag  TagType
		want int64
	}{
		{TagByte, 1},
		{TagShort, 2},
		{TagInt, 4},
		{TagFloat, 4},
		{TagLong, 8},
		{TagDouble, 8},
		{TagEnd, 0},
		{TagString, 0},
		{TagList, 0},
		{TagCompound, 0},
		{TagByteArray, 0},
		{TagIntArray, 0},
		{TagLongArray, 0},
	}

	for _, tt := range tests {
		t.Run(string(rune(tt.tag)), func(t *testing.T) {
			assert.Equal(t, tt.want, tt.tag.Size())
		})
	}
}
