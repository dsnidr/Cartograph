package assets_test

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/dsnidr/cartograph/internal/assets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDirectorySource(t *testing.T) {
	tmpDir := t.TempDir()

	base := filepath.Join(tmpDir, "assets/minecraft/blockstates")
	require.NoError(t, os.MkdirAll(base, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(base, "stone.json"), []byte("{}"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(base, "dirt.json"), []byte("{}"), 0644))

	source := assets.NewDirectorySource(tmpDir)
	defer source.Close()

	t.Run("returns list of blocks in namespace", func(t *testing.T) {
		ctx := t.Context()
		blocks, err := source.ListBlocks(ctx, "minecraft")
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"minecraft:stone", "minecraft:dirt"}, blocks)
	})

	t.Run("returns empty list for non-existent namespace", func(t *testing.T) {
		ctx := t.Context()
		blocks, err := source.ListBlocks(ctx, "nonexistent")
		require.NoError(t, err)
		assert.Empty(t, blocks)
	})

	t.Run("opens file successfully", func(t *testing.T) {
		ctx := t.Context()
		rc, err := source.Open(ctx, "assets/minecraft/blockstates/stone.json")
		require.NoError(t, err)
		defer rc.Close()

		content, err := io.ReadAll(rc)
		require.NoError(t, err)
		assert.Equal(t, "{}", string(content))
	})

	t.Run("fails when file does not exist", func(t *testing.T) {
		ctx := t.Context()
		_, err := source.Open(ctx, "nonexistent.json")
		assert.ErrorIs(t, err, os.ErrNotExist)
	})
}

func TestZipSource(t *testing.T) {
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "test.zip")

	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)

	files := map[string]string{
		"assets/minecraft/blockstates/stone.json": "{}",
		"assets/minecraft/blockstates/dirt.json":  "{}",
		"other/file.txt":                          "hello",
	}

	for name, content := range files {
		f, err := zw.Create(name)
		require.NoError(t, err)
		_, err = f.Write([]byte(content))
		require.NoError(t, err)
	}
	require.NoError(t, zw.Close())
	require.NoError(t, os.WriteFile(zipPath, buf.Bytes(), 0644))

	source, err := assets.NewZipSource(zipPath)
	require.NoError(t, err)
	defer source.Close()

	t.Run("returns list of blocks in namespace", func(t *testing.T) {
		ctx := t.Context()
		blocks, err := source.ListBlocks(ctx, "minecraft")
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"minecraft:stone", "minecraft:dirt"}, blocks)
	})

	t.Run("returns empty list for non-existent namespace", func(t *testing.T) {
		ctx := t.Context()
		blocks, err := source.ListBlocks(ctx, "nonexistent")
		require.NoError(t, err)
		assert.Empty(t, blocks)
	})

	t.Run("opens file successfully", func(t *testing.T) {
		ctx := t.Context()
		rc, err := source.Open(ctx, "assets/minecraft/blockstates/stone.json")
		require.NoError(t, err)
		defer rc.Close()

		content, err := io.ReadAll(rc)
		require.NoError(t, err)
		assert.Equal(t, "{}", string(content))
	})

	t.Run("fails when file does not exist", func(t *testing.T) {
		ctx := t.Context()
		_, err := source.Open(ctx, "nonexistent.json")
		assert.ErrorIs(t, err, os.ErrNotExist)
	})
}
