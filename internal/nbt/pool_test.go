package nbt

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStringPool_Intern(t *testing.T) {
	t.Run("returns same string value", func(t *testing.T) {
		pool := NewStringPool()
		got := pool.Intern("minecraft:stone")
		assert.Equal(t, "minecraft:stone", got)
	})

	t.Run("caches unique strings", func(t *testing.T) {
		pool := NewStringPool()

		// Dynamically create strings so they aren't treated as constants at compile time.
		pool.Intern(string([]byte("water")))
		pool.Intern(string([]byte("water")))
		pool.Intern(string([]byte("water")))

		var count int
		pool.m.Range(func(_, _ any) bool {
			count++
			return true
		})
		assert.Equal(t, 1, count, "expected pool to only contain 1 entry for duplicate inserts")

		pool.Intern(string([]byte("stone")))

		count = 0
		pool.m.Range(func(_, _ any) bool {
			count++
			return true
		})
		assert.Equal(t, 2, count, "expected pool to contain 2 entries for distinct inserts")
	})

	t.Run("is thread safe under concurrent access", func(t *testing.T) {
		pool := NewStringPool()
		var wg sync.WaitGroup

		numWorkers := 10
		numIterations := 100

		for range numWorkers {
			wg.Go(func() {
				for range numIterations {
					for keyID := range 5 {
						val := fmt.Sprintf("key-%d", keyID)
						pool.Intern(val)
					}
				}
			})
		}

		wg.Wait()

		// we should have exactly 5 entries unless we had any upstream panics/corruption.
		var count int
		pool.m.Range(func(_, _ any) bool {
			count++
			return true
		})
		assert.Equal(t, 5, count, "expected pool to contain exactly 5 unique keys")
	})
}
