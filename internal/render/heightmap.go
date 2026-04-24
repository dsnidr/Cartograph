package render

// GlobalHeightmap represents a 2D array of absolute Y-coordinates for an entire region.
type GlobalHeightmap [RegionBlocks][RegionBlocks]int

// BuildHeightmapFromSurface performs a fast sweep of the pre-calculated surface grid
// to determine the surface height, storing the results in a 512x512 grid.
func BuildHeightmapFromSurface(surface *SurfaceGrid) *GlobalHeightmap {
	hm := new(GlobalHeightmap)
	for i := range RegionBlocks {
		for j := range RegionBlocks {
			if !surface[i][j].HasBlock {
				hm[i][j] = minY
			} else {
				hm[i][j] = surface[i][j].Y
			}
		}
	}

	return hm
}
