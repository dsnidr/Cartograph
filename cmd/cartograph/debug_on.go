//go:build debug

package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	_ "net/http/pprof"
	"os"
	"runtime"
	"runtime/pprof"
	"strings"
)

type debugConfig struct {
	opts       string
	pprofAddr  string
	cpuProfile string
	memProfile string
}

func hasDebugFlags() bool {
	return true
}

func registerDebugFlags() *debugConfig {
	cfg := &debugConfig{}
	flag.StringVar(&cfg.opts, "debug", "", "Comma-separated debug options (e.g., 'missing-blocks,memory')")
	flag.StringVar(&cfg.pprofAddr, "pprof-addr", "", "Address for live pprof server (e.g. 'localhost:6060'). Keeps app open after completion.")
	flag.StringVar(&cfg.cpuProfile, "cpuprofile", "", "Write CPU profile to the given file")
	flag.StringVar(&cfg.memProfile, "memprofile", "", "Write memory profile to the given file upon completion")
	return cfg
}

func applyDebug(cfg *debugConfig, opts *runOptions) func(context.Context) {
	if cfg.opts != "" {
		for opt := range strings.SplitSeq(cfg.opts, ",") {
			switch strings.TrimSpace(opt) {
			case "missing-blocks":
				opts.RendererConfig.DebugMissingBlocks = true
			case "memory":
				opts.DebugMemory = true
			case "log":
				// Override the logger setup by config.go to be highly verbose
				logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
				slog.SetDefault(logger)
			}
		}
	}

	var cpuF *os.File
	if cfg.cpuProfile != "" {
		f, err := os.Create(cfg.cpuProfile)
		if err != nil {
			slog.Error("could not create CPU profile", "err", err)
			os.Exit(1)
		}
		if err := pprof.StartCPUProfile(f); err != nil {
			slog.Error("could not start CPU profile", "err", err)
			os.Exit(1)
		}
		cpuF = f
		if opts.Verbosity == VerbosityNormal {
			slog.Info("Profiling enabled", "cpuprofile", cfg.cpuProfile)
		}
	}

	if cfg.pprofAddr != "" {
		go func() {
			slog.Info("Starting live pprof", "url", "http://"+cfg.pprofAddr+"/debug/pprof/")
			if err := http.ListenAndServe(cfg.pprofAddr, nil); err != nil {
				slog.Error("pprof server error", "err", err)
			}
		}()
	}

	return func(ctx context.Context) {
		if cfg.pprofAddr != "" {
			slog.Info("Processing complete. Live pprof server remains active.", "url", "http://"+cfg.pprofAddr+"/debug/pprof/")
			slog.Info("Press Ctrl+C to exit.")
			<-ctx.Done()
		}

		if cpuF != nil {
			pprof.StopCPUProfile()
			cpuF.Close()
			slog.Info("CPU profile written", "file", cfg.cpuProfile)
		}

		if cfg.memProfile != "" {
			f, err := os.Create(cfg.memProfile)
			if err != nil {
				slog.Error("could not create memory profile", "err", err)
				os.Exit(1)
			}
			defer f.Close()

			runtime.GC() // get up-to-date statistics
			if err := pprof.WriteHeapProfile(f); err != nil {
				slog.Error("could not write memory profile", "err", err)
				os.Exit(1)
			}
			slog.Info("Memory profile written", "file", cfg.memProfile)
		}
	}
}
