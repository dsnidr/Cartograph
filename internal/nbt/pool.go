package nbt

import (
	"sync"
	"unsafe"
)

// StringPool is a thread-safe intern pool for strings. It is used to dedupe highly repetitive
// strings (such as block/biome names) to prevent redundant memory allocations during NBT parsing.
//
// This might seem minor, but the savings very much do add up over time when parsing large worlds.
type StringPool struct {
	m sync.Map
}

// NewStringPool creates a new, empty StringPool.
func NewStringPool() *StringPool {
	return &StringPool{}
}

// Intern returns a deduplicated version of the provided string. If the string already exists in the
// pool, it returns the existing pointer. Otherwise, it adds it to the pool and returns the newly
// added string value.
func (sp *StringPool) Intern(s string) string {
	if actual, loaded := sp.m.LoadOrStore(s, s); loaded {
		return actual.(string)
	}
	return s
}

// InternBytes looks up a string by a byte slice without allocating a string for the lookup.
// If the string does not exist, it allocates a fresh string from the bytes and stores it.
func (sp *StringPool) InternBytes(b []byte) string {
	if len(b) == 0 {
		return ""
	}

	// #nosec G103 // intentional use of unsafe to avoid allocations
	key := unsafe.String(unsafe.SliceData(b), len(b))

	if actual, ok := sp.m.Load(key); ok {
		return actual.(string)
	}

	safeStr := string(b)
	if actual, loaded := sp.m.LoadOrStore(safeStr, safeStr); loaded {
		return actual.(string)
	}

	return safeStr
}
