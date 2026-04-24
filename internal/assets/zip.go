package assets

import (
	"archive/zip"
	"context"
	"io"
	"os"
	"path"
	"strings"
)

// ZipSource implements the Source interface by reading assets from a ZIP archive (e.g., a jar file).
type ZipSource struct {
	r     *zip.ReadCloser
	files map[string]*zip.File
}

// NewZipSource creates a new ZipSource from the specified ZIP file path.
func NewZipSource(path string) (*ZipSource, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}

	files := make(map[string]*zip.File, len(r.File))
	for _, f := range r.File {
		files[f.Name] = f
	}

	return &ZipSource{
		r:     r,
		files: files,
	}, nil
}

// Open returns a read stream for the requested file path from the ZIP archive.
func (s *ZipSource) Open(_ context.Context, path string) (io.ReadCloser, error) {
	f, ok := s.files[path]
	if !ok {
		return nil, os.ErrNotExist
	}

	return f.Open()
}

// ListBlocks returns a list of block IDs found in the given namespace within the ZIP.
func (s *ZipSource) ListBlocks(_ context.Context, namespace string) ([]string, error) {
	prefix := path.Join("assets", namespace, "blockstates") + "/"
	var blocks []string

	for name := range s.files {
		if strings.HasPrefix(name, prefix) && strings.HasSuffix(name, ".json") {
			relPath := name[len(prefix):]
			blockName := strings.TrimSuffix(relPath, ".json")
			blocks = append(blocks, namespace+":"+blockName)
		}
	}

	return blocks, nil
}

// ListBiomes returns a list of biome IDs found in the given namespace within the ZIP.
func (s *ZipSource) ListBiomes(_ context.Context, namespace string) ([]string, error) {
	prefix := path.Join("data", namespace, "worldgen", "biome") + "/"
	var biomes []string

	for name := range s.files {
		if strings.HasPrefix(name, prefix) && strings.HasSuffix(name, ".json") {
			relPath := name[len(prefix):]
			biomeName := strings.TrimSuffix(relPath, ".json")
			biomes = append(biomes, namespace+":"+biomeName)
		}
	}

	return biomes, nil
}

// Close closes the underlying ZIP reader.
func (s *ZipSource) Close() error {
	return s.r.Close()
}
