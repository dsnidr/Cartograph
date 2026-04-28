package stringcolour

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestToColour(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected color.RGBA
	}{
		{
			name:  "returns deterministic colour for stone",
			input: "minecraft:stone",
			// Actual FNV-1a hash of "minecraft:stone" to color.RGBA{R: 186, G: 216, B: 171, A: 255}
			expected: color.RGBA{R: 186, G: 216, B: 171, A: 255},
		},
		{
			name:     "returns deterministic colour for grass",
			input:    "minecraft:grass_block",
			expected: color.RGBA{R: 207, G: 255, B: 86, A: 255},
		},
		{
			name:     "returns different colour for different string",
			input:    "minecraft:dirt",
			expected: color.RGBA{R: 209, G: 107, B: 235, A: 255},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ToColour(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
