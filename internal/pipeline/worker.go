package pipeline

import (
	"context"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strings"
	"sync"

	png "github.com/dsnidr/cartograph/internal/fastpng"

	"github.com/dsnidr/cartograph/internal/nbt"
	"github.com/dsnidr/cartograph/internal/region"
	"github.com/dsnidr/cartograph/internal/render"
)

// localWorkerPool wraps the Orchestrator to provide thread-local, lock-free slice caching for intense operations
// like parsing palettes from NBT.
type localWorkerPool struct {
	*Orchestrator
	int64Slices      [][]int64
	blockStateSlices [][]nbt.BlockState
	boolSlices       [][]bool
}

func (p *localWorkerPool) GetInt64Slice() []int64 {
	if n := len(p.int64Slices); n > 0 {
		s := p.int64Slices[n-1]
		p.int64Slices = p.int64Slices[:n-1]
		return s
	}

	return make([]int64, 0, 4096)
}

func (p *localWorkerPool) PutInt64Slice(s []int64) {
	if s != nil {
		p.int64Slices = append(p.int64Slices, s[:0])
	}
}

func (p *localWorkerPool) GetBlockStateSlice() []nbt.BlockState {
	if n := len(p.blockStateSlices); n > 0 {
		s := p.blockStateSlices[n-1]
		p.blockStateSlices = p.blockStateSlices[:n-1]
		return s
	}

	return make([]nbt.BlockState, 0, 256)
}

func (p *localWorkerPool) PutBlockStateSlice(s []nbt.BlockState) {
	if s != nil {
		for i := range s {
			s[i].Name = ""
			if s[i].Properties != nil {
				clear(s[i].Properties)
			}
		}

		p.blockStateSlices = append(p.blockStateSlices, s[:0])
	}
}

func (p *localWorkerPool) GetBoolSlice() []bool {
	if n := len(p.boolSlices); n > 0 {
		s := p.boolSlices[n-1]
		p.boolSlices = p.boolSlices[:n-1]
		return s
	}

	return make([]bool, 0, 256)
}

func (p *localWorkerPool) PutBoolSlice(s []bool) {
	if s != nil {
		p.boolSlices = append(p.boolSlices, s[:0])
	}
}

// StartWorkers spins up a bounded pool of goroutines to process incoming jobs. It returns a receive-only channel
// of result structs.
func (o *Orchestrator) StartWorkers(ctx context.Context, numWorkers int, jobs <-chan Job) <-chan Result {
	renderJobs := make(chan RenderJob, numWorkers)
	results := make(chan Result, numWorkers)

	var parserWg sync.WaitGroup
	var renderWg sync.WaitGroup

	// NBT parser stage
	for range numWorkers {
		parserWg.Go(func() {
			// Each worker gets its own lock-free local slice cache
			pool := &localWorkerPool{
				Orchestrator:     o,
				int64Slices:      make([][]int64, 0, 1024),
				blockStateSlices: make([][]nbt.BlockState, 0, 1024),
				boolSlices:       make([][]bool, 0, 1024),
			}

			for job := range jobs {
				if ctx.Err() != nil {
					return
				}

				file, err := os.Open(job.RegionPath)
				if err != nil {
					select {
					case <-ctx.Done():
						return
					case results <- Result{RegionPath: job.RegionPath, Err: fmt.Errorf("open region: %w", err)}:
					}
					continue
				}

				surface := new(render.SurfaceGrid)
				err = region.ProcessChunks(ctx, file, o.StringPool, pool, func(chunk *nbt.RegionChunk) error {
					o.Renderer.ExtractChunkSurface(surface, chunk, o.BlockRegistry, o.Scale, pool)

					for _, sec := range chunk.Sections {
						if sec.BlockStates != nil {
							pool.PutInt64Slice(sec.BlockStates.Data)
							pool.PutBlockStateSlice(sec.BlockStates.Palette)
						}
						if sec.Biomes != nil {
							pool.PutInt64Slice(sec.Biomes.Data)
							pool.PutBlockStateSlice(sec.Biomes.Palette)
						}
					}

					return nil
				})
				func() {
					_ = file.Close()
				}()

				if err != nil {
					select {
					case <-ctx.Done():
						return
					case results <- Result{RegionPath: job.RegionPath, Err: fmt.Errorf("process region chunks: %w", err)}:
					}
					continue
				}

				select {
				case <-ctx.Done():
					return
				case renderJobs <- RenderJob{RegionPath: job.RegionPath, X: job.X, Z: job.Z, SurfaceGrid: surface}:
				}
			}
		})
	}

	go func() {
		parserWg.Wait()
		close(renderJobs)
	}()

	// SurfaceGrid to PNG render stage
	for range numWorkers {
		renderWg.Go(func() {
			for rJob := range renderJobs {
				if ctx.Err() != nil {
					return
				}

				img, hm := o.Renderer.ExtractBaseSurface(rJob.SurfaceGrid, o.BlockRegistry, o.Scale)

				if o.OutputMode == OutputModeTiles {
					baseName := filepath.Base(rJob.RegionPath)
					pngName := strings.TrimSuffix(baseName, filepath.Ext(baseName)) + ".png"
					outPath := filepath.Join(o.OutputDir, pngName)

					err := savePNG(outPath, img)

					select {
					case <-ctx.Done():
						return
					case results <- Result{RegionPath: rJob.RegionPath, Err: err}:
					}
				} else {
					select {
					case <-ctx.Done():
						return
					case results <- Result{RegionPath: rJob.RegionPath, X: rJob.X, Z: rJob.Z, Image: img, Heightmap: hm, Err: nil}:
					}
				}
			}
		})
	}

	go func() {
		renderWg.Wait()
		close(results)
	}()

	return results
}

func savePNG(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() {
		_ = f.Close()
	}()

	encoder := png.Encoder{CompressionLevel: png.BestSpeed}
	return encoder.Encode(f, img)
}
