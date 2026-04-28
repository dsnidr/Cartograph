package pipeline

import (
	"bufio"
	"sync"

	"github.com/dsnidr/cartograph/internal/nbt"
	"github.com/dsnidr/cartograph/internal/registry"
	"github.com/dsnidr/cartograph/internal/render"
)

// OutputMode defines the final format of the rendered map images.
type OutputMode string

const (
	// OutputModeTiles outputs one separate PNG image per region file.
	OutputModeTiles OutputMode = "tiles"

	// OutputModeComposite outputs a single large PNG image containing all regions.
	OutputModeComposite OutputMode = "composite"
)

// Orchestrator manages the pipeline for parsing and rendering regions.
type Orchestrator struct {
	StringPool    *nbt.StringPool
	BlockRegistry *registry.Registry
	OutputDir     string
	Scale         int
	OutputMode    OutputMode
	Renderer      *render.Renderer
	BiomePalette  *render.BiomePalette

	compressionPool sync.Pool
	bufioPool       sync.Pool
}

// NewOrchestrator constructs an Orchestrator with the provided dependencies and configuration.
func NewOrchestrator(sp *nbt.StringPool, br *registry.Registry, outputDir string, scale int, outputMode OutputMode, renderer *render.Renderer) *Orchestrator {
	return &Orchestrator{
		StringPool:    sp,
		BlockRegistry: br,
		OutputDir:     outputDir,
		Scale:         scale,
		OutputMode:    outputMode,
		Renderer:      renderer,
		BiomePalette:  render.NewBiomePalette(),

		compressionPool: sync.Pool{},
		bufioPool:       sync.Pool{},
	}
}

// GetCompressionReader retrieves a reusable compression reader or nil if the pool is empty.
func (o *Orchestrator) GetCompressionReader() any {
	return o.compressionPool.Get()
}

// PutCompressionReader returns a compression reader to the pool for reuse.
func (o *Orchestrator) PutCompressionReader(r any) {
	if r != nil {
		o.compressionPool.Put(r)
	}
}

// GetBufioReader retrieves a reusable bufio.Reader.
func (o *Orchestrator) GetBufioReader() *bufio.Reader {
	if v := o.bufioPool.Get(); v != nil {
		return v.(*bufio.Reader)
	}
	return bufio.NewReaderSize(nil, 4096)
}

// PutBufioReader returns a bufio.Reader to the pool.
func (o *Orchestrator) PutBufioReader(r *bufio.Reader) {
	if r != nil {
		r.Reset(nil)
		o.bufioPool.Put(r)
	}
}
