package region

import "github.com/dsnidr/cartograph/internal/nbt"

// Region represents a full 32x32 chunk region parsed from a single .mca file
type Region struct {
	X      int
	Z      int
	Chunks []*nbt.RegionChunk
}
