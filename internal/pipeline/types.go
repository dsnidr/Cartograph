package pipeline

import (
	"image"

	"github.com/dsnidr/cartograph/internal/render"
)

// Job represents a single region file that needs to be processed.
type Job struct {
	RegionPath string
	X          int
	Z          int
}

// RenderJob represents the output of Stage 2 (Parsing) and the input of Stage 3 (Rendering).
type RenderJob struct {
	RegionPath  string
	X           int
	Z           int
	SurfaceGrid *render.SurfaceGrid
}

// Result represents the final rendered image and heightmap for a single region.
type Result struct {
	RegionPath string
	X, Z       int
	Image      *image.RGBA
	Heightmap  *image.Gray16
	Biomemap   *image.Gray16
	Err        error
}
