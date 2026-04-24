package render

import (
	"github.com/dsnidr/cartograph/internal/nbt"
)

const minY = -64

// RaycastColumn casts a ray straight down through a chunk at specific x,z coords and returns the topmost
// solid block, its absolute Y-coordinate, the biome name at that location, and the water depth if submerged.
func RaycastColumn(sections []nbt.Section, transparencies [][]bool, isWater [][]bool, localX, localZ int, maxWaterDepth int) (*nbt.BlockState, int, string, int) {
	waterSurfaceY := minY

	for i := range sections {
		section := &sections[i]
		
		if section.BlockStates == nil || len(section.BlockStates.Palette) == 0 || (transparencies[i] == nil && isWater[i] == nil) {
			continue
		}

		if len(section.BlockStates.Palette) == 1 {
			y := ChunkBlocks - 1
			absoluteHeightTop := (int(section.Y) * ChunkBlocks) + y
			absoluteHeightBottom := int(section.Y) * ChunkBlocks
			block := &section.BlockStates.Palette[0]

			if isWater[i] != nil && isWater[i][0] {
				if waterSurfaceY == minY {
					waterSurfaceY = absoluteHeightTop
				}

				if maxWaterDepth >= 0 && (waterSurfaceY - absoluteHeightBottom) >= maxWaterDepth {
					cutoffY := waterSurfaceY - maxWaterDepth
					localY := cutoffY - absoluteHeightBottom
					if localY < 0 {
						localY = 0
					} else if localY >= ChunkBlocks {
						localY = ChunkBlocks - 1
					}

					biome := section.GetBiome(localX, localY, localZ)
					biomeName := "minecraft:plains"
					if biome != nil {
						biomeName = biome.Name
					}
					return &nbt.BlockState{Name: "minecraft:water"}, cutoffY, biomeName, maxWaterDepth
				}
			}

			if transparencies[i] == nil || transparencies[i][0] {
				continue
			}

			biome := section.GetBiome(localX, y, localZ)
			biomeName := "minecraft:plains" // Default fallback
			if biome != nil {
				biomeName = biome.Name
			}

			depth := 0
			if waterSurfaceY != minY {
				depth = waterSurfaceY - absoluteHeightTop
			}
			return block, absoluteHeightTop, biomeName, depth
		}

		// Iterate top down within the section
		for y := ChunkBlocks - 1; y >= 0; y-- {
			palIdx := section.GetBlockIndex(localX, y, localZ)
			if palIdx < 0 || palIdx >= len(section.BlockStates.Palette) {
				continue
			}

			absoluteHeight := (int(section.Y) * ChunkBlocks) + y

			if isWater[i] != nil && isWater[i][palIdx] {
				if waterSurfaceY == minY {
					waterSurfaceY = absoluteHeight
				}

				if maxWaterDepth >= 0 && (waterSurfaceY - absoluteHeight) >= maxWaterDepth {
					cutoffY := waterSurfaceY - maxWaterDepth
					biome := section.GetBiome(localX, y, localZ)
					biomeName := "minecraft:plains"
					if biome != nil {
						biomeName = biome.Name
					}
					return &nbt.BlockState{Name: "minecraft:water"}, cutoffY, biomeName, maxWaterDepth
				}
			}

			if transparencies[i] != nil && !transparencies[i][palIdx] {
				// solid block
				block := &section.BlockStates.Palette[palIdx]

				biome := section.GetBiome(localX, y, localZ)
				biomeName := "minecraft:plains" // Default fallback
				if biome != nil {
					biomeName = biome.Name
				}

				depth := 0
				if waterSurfaceY != minY {
					depth = waterSurfaceY - absoluteHeight
				}
				return block, absoluteHeight, biomeName, depth
			}
		}
	}

	// The ray fell into the void, never to be seen again :(
	return nil, minY, "minecraft:plains", 0
}
