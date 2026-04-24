package main

import (
	"compress/gzip"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"

	"github.com/dsnidr/cartograph/internal/nbt"
	"github.com/dsnidr/cartograph/internal/pipeline"
	"github.com/dsnidr/cartograph/internal/render"
)

func setupFlags() (*runOptions, *debugConfig) {
	var (
		worldDir      string
		outDir        string
		outFile       string
		outModeStr    string
		targetVersion string
		scaleStr      string
		maxMemory     string
		workers       int
		waterDepth    int
		statsCSV      string
		quiet         bool
		silent        bool
	)

	debugCfg := registerDebugFlags()

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Cartograph - Minecraft World Mapper\n\n")
		fmt.Fprintf(os.Stderr, "Usage:\n  cartograph [options]\n\n")

		fmt.Fprintf(os.Stderr, "Core Options:\n")
		fmt.Fprintf(os.Stderr, "  --world <path>          Path to the world directory. (Optional if in world dir)\n")

		fmt.Fprintf(os.Stderr, "\nOutput Options:\n")
		fmt.Fprintf(os.Stderr, "  --out <path>            Output file name for composite mode (default: \"[out-dir]/map.png\")\n")
		fmt.Fprintf(os.Stderr, "  --out-dir <path>        Output directory for tiles mode (default: \"out\")\n")
		fmt.Fprintf(os.Stderr, "  --out-mode <mode>       'tiles' (per region) or 'composite' (single image) (default: \"composite\")\n")
		fmt.Fprintf(os.Stderr, "  --scale <value>         Downsampling: 1:1, 1:2, 1:4, 1:16, or 100%%, 50%%, 25%% (default: \"1:1\")\n")
		fmt.Fprintf(os.Stderr, "  --water-depth <value>   Maximum water depth to render, shorter depths increase render speed and generate smaller output images. -1 for infinite (default: 6)\n")

		fmt.Fprintf(os.Stderr, "\nPerformance Options:\n")
		fmt.Fprintf(os.Stderr, "  --workers <num>         Concurrent workers (default: NumCPU - 1)\n")
		fmt.Fprintf(os.Stderr, "  --max-memory <limit>    Soft memory limit, e.g., '1G', '512M'. Note that there is no guarantee it'll stay under this, it just provides an execution target to tune memory cleanup.\n")

		fmt.Fprintf(os.Stderr, "\nAdvanced Options:\n")
		fmt.Fprintf(os.Stderr, "  --target-version <ver>  Target Minecraft version, e.g., '1.18' (default: \"auto\")\n")
		fmt.Fprintf(os.Stderr, "  --stats-csv <path>      Append runtime stats to CSV file\n")

		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  cartograph --world ./myworld --out-mode composite\n")
		fmt.Fprintf(os.Stderr, "  cartograph --scale 5%% --workers 4\n")
	}

	flag.StringVar(&worldDir, "world", "", "Path to the world directory.")
	flag.StringVar(&outDir, "out-dir", "out", "Output directory for rendered PNGs.")
	flag.StringVar(&outFile, "out", "", "Output file name for composite mode.")
	flag.StringVar(&outModeStr, "out-mode", string(pipeline.OutputModeComposite), "Output mode: 'tiles' or 'composite'.")
	flag.StringVar(&targetVersion, "target-version", "auto", "Target Minecraft version (e.g., 1.18).")
	flag.StringVar(&scaleStr, "scale", "1:1", "Scale downsampling.")
	flag.StringVar(&maxMemory, "max-memory", "", "Set a soft memory limit.")
	flag.IntVar(&workers, "workers", 0, "Number of concurrent workers.")
	flag.IntVar(&waterDepth, "water-depth", 6, "Maximum water depth to render.")
	flag.StringVar(&statsCSV, "stats-csv", "", "Append runtime stats to the specified CSV file.")
	flag.BoolVar(&quiet, "quiet", false, "Quiet mode: print only warnings, errors, and the final completion summary.")
	flag.BoolVar(&silent, "silent", false, "Silent mode: print absolutely nothing except fatal panics.")
	flag.Parse()

	var verbosity VerbosityLevel
	switch {
	case silent:
		verbosity = VerbositySilent
	case quiet:
		verbosity = VerbosityQuiet
	default:
		verbosity = VerbosityNormal
	}

	// Setup global logger immediately so setup logs are correctly filtered
	logLevel := slog.LevelInfo
	switch verbosity {
	case VerbosityQuiet:
		logLevel = slog.LevelWarn
	case VerbositySilent:
		logLevel = slog.LevelError
	}

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel})))

	// World dir auto-detection
	if worldDir == "" {
		if _, err := os.Stat("level.dat"); err == nil {
			worldDir = "."
			slog.Info("Auto-detected world in current directory")
		} else if _, err := os.Stat("region"); err == nil {
			worldDir = "."
			slog.Info("Auto-detected world in current directory (region folder found)")
		} else {
			slog.Error("--world flag is required (could not auto-detect world in current directory)")
			os.Exit(1)
		}
	}

	// Version auto-detection from level dat
	if targetVersion == "auto" {
		targetVersion = detectVersion(worldDir)
		slog.Info("Detected Minecraft version", "version", targetVersion)
	}

	scale, err := parseScale(scaleStr)
	if err != nil {
		slog.Error("invalid scale", "err", err)
		os.Exit(1)
	}

	if maxMemory != "" {
		limit, err := parseMemoryLimit(maxMemory)
		if err != nil {
			slog.Error("invalid max-memory", "err", err)
			os.Exit(1)
		}

		debug.SetMemoryLimit(limit)
		slog.Info("Set soft memory limit", "limit", maxMemory, "bytes", limit)
	}

	outMode := pipeline.OutputMode(strings.ToLower(strings.TrimSpace(outModeStr)))
	if outMode != pipeline.OutputModeTiles && outMode != pipeline.OutputModeComposite {
		slog.Error("invalid out-mode", "mode", outModeStr, "expected", "'tiles' or 'composite'")
		os.Exit(1)
	}

	regionDir := filepath.Join(worldDir, "dimensions", "minecraft", "overworld", "region")
	stat, err := os.Stat(regionDir)
	if err != nil || !stat.IsDir() {
		// Fallback to legacy/root region folder
		regionDir = filepath.Join(worldDir, "region")
		stat, err = os.Stat(regionDir)
		if err != nil {
			slog.Error("invalid region dir (checked overworld and root)", "dir", worldDir, "err", err)
			os.Exit(1)
		} else if !stat.IsDir() {
			slog.Error("region dir is not a directory", "dir", regionDir)
			os.Exit(1)
		}
	}

	if err := os.MkdirAll(outDir, 0o750); err != nil {
		slog.Error("failed to create out-dir", "err", err)
		os.Exit(1)
	}

	if outFile != "" {
		if err := os.MkdirAll(filepath.Dir(outFile), 0o750); err != nil {
			slog.Error("failed to create out file directory", "err", err)
			os.Exit(1)
		}
	}

	return &runOptions{
		RegionDir:     regionDir,
		OutDir:        outDir,
		OutFile:       outFile,
		OutMode:       outMode,
		Scale:         scale,
		TargetVersion: targetVersion,
		Workers:       workers,
		StatsCSV:      statsCSV,
		Verbosity:     verbosity,
		RendererConfig: render.RendererConfig{
			DebugMissingBlocks: false, // Could be added as a flag later if needed
			MaxWaterDepth:      waterDepth,
		},
	}, debugCfg
}

