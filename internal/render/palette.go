package render

import (
	"sync"
)

// BiomePalette manages a thread-safe mapping between Minecraft biome names and numeric IDs.
type BiomePalette struct {
	mu     sync.RWMutex
	names  []string
	lookup map[string]uint16
}

// NewBiomePalette creates a new BiomePalette, reserving index 0 for missing/void data.
func NewBiomePalette() *BiomePalette {
	return &BiomePalette{
		names:  []string{""}, // Reserve index 0 for void/empty
		lookup: map[string]uint16{"": 0},
	}
}

// GetID returns the numeric ID for a biome name, creating one if it doesn't exist.
func (p *BiomePalette) GetID(name string) uint16 {
	p.mu.RLock()
	id, ok := p.lookup[name]
	p.mu.RUnlock()
	if ok {
		return id
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// Re-check after acquiring write lock
	if id, ok := p.lookup[name]; ok {
		return id
	}

	id = uint16(len(p.names))
	p.names = append(p.names, name)
	p.lookup[name] = id
	return id
}

// GetNames returns a copy of the biome names in the palette, indexed by their ID.
func (p *BiomePalette) GetNames() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	names := make([]string, len(p.names))
	copy(names, p.names)
	return names
}
