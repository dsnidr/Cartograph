package render

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBiomePalette(t *testing.T) {
	t.Run("should assign consecutive ids and return names", func(t *testing.T) {
		p := NewBiomePalette()
		
		id1 := p.GetID("minecraft:plains")
		id2 := p.GetID("minecraft:desert")
		id3 := p.GetID("minecraft:plains") // Existing

		assert.Equal(t, uint16(1), id1)
		assert.Equal(t, uint16(2), id2)
		assert.Equal(t, uint16(1), id3) // Should return same ID

		names := p.GetNames()
		assert.Equal(t, []string{"", "minecraft:plains", "minecraft:desert"}, names)
	})
}