func parseScale(s string) (int, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "1:1", "1", "100%", "1.0":
		return 0, nil
	case "1:2", "2", "50%", "0.5":
		return 1, nil
	case "1:4", "4", "25%", "0.25":
		return 2, nil
	case "1:16", "16", "6.25%", "thumbnail":
		return 3, nil
	default:
		return 0, fmt.Errorf("unrecognized scale format '%s' (valid: 1:1, 1:2, 1:4, 1:16, 100%%, 50%%, 25%%, thumbnail)", s)
	}
}

func parseMemoryLimit(s string) (int64, error) {
	s = strings.ToUpper(strings.TrimSpace(s))
	if s == "" {
		return 0, nil
	}

	var multiplier int64 = 1
	switch {
	case strings.HasSuffix(s, "G") || strings.HasSuffix(s, "GB"):
		multiplier = 1024 * 1024 * 1024
		s = strings.TrimRight(strings.TrimRight(s, "B"), "G")
	case strings.HasSuffix(s, "M") || strings.HasSuffix(s, "MB"):
		multiplier = 1024 * 1024
		s = strings.TrimRight(strings.TrimRight(s, "B"), "M")
	case strings.HasSuffix(s, "K") || strings.HasSuffix(s, "KB"):
		multiplier = 1024
		s = strings.TrimRight(strings.TrimRight(s, "B"), "K")
	}

	val, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse numeric value: %w", err)
	}

	return int64(val * float64(multiplier)), nil
}

func detectVersion(worldDir string) string {
	levelDatPath := filepath.Join(worldDir, "level.dat")
	f, err := os.Open(levelDatPath)
	if err != nil {
		slog.Warn("could not open level.dat for version detection, defaulting to 1.18", "err", err)
		return "1.18"
	}
	defer func() {
		_ = f.Close()
	}()

	gr, err := gzip.NewReader(f)
	if err != nil {
		slog.Warn("could not decompress level.dat, defaulting to 1.18", "err", err)
		return "1.18"
	}
	defer func() {
		_ = gr.Close()
	}()

	version, err := nbt.ReadLevelVersion(gr)
	if err != nil {
		slog.Warn("failed to parse version from level.dat, defaulting to 1.18", "err", err)
		return "1.18"
	}

	// Simplify version string. e.g., "1.18.2" becomes "1.18"
	parts := strings.Split(version, ".")
	if len(parts) >= 2 {
		return parts[0] + "." + parts[1]
	}

	return version
}
