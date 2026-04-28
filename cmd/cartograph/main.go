package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	png "github.com/dsnidr/cartograph/internal/fastpng"

	"github.com/dsnidr/cartograph/internal/nbt"
	"github.com/dsnidr/cartograph/internal/pipeline"
	"github.com/dsnidr/cartograph/internal/registry"
	"github.com/dsnidr/cartograph/internal/render"
	"github.com/dsnidr/cartograph/internal/render/csd"
)

type VerbosityLevel int

const (
	VerbosityNormal VerbosityLevel = iota
	VerbosityQuiet
	VerbositySilent
)

type runOptions struct {
	RegionDir      string
	OutDir         string
	OutFile        string
	OutMode        pipeline.OutputMode
	Scale          int
	TargetVersion  string
	Workers        int
	RendererConfig render.RendererConfig
	DebugMemory    bool
	StatsCSV       string
	EmitSpatial    bool
	Verbosity      VerbosityLevel
}

func main() {
	if len(os.Args) == 2 && (os.Args[1] == "version" || os.Args[1] == "--version" || os.Args[1] == "-v") {
		printVersion()
		os.Exit(0)
	}

	opts, debugCfg := setupFlags()

	waitProfile := applyDebug(debugCfg, opts)
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)

	if err := run(ctx, *opts); err != nil {
		slog.Error("cartographer failed", "err", err)
		cancel()
		os.Exit(1)
	}

	waitProfile(ctx)
	cancel()
}

