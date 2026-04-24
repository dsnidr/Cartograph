package pipeline

import (
	"context"
	"image"
	"sync"

	"github.com/dsnidr/cartograph/internal/render"
)

const (
	// ShadingPadding is the number of extra blocks we read on each side for slope/shadows
	ShadingPadding = 16
)

// ShadingJob represents a unit of work for shading a specific rectangular region of the map.
type ShadingJob struct {
	X, Z          int
	Width, Height int
}

// StartShadingWorkers launches a pool of goroutines to apply shading to map tiles in parallel.
func (o *Orchestrator) StartShadingWorkers(ctx context.Context, numWorkers int, jobs <-chan ShadingJob, baseColors *render.FileBackedImage, heightmap *render.FileBackedHeightmap) <-chan Result {
	results := make(chan Result, numWorkers)
	var wg sync.WaitGroup

	for range numWorkers {
		wg.Go(func() {
			for job := range jobs {
				if ctx.Err() != nil {
					return
				}

				// Read base colors for this tile
				base, err := baseColors.ReadSubImage(job.X, job.Z, job.Width, job.Height)
				if err != nil {
					results <- Result{Err: err}
					continue
				}

				// Read padded heightmap
				paddedHM, err := heightmap.ReadSubHeightmap(
					job.X-ShadingPadding,
					job.Z-ShadingPadding,
					job.Width+ShadingPadding*2,
					job.Height+ShadingPadding*2,
				)
				if err != nil {
					results <- Result{Err: err}
					continue
				}

				// Apply shading
				final := image.NewRGBA(image.Rect(0, 0, job.Width, job.Height))
				for lx := range job.Width {
					for lz := range job.Height {
						pixel := base.RGBAAt(lx, lz)
						if pixel.A == 0 {
							continue
						}

						shaded := render.ApplyReliefShading(pixel, lx+ShadingPadding, lz+ShadingPadding, paddedHM)
						final.Set(lx, lz, shaded)
					}
				}

				select {
				case <-ctx.Done():
					return
				case results <- Result{
					X:     job.X,
					Z:     job.Z,
					Image: final,
				}:
				}
			}
		})
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	return results
}
