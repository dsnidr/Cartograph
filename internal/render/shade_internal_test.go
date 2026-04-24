package render

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClampUint8(t *testing.T) {
	tests := []struct {
		name     string
		input    float64
		expected uint8
	}{
		{
			name:     "returns exact value when within range",
			input:    128.0,
			expected: 128,
		},
		{
			name:     "clamps to maximum when above 255",
			input:    300.0,
			expected: 255,
		},
		{
			name:     "clamps to minimum when below 0",
			input:    -50.0,
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, clampUint8(tt.input))
		})
	}
}
