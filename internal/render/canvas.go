// Package render provides functionality for rendering Minecraft world data into images.
package render

import (
	"fmt"
	"image"
	"image/color"
	"os"
	"sync"
)

// FileBackedImage implements the image.Image interface using a temporary file as its backing store.
// This prevents massive memory allocations for large maps.
type FileBackedImage struct {
	file       *os.File
	bounds     image.Rectangle
	mu         sync.Mutex
	rowBuffer  []byte
	currentRow int
}

// NewFileBackedImage creates a new disk-backed RGBA image of the specified size. The caller is responsible
// for calling Close() when done to clean up the temp file.
func NewFileBackedImage(width, height int, tempPath string) (*FileBackedImage, error) {
	f, err := os.Create(tempPath)
	if err != nil {
		return nil, fmt.Errorf("create temp canvas file: %w", err)
	}

	size := int64(width) * int64(height) * 4
	if err := f.Truncate(size); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("truncate temp canvas file: %w", err)
	}

	return &FileBackedImage{
		file:       f,
		bounds:     image.Rect(0, 0, width, height),
		rowBuffer:  make([]byte, width*4),
		currentRow: -1,
	}, nil
}

// Close removes the temporary file and closes the file handle.
func (c *FileBackedImage) Close() error {
	path := c.file.Name()
	if err := c.file.Close(); err != nil {
		return err
	}

	return os.Remove(path)
}

// ColorModel returns the image's color model (RGBA).
func (c *FileBackedImage) ColorModel() color.Model {
	return color.RGBAModel
}

// Bounds returns the domain for which At can return non-zero color.
func (c *FileBackedImage) Bounds() image.Rectangle {
	return c.bounds
}

// At implements the image.Image interface. It reads a pixel from the disk-backed store at the specified
// position. In order to optimize sequential reads (like those done by png.Encode), it caches an entire
// row of pixels at a time.
func (c *FileBackedImage) At(x, y int) color.Color {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !(image.Point{x, y}.In(c.bounds)) {
		return color.RGBA{}
	}

	// Cache miss. Load the requested row from disk
	if y != c.currentRow {
		rowOffset := int64(y) * int64(c.bounds.Dx()) * 4
		_, err := c.file.ReadAt(c.rowBuffer, rowOffset)
		if err != nil {
			// Return a transparent/empty colour on IO error
			return color.RGBA{}
		}
		c.currentRow = y
	}

	// Read from cache
	idx := x * 4
	return color.RGBA{
		R: c.rowBuffer[idx+0],
		G: c.rowBuffer[idx+1],
		B: c.rowBuffer[idx+2],
		A: c.rowBuffer[idx+3],
	}
}

// WriteSubImage writes an entire image.RGBA block into the canvas at the given top-left offset.
// This is thread-safe as long as no two goroutines write to overlapping regions.
func (c *FileBackedImage) WriteSubImage(offsetX, offsetY int, img *image.RGBA) error {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	canvasWidth := c.bounds.Dx()

	// Ensure the write is completely within canvas bounds
	if offsetX < 0 || offsetY < 0 || offsetX+width > canvasWidth || offsetY+height > c.bounds.Dy() {
		return fmt.Errorf("write out of bounds: offset(%d,%d) img(%dx%d) canvas(%dx%d)",
			offsetX, offsetY, width, height, canvasWidth, c.bounds.Dy())
	}

	stride := width * 4

	for y := range height {
		// Calculate source byte slice
		srcIdx := img.PixOffset(bounds.Min.X, bounds.Min.Y+y)
		rowBytes := img.Pix[srcIdx : srcIdx+stride]

		// Calculate destination file offset
		destOffset := int64(offsetY+y)*int64(canvasWidth)*4 + int64(offsetX)*4

		if _, err := c.file.WriteAt(rowBytes, destOffset); err != nil {
			return fmt.Errorf("write row %d: %w", y, err)
		}
	}

	return nil
}

// ReadSubImage reads a rectangular region from the file-backed canvas.
func (c *FileBackedImage) ReadSubImage(offsetX, offsetY, width, height int) (*image.RGBA, error) {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	canvasWidth := c.bounds.Dx()

	// Handle out-of-bounds by returning what we can (black/transparent for out of bounds)
	for y := range height {
		globalY := offsetY + y
		if globalY < 0 || globalY >= c.bounds.Dy() {
			continue
		}

		// Calculate how much we can read horizontally
		readStartX := offsetX
		readWidth := width
		imgStartX := 0

		if readStartX < 0 {
			imgStartX = -readStartX
			readWidth += readStartX
			readStartX = 0
		}

		if readStartX+readWidth > canvasWidth {
			readWidth = canvasWidth - readStartX
		}

		if readWidth <= 0 {
			continue
		}

		rowBytes := make([]byte, readWidth*4)
		fileOffset := int64(globalY)*int64(canvasWidth)*4 + int64(readStartX)*4
		_, err := c.file.ReadAt(rowBytes, fileOffset)
		if err != nil {
			return nil, fmt.Errorf("read row %d: %w", globalY, err)
		}

		destIdx := img.PixOffset(imgStartX, y)
		copy(img.Pix[destIdx:], rowBytes)
	}

	return img, nil
}

// FileBackedHeightmap implements a disk-backed store for image.Gray16 (uint16) data.
type FileBackedHeightmap struct {
	file   *os.File
	bounds image.Rectangle
}

