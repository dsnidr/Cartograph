package nbt

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBlockState_HasProperty(t *testing.T) {
	tests := []struct {
		name       string
		properties map[string]string
		key        string
		value      string
		want       bool
	}{
		{
			name:       "returns true when property matches",
			properties: map[string]string{"axis": "y"},
			key:        "axis",
			value:      "y",
			want:       true,
		},
		{
			name:       "returns false when property value differs",
			properties: map[string]string{"axis": "y"},
			key:        "axis",
			value:      "x",
			want:       false,
		},
		{
			name:       "returns false when key is missing",
			properties: map[string]string{"axis": "y"},
			key:        "waterlogged",
			value:      "true",
			want:       false,
		},
		{
			name:       "returns false when properties are nil",
			properties: nil,
			key:        "axis",
			value:      "y",
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &BlockState{Properties: tt.properties}
			assert.Equal(t, tt.want, b.HasProperty(tt.key, tt.value))
		})
	}
}

func TestBlockState_String(t *testing.T) {
	tests := []struct {
		name       string
		blockName  string
		properties map[string]string
		want       string
	}{
		{
			name:      "returns name only when no properties exist",
			blockName: "minecraft:stone",
			want:      "minecraft:stone",
		},
		{
			name:      "returns name and properties when present",
			blockName: "minecraft:oak_log",
			properties: map[string]string{
				"axis": "y",
			},
			want: "minecraft:oak_log[axis=y]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &BlockState{Name: tt.blockName, Properties: tt.properties}
			assert.Equal(t, tt.want, b.String())
		})
	}
}

func TestPaletteContainer_GetState(t *testing.T) {
	// packData packs multiple NBT palette indices into a slice of 64-bit longs. Entries cannot span
	// multiple longs since Minecraft 1.16, so any remaining bits at the end of a long are ignored
	// and the next entry starts at bit 0 of the next long.
	packData := func(bitsPerEntry int, values ...int) []int64 {
		entriesPerLong := 64 / bitsPerEntry // remainder is ignored
		data := make([]int64, (len(values)/entriesPerLong)+1)
		for i, v := range values {
			longIndex := i / entriesPerLong
			bitOffset := (i % entriesPerLong) * bitsPerEntry

			// shift to correct bit position and OR the value into the long
			data[longIndex] |= int64(v) << bitOffset
		}

		return data
	}

	tests := []struct {
		name         string
		palette      []BlockState
		data         []int64
		index        int
		bitsPerEntry int
		want         *BlockState
	}{
		{
			name:    "returns nil when palette is empty",
			palette: []BlockState{},
			want:    nil,
		},
		{
			name:    "returns first element when data is missing",
			palette: []BlockState{{Name: "stone"}},
			data:    nil,
			want:    &BlockState{Name: "stone"},
		},
		{
			name:    "returns first element when palette has only one item",
			palette: []BlockState{{Name: "air"}},
			data:    []int64{1, 2, 3},
			want:    &BlockState{Name: "air"},
		},
		{
			name:         "returns first element when bitsPerEntry is invalid",
			palette:      []BlockState{{Name: "a"}, {Name: "b"}},
			data:         []int64{0},
			index:        1,
			bitsPerEntry: 0,
			want:         &BlockState{Name: "a"},
		},
		{
			name:         "returns correct state from packed data",
			palette:      []BlockState{{Name: "0"}, {Name: "1"}, {Name: "2"}, {Name: "3"}},
			data:         packData(2, 2, 3, 0, 1),
			index:        1,
			bitsPerEntry: 2,
			want:         &BlockState{Name: "3"},
		},
		{
			name:         "returns first element when index is out of bounds",
			palette:      []BlockState{{Name: "0"}, {Name: "1"}},
			data:         []int64{0},
			index:        1000,
			bitsPerEntry: 4,
			want:         &BlockState{Name: "0"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pc := &PaletteContainer{Palette: tt.palette, Data: tt.data}
			assert.Equal(t, tt.want, pc.GetState(tt.index, tt.bitsPerEntry))
		})
	}
}

func setPackedValue(data []int64, bitsPerEntry, index, value int) {
	entriesPerLong := 64 / bitsPerEntry // remainder is ignored
	longIndex := index / entriesPerLong
	bitOffset := (index % entriesPerLong) * bitsPerEntry

	// shift to correct bit position and OR the value into the long
	data[longIndex] |= int64(value) << bitOffset
}

func TestSection_GetBlock(t *testing.T) {
	t.Run("returns nil when block states are missing", func(t *testing.T) {
		s := &Section{}
		assert.Nil(t, s.GetBlock(0, 0, 0))
	})

	t.Run("returns correct block state at coordinates", func(t *testing.T) {
		data := make([]int64, 256)
		bitsPerEntry := 4

		setPackedValue(data, bitsPerEntry, 0, 0)
		setPackedValue(data, bitsPerEntry, 1, 1)
		setPackedValue(data, bitsPerEntry, 16, 2)
		setPackedValue(data, bitsPerEntry, 256, 3)

		section := &Section{
			BlockStates: &PaletteContainer{
				Palette: []BlockState{
					{Name: "air"},
					{Name: "stone"},
					{Name: "dirt"},
					{Name: "wood"},
				},
				Data: data,
			},
		}

		tests := []struct {
			x, y, z int
			want    string
		}{
			{x: 0, y: 0, z: 0, want: "air"},
			{x: 1, y: 0, z: 0, want: "stone"},
			{x: 0, y: 0, z: 1, want: "dirt"},
			{x: 0, y: 1, z: 0, want: "wood"},
			{x: 15, y: 15, z: 15, want: "air"},
		}

		for _, tt := range tests {
			assert.Equal(t, tt.want, section.GetBlock(tt.x, tt.y, tt.z).Name)
		}
	})
}

func TestSection_GetBiome(t *testing.T) {
	t.Run("returns nil when biomes are missing", func(t *testing.T) {
		s := &Section{}
		assert.Nil(t, s.GetBiome(0, 0, 0))
	})

	t.Run("returns correct biome at coordinates", func(t *testing.T) {
		data := make([]int64, 1)
		bitsPerEntry := 1

		setPackedValue(data, bitsPerEntry, 0, 0)
		setPackedValue(data, bitsPerEntry, 21, 1)

		section := &Section{
			Biomes: &PaletteContainer{
				Palette: []BlockState{
					{Name: "plains"},
					{Name: "desert"},
				},
				Data: data,
			},
		}

		tests := []struct {
			x, y, z int
			want    string
		}{
			{x: 0, y: 0, z: 0, want: "plains"},
			{x: 3, y: 3, z: 3, want: "plains"},
			{x: 4, y: 4, z: 4, want: "desert"},
			{x: 7, y: 7, z: 7, want: "desert"},
		}

		for _, tt := range tests {
			assert.Equal(t, tt.want, section.GetBiome(tt.x, tt.y, tt.z).Name)
		}
	})
}
