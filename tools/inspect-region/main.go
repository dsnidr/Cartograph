//nolint:all
package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
)

const (
	sectorBytes        = 4096
	maxChunksPerRegion = 1024
	chunkHeaderSize    = 4
)

func main() {
	var file string
	flag.StringVar(&file, "file", "", "Path to the .mca region file to inspect")
	flag.Parse()

	if file == "" {
		log.Fatal("Please provide a --file argument")
	}

	stat, err := os.Stat(file)
	if err != nil {
		log.Fatalf("Failed to stat file: %v", err)
	}

	fmt.Printf("File: %s\n", file)
	fmt.Printf("Size: %d bytes\n", stat.Size())

	if stat.Size() == 0 {
		fmt.Println("Error: File is completely empty (0 bytes).")
		return
	}

	if stat.Size() < sectorBytes {
		fmt.Printf("Error: File is smaller than the required region header size (4096 bytes). Actual size: %d bytes.\n", stat.Size())
		return
	}

	f, err := os.Open(file)
	if err != nil {
		log.Fatalf("Failed to open file: %v", err)
	}
	defer f.Close()

	var locationsData [sectorBytes]byte
	if _, err := io.ReadFull(f, locationsData[:]); err != nil {
		log.Fatalf("Failed to read region locations header: %v", err)
	}

	var timestampsData [sectorBytes]byte
	if _, err := io.ReadFull(f, timestampsData[:]); err != nil {
		fmt.Printf("Warning: Failed to read timestamps header: %v\n", err)
	}

	validChunks := 0
	outOfBoundsChunks := 0

	for i := 0; i < maxChunksPerRegion; i++ {
		offsetIdx := i * chunkHeaderSize

		sectorOffset := int64(locationsData[offsetIdx])<<16 |
			int64(locationsData[offsetIdx+1])<<8 |
			int64(locationsData[offsetIdx+2])

		sectorCount := int(locationsData[offsetIdx+3])

		if sectorOffset == 0 {
			continue
		}

		validChunks++

		chunkOffset := sectorOffset * sectorBytes
		chunkLength := int64(sectorCount) * sectorBytes

		if chunkOffset+chunkLength > stat.Size() {
			outOfBoundsChunks++
			fmt.Printf("Chunk %d (offset %d, len %d) goes beyond EOF!\n", i, chunkOffset, chunkLength)
		}
	}

	fmt.Printf("Valid chunk entries in header: %d\n", validChunks)
	fmt.Printf("Chunks pointing past EOF: %d\n", outOfBoundsChunks)

	if validChunks > 0 && outOfBoundsChunks == 0 {
		fmt.Println("File header looks well-formed.")
	} else if outOfBoundsChunks > 0 {
		fmt.Println("Warning: Some chunks point to data beyond the end of the file. The file may be truncated or corrupted.")
	}
}
