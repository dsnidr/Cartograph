// Package pipeline defines the multi-stage processing pipeline for rendering regions.
package pipeline

import (
	"context"
	"fmt"
	"math"
	"path/filepath"
	"strconv"
	"strings"
)

// RegionBounds keeps track of the spatial extents of all discovered regions
type RegionBounds struct {
	MinX int
	MaxX int
	MinZ int
	MaxZ int
}

// DiscoverRegions scans a directory for mca region files, determines the overall spatial bounds of the map
// and returns a buffered channel pre-populated with all parsing jobs.
func DiscoverRegions(ctx context.Context, dir string) (jobQueue <-chan Job, total int, bounds RegionBounds, err error) {
	bounds = RegionBounds{
		MinX: math.MaxInt32,
		MaxX: math.MinInt32,
		MinZ: math.MaxInt32,
		MaxZ: math.MinInt32,
	}

	matches, err := filepath.Glob(filepath.Join(dir, "*.mca"))
	if err != nil {
		return nil, 0, bounds, fmt.Errorf("failed to scan directory: %w", err)
	}

	totalJobs := len(matches)
	if totalJobs == 0 {
		return nil, 0, bounds, nil
	}

	jobs := make(chan Job, totalJobs)

	for _, match := range matches {
		if err := ctx.Err(); err != nil {
			close(jobs)
			return jobs, 0, bounds, err
		}

		base := filepath.Base(match)
		parts := strings.Split(base, ".")

		x, z := 0, 0
		if len(parts) == 4 && parts[0] == "r" {
			x, _ = strconv.Atoi(parts[1])
			z, _ = strconv.Atoi(parts[2])

			if x < bounds.MinX {
				bounds.MinX = x
			}
			if x > bounds.MaxX {
				bounds.MaxX = x
			}
			if z < bounds.MinZ {
				bounds.MinZ = z
			}
			if z > bounds.MaxZ {
				bounds.MaxZ = z
			}
		}

		jobs <- Job{RegionPath: match, X: x, Z: z}
	}
	close(jobs)

	return jobs, totalJobs, bounds, nil
}