// NewFileBackedHeightmap creates a new disk-backed heightmap (Gray16) of the specified size.
func NewFileBackedHeightmap(width, height int, tempPath string) (*FileBackedHeightmap, error) {
	f, err := os.Create(tempPath)
	if err != nil {
		return nil, fmt.Errorf("create temp heightmap file: %w", err)
	}

	size := int64(width) * int64(height) * 2
	if err := f.Truncate(size); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("truncate temp heightmap file: %w", err)
	}

	return &FileBackedHeightmap{
		file:   f,
		bounds: image.Rect(0, 0, width, height),
	}, nil
}

// Close removes the temporary file and closes the file handle.
func (c *FileBackedHeightmap) Close() error {
	path := c.file.Name()
	if err := c.file.Close(); err != nil {
		return err
	}
	return os.Remove(path)
}

// WriteSubHeightmap writes an entire image.Gray16 block into the heightmap at the given offset.
func (c *FileBackedHeightmap) WriteSubHeightmap(offsetX, offsetY int, img *image.Gray16) error {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	canvasWidth := c.bounds.Dx()

	stride := width * 2
	for y := range height {
		srcIdx := img.PixOffset(bounds.Min.X, bounds.Min.Y+y)
		rowBytes := img.Pix[srcIdx : srcIdx+stride]
		destOffset := int64(offsetY+y)*int64(canvasWidth)*2 + int64(offsetX)*2

		if _, err := c.file.WriteAt(rowBytes, destOffset); err != nil {
			return fmt.Errorf("write heightmap row %d: %w", y, err)
		}
	}
	return nil
}

// ReadSubHeightmap reads a rectangular region from the file-backed heightmap.
func (c *FileBackedHeightmap) ReadSubHeightmap(offsetX, offsetY, width, height int) (*image.Gray16, error) {
	img := image.NewGray16(image.Rect(0, 0, width, height))
	canvasWidth := c.bounds.Dx()

	for y := range height {
		globalY := offsetY + y
		if globalY < 0 || globalY >= c.bounds.Dy() {
			continue
		}

		readStartX := offsetX
		readWidth := width
		imgStartX := 0

		if readStartX < 0 {
			imgStartX = -readStartX
			readWidth += readStartX
			readStartX = 0
		}

		if readStartX+readWidth > canvasWidth {
			readWidth = canvasWidth - readStartX
		}

		if readWidth <= 0 {
			continue
		}

		rowBytes := make([]byte, readWidth*2)
		fileOffset := int64(globalY)*int64(canvasWidth)*2 + int64(readStartX)*2
		_, err := c.file.ReadAt(rowBytes, fileOffset)
		if err != nil {
			return nil, fmt.Errorf("read heightmap row %d: %w", globalY, err)
		}

		destIdx := img.PixOffset(imgStartX, y)
		copy(img.Pix[destIdx:], rowBytes)
	}

	return img, nil
}

// FileBackedBiomeMap implements a disk-backed store for image.Gray16 (uint16) data.
type FileBackedBiomeMap struct {
	file   *os.File
	bounds image.Rectangle
}

// NewFileBackedBiomeMap creates a new disk-backed biome map (Gray16) of the specified size.
func NewFileBackedBiomeMap(width, height int, tempPath string) (*FileBackedBiomeMap, error) {
	f, err := os.Create(tempPath)
	if err != nil {
		return nil, fmt.Errorf("create temp biome map file: %w", err)
	}

	size := int64(width) * int64(height) * 2 // 2 bytes per pixel
	if err := f.Truncate(size); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("truncate temp biome map file: %w", err)
	}

	return &FileBackedBiomeMap{
		file:   f,
		bounds: image.Rect(0, 0, width, height),
	}, nil
}

// Close removes the temporary file and closes the file handle.
func (c *FileBackedBiomeMap) Close() error {
	path := c.file.Name()
	if err := c.file.Close(); err != nil {
		return err
	}
	return os.Remove(path)
}

// WriteSubBiomeMap writes an entire image.Gray16 block into the biome map at the given offset.
func (c *FileBackedBiomeMap) WriteSubBiomeMap(offsetX, offsetY int, img *image.Gray16) error {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	canvasWidth := c.bounds.Dx()

	for y := range height {
		srcIdx := img.PixOffset(bounds.Min.X, bounds.Min.Y+y)
		rowBytes := img.Pix[srcIdx : srcIdx+width*2]
		destOffset := (int64(offsetY+y)*int64(canvasWidth) + int64(offsetX)) * 2

		if _, err := c.file.WriteAt(rowBytes, destOffset); err != nil {
			return fmt.Errorf("write biome map row %d: %w", y, err)
		}
	}
	return nil
}

// ReadSubBiomeMap reads a rectangular region from the file-backed biome map.
func (c *FileBackedBiomeMap) ReadSubBiomeMap(offsetX, offsetY, width, height int) (*image.Gray16, error) {
	img := image.NewGray16(image.Rect(0, 0, width, height))
	canvasWidth := c.bounds.Dx()

	for y := range height {
		globalY := offsetY + y
		if globalY < 0 || globalY >= c.bounds.Dy() {
			continue
		}

		readStartX := offsetX
		readWidth := width
		imgStartX := 0

		if readStartX < 0 {
			imgStartX = -readStartX
			readWidth += readStartX
			readStartX = 0
		}

		if readStartX+readWidth > canvasWidth {
			readWidth = canvasWidth - readStartX
		}

		if readWidth <= 0 {
			continue
		}

		rowBytes := make([]byte, readWidth*2)
		fileOffset := (int64(globalY)*int64(canvasWidth) + int64(readStartX)) * 2
		_, err := c.file.ReadAt(rowBytes, fileOffset)
		if err != nil {
			return nil, fmt.Errorf("read biome map row %d: %w", globalY, err)
		}

		destIdx := img.PixOffset(imgStartX, y)
		copy(img.Pix[destIdx:], rowBytes)
	}

	return img, nil
}
