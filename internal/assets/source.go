package assets

import (
	"context"
	"io"
)

// Source provides an abstraction for reading Minecraft asset files from various locations.
type Source interface {
	// Open returns a read stream for the requested file path. It returns an os.ErrNotExist error
	// if the file is not found.
	Open(ctx context.Context, path string) (io.ReadCloser, error)

	// ListBlocks returns a list of block IDs (e.g., "minecraft:stone") found in the given namespace.
	ListBlocks(ctx context.Context, namespace string) ([]string, error)

	// ListBiomes returns a list of biome IDs (e.g., "minecraft:plains") found in the given namespace.
	ListBiomes(ctx context.Context, namespace string) ([]string, error)

	// Close cleans up any open file handles or readers.
	Close() error
}
