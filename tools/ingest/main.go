// Package main provides a tool to ingest Minecraft jar files and extract block and biome colours.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"

	"github.com/dsnidr/cartograph/internal/assets"
)

func main() {
	jarPath := flag.String("jar", "", "Path to the Minecraft version .jar file. Use the latest version.")
	outDir := flag.String("outDir", ".", "Directory to save the generated JSONL files (defaults to current directory)")
	namespace := flag.String("namespace", "minecraft", "Namespace to extract blocks/biomes from")
	ingestType := flag.String("type", "all", "Type of data to ingest: 'blocks', 'biomes', or 'all'")
	debug := flag.Bool("debug", false, "Enable debug logging")
	flag.Parse()

	level := slog.LevelInfo
	if *debug {
		level = slog.LevelDebug
	}

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))

	if err := run(*jarPath, *outDir, *namespace, *ingestType); err != nil {
		slog.Error("asset ingest failed", "error", err)
		os.Exit(1)
	}
}

func run(jarPath, outDir, namespace, ingestType string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if jarPath == "" {
		return errors.New("-jar path must be provided")
	}

	slog.Info("Opening jar file", "path", jarPath)
	source, err := assets.NewZipSource(jarPath)
	if err != nil {
		return fmt.Errorf("failed to open jar: %w", err)
	}
	defer func() {
		_ = source.Close()
	}()

	resolver := assets.NewResolver(source)

	if ingestType == "blocks" || ingestType == "all" {
		path := filepath.Join(outDir, "vanilla_colours.jsonl")
		if err := runBlocks(ctx, source, resolver, path, namespace); err != nil {
			return err
		}
	}

	if ingestType == "biomes" || ingestType == "all" {
		path := filepath.Join(outDir, "vanilla_biomes.jsonl")
		if err := runBiomes(ctx, source, resolver, path, namespace); err != nil {
			return err
		}
	}

	return nil
}

func runBlocks(ctx context.Context, source assets.Source, resolver *assets.Resolver, outPath, namespace string) error {
	slog.Info("Listing blocks", "namespace", namespace)
	blocks, err := source.ListBlocks(ctx, namespace)
	if err != nil {
		return fmt.Errorf("failed to list blocks: %w", err)
	}

	if len(blocks) == 0 {
		return fmt.Errorf("no blocks found in namespace '%s'", namespace)
	}

	sort.Strings(blocks)
	slog.Info("Found blocks, resolving colours...", "count", len(blocks))

	outFile, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer func() {
		_ = outFile.Close()
	}()

	encoder := json.NewEncoder(outFile)
	if err := encoder.Encode(map[string]string{"$namespace": namespace}); err != nil {
		return fmt.Errorf("failed to encode namespace header: %w", err)
	}

	successCount := 0
	failCount := 0

	for _, blockID := range blocks {
		colour, transparent, err := resolver.Resolve(ctx, blockID)
		if err != nil {
			var assetErr *assets.Error
			if errors.As(err, &assetErr) {
				if assetErr.Phase == assets.PhaseLoadBlockState || assetErr.Phase == assets.PhaseLoadModel || errors.Is(assetErr.Err, assets.ErrNoTexture) || errors.Is(assetErr.Err, os.ErrNotExist) {
					outID := blockID
					if len(outID) > len(namespace)+1 && outID[:len(namespace)] == namespace && outID[len(namespace)] == ':' {
						outID = outID[len(namespace)+1:]
					}

					entry := map[string][5]uint8{
						outID: {0, 0, 0, 0, 1},
					}
					if err := encoder.Encode(entry); err == nil {
						successCount++
						continue
					}
				}
				slog.Debug("Skipping block", "block", assetErr.BlockID, "cause", assetErr.Err)
			} else {
				slog.Debug("Skipping block", "block", blockID, "reason", err)
			}
			failCount++
			continue
		}

		tFlag := uint8(0)
		if transparent {
			tFlag = 1
		}

		outID := blockID
		if len(outID) > len(namespace)+1 && outID[:len(namespace)] == namespace && outID[len(namespace)] == ':' {
			outID = outID[len(namespace)+1:]
		}

		entry := map[string][5]uint8{
			outID: {colour.R, colour.G, colour.B, colour.A, tFlag},
		}

		if err := encoder.Encode(entry); err != nil {
			slog.Error("Failed to encode block", "block", blockID, "error", err)
		} else {
			successCount++
		}
	}

	slog.Info("Block extraction complete", "success", successCount, "failed", failCount, "output", outPath)
	return nil
}

func runBiomes(ctx context.Context, source assets.Source, resolver *assets.Resolver, outPath, namespace string) error {
	slog.Info("Listing biomes", "namespace", namespace)
	biomes, err := source.ListBiomes(ctx, namespace)
	if err != nil {
		return fmt.Errorf("failed to list biomes: %w", err)
	}

	if len(biomes) == 0 {
		return fmt.Errorf("no biomes found in namespace '%s'", namespace)
	}

	sort.Strings(biomes)
	slog.Info("Found biomes, resolving colours...", "count", len(biomes))

	outFile, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer func() {
		_ = outFile.Close()
	}()

	encoder := json.NewEncoder(outFile)
	if err := encoder.Encode(map[string]string{"$namespace": namespace}); err != nil {
		return fmt.Errorf("failed to encode namespace header: %w", err)
	}

	successCount := 0
	failCount := 0

	for _, biomeID := range biomes {
		data, err := resolver.ResolveBiome(ctx, biomeID)
		if err != nil {
			slog.Warn("Skipping biome", "biome", biomeID, "error", err)
			failCount++
			continue
		}

		outID := biomeID
		if len(outID) > len(namespace)+1 && outID[:len(namespace)] == namespace && outID[len(namespace)] == ':' {
			outID = outID[len(namespace)+1:]
		}

		entry := map[string]map[string][3]uint8{
			outID: {
				"grass":   {data.Grass.R, data.Grass.G, data.Grass.B},
				"foliage": {data.Foliage.R, data.Foliage.G, data.Foliage.B},
				"water":   {data.Water.R, data.Water.G, data.Water.B},
			},
		}

		if err := encoder.Encode(entry); err != nil {
			slog.Error("Failed to encode biome", "biome", biomeID, "error", err)
			failCount++
		} else {
			successCount++
		}
	}

	slog.Info("Biome extraction complete", "success", successCount, "failed", failCount, "output", outPath)
	return nil
}
