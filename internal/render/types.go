package render

// BlockHit represents the surface block at a specific X,Z coordinate.
// It is lightweight enough to pass through channels without blowing up memory.
type BlockHit struct {
	BlockName     string
	IsWaterlogged bool
	BiomeName     string
	Y             int
	HasBlock      bool
	WaterDepth    int
}

// SurfaceGrid is a 512x512 array of BlockHits representing the exposed surface of a region.
type SurfaceGrid [RegionBlocks][RegionBlocks]BlockHit
