package assets

import (
	"context"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// DirectorySource implements the Source interface by reading assets from a local directory.
type DirectorySource struct {
	basePath string
}

// NewDirectorySource creates a new DirectorySource with the given base path.
func NewDirectorySource(basePath string) *DirectorySource {
	return &DirectorySource{
		basePath: basePath,
	}
}

// Open returns a read stream for the requested file path from the directory.
func (s *DirectorySource) Open(_ context.Context, path string) (io.ReadCloser, error) {
	targetPath := filepath.Join(s.basePath, filepath.FromSlash(path))
	return os.Open(targetPath)
}

// ListBlocks returns a list of block IDs found in the given namespace directory.
func (s *DirectorySource) ListBlocks(_ context.Context, namespace string) ([]string, error) {
	blockstatesDir := filepath.Join(s.basePath, "assets", namespace, "blockstates")
	var blocks []string

	err := filepath.WalkDir(blockstatesDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".json") {
			relPath, err := filepath.Rel(blockstatesDir, path)
			if err == nil {
				// Normalize slashes for ResourceLocation path
				relPath = filepath.ToSlash(relPath)
				blockName := strings.TrimSuffix(relPath, ".json")
				blocks = append(blocks, namespace+":"+blockName)
			}
		}
		return nil
	})

	if err != nil {
		if os.IsNotExist(err) {
			// No blocks found for this namespace
			return nil, nil
		}
		return nil, err
	}

	return blocks, nil
}

// ListBiomes returns a list of biome IDs found in the given namespace directory.
func (s *DirectorySource) ListBiomes(_ context.Context, namespace string) ([]string, error) {
	biomesDir := filepath.Join(s.basePath, "data", namespace, "worldgen", "biome")
	var biomes []string

	err := filepath.WalkDir(biomesDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".json") {
			relPath, err := filepath.Rel(biomesDir, path)
			if err == nil {
				// Normalize slashes for ResourceLocation path
				relPath = filepath.ToSlash(relPath)
				biomeName := strings.TrimSuffix(relPath, ".json")
				biomes = append(biomes, namespace+":"+biomeName)
			}
		}
		return nil
	})

	if err != nil {
		if os.IsNotExist(err) {
			// No biomes found for this namespace
			return nil, nil
		}
		return nil, err
	}

	return biomes, nil
}

// Close is a no-op for DirectorySource.
func (s *DirectorySource) Close() error {
	return nil
}
