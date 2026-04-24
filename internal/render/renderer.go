package render

import (
	"image"
	"image/color"
	"log/slog"
	"sync"

	"github.com/dsnidr/cartograph/internal/nbt"
	"github.com/dsnidr/cartograph/internal/region"
	"github.com/dsnidr/cartograph/internal/registry"
)

const (
	// RegionChunks is the number of chunks along one axis of a Minecraft region.
	RegionChunks = 32
	// ChunkBlocks is the number of blocks along one axis of a Minecraft chunk.
	ChunkBlocks = 16
	// RegionBlocks is the total number of blocks along one axis of a region.
	RegionBlocks = RegionChunks * ChunkBlocks // 512
)

const (
	// Scale1x1 renders one pixel per block.
	Scale1x1 = 0 // 1:1 Block Precision
	// Scale2x2 downsamples 2x2 blocks into one pixel.
	Scale2x2 = 1 // 2:1 Downsampled
	// Scale4x4 downsamples 4x4 blocks into one pixel.
	Scale4x4 = 2 // 4:1 Downsampled
	// ScaleThumbnail downsamples an entire chunk (16x16) into one pixel.
	ScaleThumbnail = 3 // 16:1 Chunk-to-Pixel

	// ThumbnailStep is the number of blocks represented by one pixel at ScaleThumbnail.
	ThumbnailStep = 16 // Number of blocks represented by one pixel at ScaleThumbnail
	// ThumbnailOffset is the center coordinate within the 16x16 chunk.
	ThumbnailOffset = 8 // Center coordinate within the 16x16 chunk
)

// RendererConfig defines the operational parameters for the Renderer.
type RendererConfig struct {
	DebugMissingBlocks bool
	MaxWaterDepth      int
}

// Renderer handles the raycasting and colour resolving of Minecraft regions.
type Renderer struct {
	Config               RendererConfig
	missingBlocksTracker sync.Map
}

// NewRenderer creates a new Renderer with the specified configuration.
func NewRenderer(cfg RendererConfig) *Renderer {
	return &Renderer{
		Config: cfg,
	}
}

// ExtractSurface runs the raycaster over the raw NBT region data and returns a lightweight SurfaceGrid.
func (r *Renderer) ExtractSurface(reg *region.Region, blockReg *registry.Registry, scale int) *SurfaceGrid {
	surface := new(SurfaceGrid)

	for _, chunk := range reg.Chunks {
		r.ExtractChunkSurface(surface, chunk, blockReg, scale, nil)
	}

	return surface
}

// BoolPool allows for reusing boolean slices to avoid allocation churn.
type BoolPool interface {
	GetBoolSlice() []bool
	PutBoolSlice([]bool)
}

// ExtractChunkSurface runs the raycaster over a single chunk and updates the SurfaceGrid.
// This method expects the chunk's sections to already be sorted in descending Y order!
func (r *Renderer) ExtractChunkSurface(surface *SurfaceGrid, chunk *nbt.RegionChunk, blockReg *registry.Registry, scale int, boolPool BoolPool) {
	localChunkX := int(chunk.XPos) & 31
	localChunkZ := int(chunk.ZPos) & 31

	pixelOffsetX := localChunkX * ChunkBlocks
	pixelOffsetZ := localChunkZ * ChunkBlocks

	sections := chunk.Sections

	sectionTransparencies := make([][]bool, len(sections))
	sectionIsWater := make([][]bool, len(sections))
	for i := range sections {
		sec := &sections[i]
		if sec.BlockStates != nil {
			length := len(sec.BlockStates.Palette)
			var secTrans []bool
			var secWater []bool

			if boolPool != nil {
				pooled := boolPool.GetBoolSlice()
				if length > cap(pooled) {
					pooled = make([]bool, 0, length)
				}
				secTrans = pooled[:length]
				
				pooledW := boolPool.GetBoolSlice()
				if length > cap(pooledW) {
					pooledW = make([]bool, 0, length)
				}
				secWater = pooledW[:length]
			} else {
				secTrans = make([]bool, length)
				secWater = make([]bool, length)
			}

			allTransparent := true
			for j := range length {
				name := sec.BlockStates.Palette[j].Name
				trans := blockReg.IsTransparent(name)
				// Make water transparent so the raycast passes through it to find the sea floor
				isWaterBlock := name == "minecraft:water" || sec.BlockStates.Palette[j].HasProperty("waterlogged", "true")
				secWater[j] = isWaterBlock
				if isWaterBlock {
					trans = true
				}

				secTrans[j] = trans
				if !trans {
					allTransparent = false
				}
			}

			if allTransparent {
				if boolPool != nil {
					boolPool.PutBoolSlice(secTrans)
					// boolPool.PutBoolSlice(secWater) // we still need secWater
				}
				sectionTransparencies[i] = nil
				sectionIsWater[i] = secWater
			} else {
				sectionTransparencies[i] = secTrans
				sectionIsWater[i] = secWater
			}
		}
	}

	if scale == ScaleThumbnail {
		block, y, biome, waterDepth := RaycastColumn(sections, sectionTransparencies, sectionIsWater, ThumbnailOffset, ThumbnailOffset, r.Config.MaxWaterDepth)
		if block != nil {
			surface[pixelOffsetX+ThumbnailOffset][pixelOffsetZ+ThumbnailOffset] = BlockHit{
				BlockName:     block.Name,
				IsWaterlogged: block.HasProperty("waterlogged", "true"),
				Y:             y,
				BiomeName:     biome,
				HasBlock:      true,
				WaterDepth:    waterDepth,
			}
		}

		return
	}

	for x := range ChunkBlocks {
		for z := range ChunkBlocks {
			block, y, biome, waterDepth := RaycastColumn(sections, sectionTransparencies, sectionIsWater, x, z, r.Config.MaxWaterDepth)

			if block != nil {
				surface[pixelOffsetX+x][pixelOffsetZ+z] = BlockHit{
					BlockName:     block.Name,
					IsWaterlogged: block.HasProperty("waterlogged", "true"),
					Y:             y,
					BiomeName:     biome,
					HasBlock:      true,
					WaterDepth:    waterDepth,
				}
			}
		}
	}

	// Return slices back to the pool
	if boolPool != nil {
		for i, arr := range sectionTransparencies {
			if arr != nil {
				boolPool.PutBoolSlice(arr)
			}
			if sectionIsWater[i] != nil {
				boolPool.PutBoolSlice(sectionIsWater[i])
			}
		}
	}
}

