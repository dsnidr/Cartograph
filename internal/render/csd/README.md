# Cartograph Spatial Data (.csd) Format

The Cartograph Spatial Data (`.csd`) format is a highly compact, binary file designed to store structural metadata alongside rendered map images. It provides precise information about the topmost block's Y-coordinate (height) and its corresponding biome for each chunk column.

## File Structure

The **entire file** is a `zstd` (Zstandard) compressed stream. You must stream the file through a Zstd decompressor to read the internal binary structure.

Once decompressed, the byte stream follows this exact sequence:

| Section | Size (Bytes) | Type | Endianness | Description |
| :--- | :--- | :--- | :--- | :--- |
| **Header Length** | 4 | `uint32` | Little Endian | The length of the JSON header in bytes (`N`). |
| **Header Data** | `N` | `[]byte` | UTF-8 | A JSON object containing map dimensions and palette. |
| **Heightmap** | `W * H * 2` | `[]int16` | Little Endian | A 1D flattened array of absolute Y-coordinates. |
| **Biomemap** | `W * H * 1` | `[]uint8` | - | A 1D flattened array of indices pointing to the biome palette. |

### 1. JSON Header
The JSON header contains metadata necessary to parse the rest of the stream.
```json
{
  "version": 1,
  "width": 2048,
  "height": 2048,
  "scale": 1,
  "biome_palette": [
    "",
    "minecraft:plains",
    "minecraft:ocean",
    "minecraft:desert"
  ]
}
```
*   `version`: Currently `1`.
*   `width` / `height`: The dimensions of the map in pixels.
*   `scale`: The scale of the map (e.g., `1` for 1:1, `2` for 2:1 downsampling).
*   `biome_palette`: An array of biome names. The index of the string in this array corresponds to the `uint8` value in the Biomemap array. **Index 0 is always an empty string `""` and represents missing, ungenerated, or void data.**

### 2. Heightmap Data
Immediately following the JSON header is the heightmap data. 
*   **Format**: `int16` (Little Endian).
*   **Size**: `width * height * 2` bytes.
*   **Layout**: Row-major order (top-to-bottom, left-to-right).
*   **Value**: The absolute Y-coordinate of the surface block (e.g., `-64` to `320`).

### 3. Biomemap Data
Immediately following the heightmap data is the biomemap data.
*   **Format**: `uint8` (1 byte per pixel).
*   **Size**: `width * height` bytes.
*   **Layout**: Row-major order (matching the heightmap).
*   **Value**: An index mapping directly to the `biome_palette` array in the JSON header.

---

## How to Decode

Because the `.csd` file is designed for interoperability, it can be easily parsed in almost any language. Below is an example using Go:

```go
import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/klauspost/compress/zstd"
)

type CSDHeader struct {
	Version      int      `json:"version"`
	Width        int      `json:"width"`
	Height       int      `json:"height"`
	Scale        int      `json:"scale"`
	BiomePalette []string `json:"biome_palette"`
}

func decodeCSD(filepath string) {
	f, _ := os.Open(filepath)
	defer f.Close()

	// 1. Create a Zstd decompressor stream
	zr, _ := zstd.NewReader(f)
	defer zr.Close()

	// 2. Read header length (4 bytes, uint32, Little Endian)
	var headerLen uint32
	binary.Read(zr, binary.LittleEndian, &headerLen)

	// 3. Read and parse the JSON header
	headerBytes := make([]byte, headerLen)
	io.ReadFull(zr, headerBytes)

	var header CSDHeader
	json.Unmarshal(headerBytes, &header)

	numPixels := header.Width * header.Height

	// 4. Read Heightmap (int16 array, Little Endian)
	heights := make([]int16, numPixels)
	binary.Read(zr, binary.LittleEndian, &heights)

	// 5. Read Biomemap (uint8 array)
	biomes := make([]uint8, numPixels)
	io.ReadFull(zr, biomes)

	// Example: Get the data for a specific pixel (x: 15, y: 10)
	x, y := 15, 10
	index := (y * header.Width) + x

	yCoord := heights[index]
	biomeID := biomes[index]
	
	// Index 0 is reserved for missing/void data
	if biomeID == 0 {
		fmt.Printf("Pixel (%d, %d) is void/missing at Y=%d\n", x, y, yCoord)
	} else {
		biomeName := header.BiomePalette[biomeID]
		fmt.Printf("Pixel (%d, %d) is %s at Y=%d\n", x, y, biomeName, yCoord)
	}
}
```
