// Package stringcolour provides deterministic colour generation from strings using hashing.
package stringcolour

import (
	"hash/fnv"
	"image/color"
)

// ToColour uses a hash of a string to deterministically return it a colour
func ToColour(str string) color.RGBA {
	h := fnv.New32a()
	h.Write([]byte(str))
	hash := h.Sum32()

	r := uint8((hash >> 16) & 0xFF)
	g := uint8((hash >> 8) & 0xFF)
	b := uint8(hash & 0xFF)

	return color.RGBA{
		R: r,
		G: g,
		B: b,
		A: 255,
	}
}