// ExtractBaseSurface downsamples a surface grid into an image and a heightmap.
func (r *Renderer) ExtractBaseSurface(surface *SurfaceGrid, blockReg *registry.Registry, scale int) (*image.RGBA, *image.Gray16) {
	var step int
	switch scale {
	case Scale2x2:
		step = 2
	case Scale4x4:
		step = 4
	case ScaleThumbnail:
		step = ThumbnailStep
	default:
		step = 1
	}

	outSize := RegionBlocks / step
	img := image.NewRGBA(image.Rect(0, 0, outSize, outSize))
	hm := image.NewGray16(image.Rect(0, 0, outSize, outSize))

	for outX := range outSize {
		for outZ := range outSize {
			if scale == ScaleThumbnail {
				globalX := outX*ThumbnailStep + ThumbnailOffset
				globalZ := outZ*ThumbnailStep + ThumbnailOffset
				hit := surface[globalX][globalZ]
				if hit.HasBlock {
					img.Set(outX, outZ, r.resolveColour(&hit, blockReg))
					hm.SetGray16(outX, outZ, color.Gray16{Y: uint16(hit.Y + 128)})
				}
				continue
			}

			var rSum, gSum, bSum, aSum uint32
			var ySum int
			var count uint32

			for dx := range step {
				for dz := range step {
					globalX := outX*step + dx
					globalZ := outZ*step + dz

					hit := surface[globalX][globalZ]
					if !hit.HasBlock {
						continue
					}

					baseColour := r.resolveColour(&hit, blockReg)
					r, g, b, a := baseColour.RGBA()

					rSum += r
					gSum += g
					bSum += b
					aSum += a
					ySum += hit.Y
					count++
				}
			}

			if count > 0 {
				img.Set(outX, outZ, color.RGBA{
					R: uint8((rSum / count) >> 8),
					G: uint8((gSum / count) >> 8),
					B: uint8((bSum / count) >> 8),
					A: uint8((aSum / count) >> 8),
				})
				// Average height for the downsampled pixel
				avgY := ySum / int(count)
				hm.SetGray16(outX, outZ, color.Gray16{Y: uint16(avgY + 128)})
			}
		}
	}

	return img, hm
}

// resolveColour takes a BlockHit and maps it to a color.RGBA value
func (r *Renderer) resolveColour(hit *BlockHit, blockReg *registry.Registry) color.RGBA {
	biomeName := hit.BiomeName
	
	// Check for properties that force a specific render style (e.g., `waterlogged`)
	if hit.IsWaterlogged || hit.WaterDepth > 0 {
		water, _ := blockReg.GetColour("minecraft:water")
		
		waterCol := blockReg.ApplyBiomeTint(water.Base, biomeName, registry.TintWater)

		// Fast path: if water depth is at or beyond the configured maximum, skip floor blending
		// and return purely opaque water color for massive zlib compression gains.
		if r.Config.MaxWaterDepth >= 0 && hit.WaterDepth >= r.Config.MaxWaterDepth {
			return waterCol
		}

		// Alpha blend the water colour with the floor block colour
		if hit.WaterDepth > 0 {
			floorC, _ := blockReg.GetColour(hit.BlockName)
			floorCol := blockReg.ApplyBiomeTint(floorC.Base, biomeName, floorC.Tint)

			// Simple depth based opacity
			opacity := 0.6 + float64(hit.WaterDepth)*0.05
			if opacity > 0.95 {
				opacity = 0.95
			}
			
			return color.RGBA{
				R: uint8(float64(waterCol.R)*opacity + float64(floorCol.R)*(1.0-opacity)),
				G: uint8(float64(waterCol.G)*opacity + float64(floorCol.G)*(1.0-opacity)),
				B: uint8(float64(waterCol.B)*opacity + float64(floorCol.B)*(1.0-opacity)),
				A: 255,
			}
		}

		return waterCol
	}

	c, wasFound := blockReg.GetColour(hit.BlockName)

	if !wasFound && r.Config.DebugMissingBlocks {
		// log each missing block once to avoid useless spam
		_, alreadyLogged := r.missingBlocksTracker.LoadOrStore(hit.BlockName, true)
		if !alreadyLogged {
			slog.Debug("missing block colour mapping, generating deterministic fallback", "block", hit.BlockName)
		}
	}

	return blockReg.ApplyBiomeTint(c.Base, biomeName, c.Tint)
}