func run(ctx context.Context, opts runOptions) error {
	slog.Info("Starting Cartographer...",
		"region_dir", opts.RegionDir,
		"output_dir", opts.OutDir,
		"target_version", opts.TargetVersion,
		"scale", opts.Scale)

	var (
		statsMu          sync.Mutex
		minCPS           float64 = -1
		maxCPS           float64
		peakMem          uint64
		currentProcessed atomic.Int64
	)

	statsDone := make(chan struct{})
	go func() {
		ticker := time.NewTicker(250 * time.Millisecond)
		defer ticker.Stop()
		var m runtime.MemStats
		var lastProcessed int64
		ticks := 0
		for {
			select {
			case <-ctx.Done():
				return
			case <-statsDone:
				return
			case <-ticker.C:
				ticks++
				curr := currentProcessed.Load()
				delta := curr - lastProcessed
				lastProcessed = curr
				cps := float64(delta*1024) * 4.0

				statsMu.Lock()
				if curr > 0 {
					if minCPS < 0 || cps < minCPS {
						minCPS = cps
					}
					if cps > maxCPS {
						maxCPS = cps
					}
				}

				if opts.DebugMemory {
					runtime.ReadMemStats(&m)
					if m.Sys > peakMem {
						peakMem = m.Sys
					}
				}
				statsMu.Unlock()

				if opts.DebugMemory && ticks%4 == 0 {
					slog.Debug("memory stats",
						slog.String("alloc", fmt.Sprintf("%v MB", m.Alloc/1024/1024)),
						slog.String("sys", fmt.Sprintf("%v MB", m.Sys/1024/1024)),
						slog.Int("num_gc", int(m.NumGC)),
					)
				}
			}
		}
	}()

	// Initialize the block registry (automatically loads the embedded vanilla colours)
	blockReg, err := registry.New(ctx)
	if err != nil {
		return fmt.Errorf("failed to create block registry: %w", err)
	}

	jobs, totalFiles, bounds, err := pipeline.DiscoverRegions(ctx, opts.RegionDir)
	if err != nil {
		return fmt.Errorf("discovery failed: %w", err)
	}

	if totalFiles == 0 {
		return errors.New("no .mca files found in region dir")
	}

	slog.Info("Discovered region files", "count", totalFiles)
	slog.Info("Global Map Bounds", "minX", bounds.MinX, "maxX", bounds.MaxX, "minZ", bounds.MinZ, "maxZ", bounds.MaxZ)
	slog.Info("Beginning processing...")

	orchestrator := pipeline.NewOrchestrator(
		nbt.NewStringPool(),
		blockReg,
		opts.OutDir,
		opts.Scale,
		opts.OutMode,
		render.NewRenderer(opts.RendererConfig),
	)

	numWorkers := opts.Workers
	if numWorkers <= 0 {
		numWorkers = max(runtime.NumCPU()-1, 1)
	}

	slog.Info("Starting worker pool", "workers", numWorkers)
	results := orchestrator.StartWorkers(ctx, numWorkers, jobs)

	var canvas *render.FileBackedImage
	var heightmap *render.FileBackedHeightmap
	var biomemap *render.FileBackedBiomeMap
	var canvasErr error
	var finalCanvas *render.FileBackedImage

	var step int
	switch opts.Scale {
	case 1:
		step = 2
	case 2:
		step = 4
	case 3:
		step = 16
	default:
		step = 1
	}

	regionPixelSize := render.RegionBlocks / step

	if opts.OutMode == pipeline.OutputModeComposite {
		width := (bounds.MaxX - bounds.MinX + 1) * regionPixelSize
		height := (bounds.MaxZ - bounds.MinZ + 1) * regionPixelSize

		slog.Info("Allocating composite canvases on disk...", "width", width, "height", height)

		tempColorFile := filepath.Join(opts.OutDir, "base_colors.tmp")
		canvas, canvasErr = render.NewFileBackedImage(width, height, tempColorFile)
		if canvasErr != nil {
			return fmt.Errorf("create composite base color canvas: %w", canvasErr)
		}
		defer func() {
			_ = canvas.Close()
		}()

		tempHeightFile := filepath.Join(opts.OutDir, "heightmap.tmp")
		heightmap, canvasErr = render.NewFileBackedHeightmap(width, height, tempHeightFile)
		if canvasErr != nil {
			return fmt.Errorf("create composite heightmap: %w", canvasErr)
		}
		defer func() {
			_ = heightmap.Close()
		}()

		if opts.EmitSpatial {
			tempBiomeFile := filepath.Join(opts.OutDir, "biomemap.tmp")
			biomemap, canvasErr = render.NewFileBackedBiomeMap(width, height, tempBiomeFile)
			if canvasErr != nil {
				return fmt.Errorf("create composite biome map: %w", canvasErr)
			}
			defer func() {
				_ = biomemap.Close()
			}()
		}

		tempFinalFile := filepath.Join(opts.OutDir, "final_map.tmp")
		finalCanvas, canvasErr = render.NewFileBackedImage(width, height, tempFinalFile)
		if canvasErr != nil {
			return fmt.Errorf("create final composite canvas: %w", canvasErr)
		}
		defer func() {
			_ = finalCanvas.Close()
		}()
	}

	var pass2Duration time.Duration
	var pngDuration time.Duration

	processed := 0
	startTime := time.Now()
	for res := range results {
		processed++
		currentProcessed.Store(int64(processed))

		if res.Err != nil {
			slog.Error("Error processing region", "region", res.RegionPath, "err", res.Err)
		} else if opts.OutMode == pipeline.OutputModeComposite {
			offsetX := (res.X - bounds.MinX) * regionPixelSize
			offsetZ := (res.Z - bounds.MinZ) * regionPixelSize

			if err := canvas.WriteSubImage(offsetX, offsetZ, res.Image); err != nil {
				slog.Error("Failed to write to base color canvas", "region", res.RegionPath, "err", err)
			}
			if err := heightmap.WriteSubHeightmap(offsetX, offsetZ, res.Heightmap); err != nil {
				slog.Error("Failed to write to heightmap", "region", res.RegionPath, "err", err)
			}
			if opts.EmitSpatial && biomemap != nil {
				if err := biomemap.WriteSubBiomeMap(offsetX, offsetZ, res.Biomemap); err != nil {
					slog.Error("Failed to write to biome map", "region", res.RegionPath, "err", err)
				}
			}
		}

		percentage := (float64(processed) / float64(totalFiles)) * 100

		elapsedSecs := time.Since(startTime).Seconds()
		chunksProcessed := processed * 1024
		var chunksPerSec float64
		if elapsedSecs > 0 {
			chunksPerSec = float64(chunksProcessed) / elapsedSecs
		}

		slog.Info("Processed region (Pass 1)",
			"current", processed,
			"total", totalFiles,
			"progress", fmt.Sprintf("%.1f%%", percentage),
			"file", filepath.Base(res.RegionPath),
			"chunks_per_sec", fmt.Sprintf("%.0f", chunksPerSec))
	}

	pass1Duration := time.Since(startTime)
	totalElapsedSecs := pass1Duration.Seconds()
	totalChunks := totalFiles * 1024
	var avgChunksPerSec float64
	if totalElapsedSecs > 0 {
		avgChunksPerSec = float64(totalChunks) / totalElapsedSecs
	}

	if opts.OutMode == pipeline.OutputModeComposite && canvas != nil && heightmap != nil {
		pass2Start := time.Now()
		slog.Info("Starting Pass 2: Global Shading & Shadows...")

		// Build tile jobs
		width := (bounds.MaxX - bounds.MinX + 1) * regionPixelSize
		height := (bounds.MaxZ - bounds.MinZ + 1) * regionPixelSize

		tileSize := 512

		var tileJobs []pipeline.ShadingJob
		for z := 0; z < height; z += tileSize {
			for x := 0; x < width; x += tileSize {
				w := tileSize
				if x+w > width {
					w = width - x
				}
				h := tileSize
				if z+h > height {
					h = height - z
				}
				tileJobs = append(tileJobs, pipeline.ShadingJob{
					X:      x,
					Z:      z,
					Width:  w,
					Height: h,
				})
			}
		}

		jobChan := make(chan pipeline.ShadingJob, len(tileJobs))
		for _, tj := range tileJobs {
			jobChan <- tj
		}
		close(jobChan)

		shadingResults := orchestrator.StartShadingWorkers(ctx, numWorkers, jobChan, canvas, heightmap)

		tilesProcessed := 0
		for res := range shadingResults {
			tilesProcessed++
			if res.Err != nil {
				slog.Error("Error shading tile", "x", res.X, "z", res.Z, "err", res.Err)
				continue
			}

			if err := finalCanvas.WriteSubImage(res.X, res.Z, res.Image); err != nil {
				slog.Error("Failed to write to final canvas", "x", res.X, "z", res.Z, "err", err)
			}

			if opts.Verbosity == VerbosityNormal {
				if tilesProcessed%10 == 0 || tilesProcessed == len(tileJobs) {
					slog.Info("Processed tile (Pass 2)",
						"current", tilesProcessed,
						"total", len(tileJobs),
						"progress", fmt.Sprintf("%.1f%%", float64(tilesProcessed)/float64(len(tileJobs))*100))
				}
			}
		}
		pass2Duration = time.Since(pass2Start)

		slog.Info("Encoding final composite PNG...")
		pngStart := time.Now()
		var mapFile string
		if opts.OutFile != "" {
			mapFile = opts.OutFile
		} else {
			mapFile = filepath.Join(opts.OutDir, "map.png")
		}

		f, err := os.Create(mapFile)
		if err != nil {
			return fmt.Errorf("create final map png: %w", err)
		}
		defer func() {
			_ = f.Close()
		}()

		encoder := png.Encoder{CompressionLevel: png.BestSpeed}
		if err := encoder.Encode(f, finalCanvas); err != nil {
			return fmt.Errorf("encode final map png: %w", err)
		}
		pngDuration = time.Since(pngStart)

		if opts.EmitSpatial && biomemap != nil {
			slog.Info("Emitting spatial data dump (.csd)...")
			csdStart := time.Now()
			csdPath := ""
			if opts.OutFile != "" {
				csdPath = strings.TrimSuffix(opts.OutFile, filepath.Ext(opts.OutFile)) + ".csd"
			} else {
				csdPath = filepath.Join(opts.OutDir, "map.csd")
			}

			cf, err := os.Create(csdPath)
			if err != nil {
				return fmt.Errorf("create csd file: %w", err)
			}

			if err := csd.Write(cf, width, height, opts.Scale, orchestrator.BiomePalette, heightmap, biomemap); err != nil {
				_ = cf.Close()
				return fmt.Errorf("write csd file: %w", err)
			}

			if err := cf.Close(); err != nil {
				return fmt.Errorf("close csd file: %w", err)
			}
			slog.Info("Spatial data dump complete", "path", csdPath, "took", time.Since(csdStart).Round(time.Millisecond))
		}
	}

	if opts.Verbosity != VerbositySilent {
		logger := slog.Default()
		if opts.Verbosity == VerbosityQuiet {
			// In quiet mode, default logger is set to Warn, so we need a one-off logger to print this Info message
			logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
		}

		logger.Info("Done!",
			"took", time.Since(startTime).Round(time.Millisecond),
			"pass_1_took", pass1Duration.Round(time.Millisecond),
			"pass_2_took", pass2Duration.Round(time.Millisecond),
			"encoding_took", pngDuration.Round(time.Millisecond),
			"avg_chunks_per_sec", fmt.Sprintf("%.0f", avgChunksPerSec),
		)
	}

	close(statsDone)

	if opts.DebugMemory {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		statsMu.Lock()
		if m.Sys > peakMem {
			peakMem = m.Sys
		}
		statsMu.Unlock()
	}

	if opts.StatsCSV != "" {
		fileExisted := true
		if _, err := os.Stat(opts.StatsCSV); os.IsNotExist(err) {
			fileExisted = false
		}
		f, err := os.OpenFile(opts.StatsCSV, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		if err == nil {
			if !fileExisted {
				_, _ = f.WriteString("workers,execution_time_s,avg_cps\n")
			}

			actualWorkers := opts.Workers
			if actualWorkers <= 0 {
				actualWorkers = max(runtime.NumCPU()-1, 1)
			}

			_, _ = fmt.Fprintf(f, "%d,%.3f,%.0f\n", actualWorkers, totalElapsedSecs, avgChunksPerSec)
			_ = f.Close()
		} else {
			slog.Error("Failed to write stats csv", "err", err)
		}
	}

	return nil
}
