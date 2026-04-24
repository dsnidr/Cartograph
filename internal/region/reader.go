// Package region provides functionality for reading and parsing Minecraft .mca region files.
package region

import (
	"bufio"
	"compress/zlib"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"

	"github.com/dsnidr/cartograph/internal/nbt"
)

const (
	sectorBytes        = 4096
	maxChunksPerRegion = 1024
	chunkHeaderSize    = 4

	// Compression schemes used in region files
	compressionGzip = 1
	compressionZlib = 2
)

// chunkLocation represents the location of a chunk's data within the .mca file.
type chunkLocation struct {
	Offset int64
	Length int
}

// ReaderPool provides an interface to reuse compression and buffered readers as well as NBT palette
// slices to prevent GC churn during chunk parsing.
type ReaderPool interface {
	GetCompressionReader() any
	PutCompressionReader(any)
	GetBufioReader() *bufio.Reader
	PutBufioReader(*bufio.Reader)
	GetInt64Slice() []int64
	PutInt64Slice([]int64)
	GetBlockStateSlice() []nbt.BlockState
	PutBlockStateSlice([]nbt.BlockState)
}

// Parse reads an entire region file from an io.ReaderAt and returns a Region object.
func Parse(ctx context.Context, r io.ReaderAt, pool *nbt.StringPool) (*Region, error) {
	region := &Region{
		Chunks: make([]*nbt.RegionChunk, 0, maxChunksPerRegion),
	}
	err := ProcessChunks(ctx, r, pool, nil, func(chunk *nbt.RegionChunk) error {
		region.Chunks = append(region.Chunks, chunk)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return region, nil
}

// ProcessChunks reads a region file and calls the given callback for each valid chunk parsed.
func ProcessChunks(ctx context.Context, r io.ReaderAt, pool *nbt.StringPool, readerPool ReaderPool, callback func(*nbt.RegionChunk) error) error {
	var locationsData [sectorBytes]byte
	n, err := r.ReadAt(locationsData[:], 0)
	if err != nil {
		if err == io.EOF && n == 0 {
			slog.DebugContext(ctx, "skipping empty region file")
			return nil
		}
		return fmt.Errorf("failed to read region header: %w", err)
	}

	locations := make([]chunkLocation, 0, maxChunksPerRegion)

	for i := range maxChunksPerRegion {
		offsetIdx := i * chunkHeaderSize

		sectorOffset := int64(locationsData[offsetIdx])<<16 |
			int64(locationsData[offsetIdx+1])<<8 |
			int64(locationsData[offsetIdx+2])

		sectorCount := int(locationsData[offsetIdx+3])

		if sectorOffset == 0 {
			continue
		}

		locations = append(locations, chunkLocation{
			Offset: sectorOffset * sectorBytes,
			Length: sectorCount * sectorBytes,
		})
	}

	for _, loc := range locations {
		if err := ctx.Err(); err != nil {
			return err
		}

		section := io.NewSectionReader(r, 0, 1<<63-1)
		if _, err := section.Seek(loc.Offset, io.SeekStart); err != nil {
			return fmt.Errorf("failed to seek to chunk offset %d: %w", loc.Offset, err)
		}

		var header [5]byte
		if _, err := io.ReadFull(section, header[:]); err != nil {
			return fmt.Errorf("failed to read chunk header: %w", err)
		}

		length := binary.BigEndian.Uint32(header[:4])
		compressionScheme := header[4]

		if compressionScheme != compressionZlib {
			slog.WarnContext(ctx, "skipping chunk with unsupported compression", slog.Int("scheme", int(compressionScheme)), slog.Int64("offset", loc.Offset))
			continue
		}

		payloadLen := int64(length - 1)
		payloadReader := io.LimitReader(section, payloadLen)

		var zReader io.ReadCloser
		var zErr error
		if readerPool != nil {
			if poolObj := readerPool.GetCompressionReader(); poolObj != nil {
				if resetter, ok := poolObj.(zlib.Resetter); ok {
					zErr = resetter.Reset(payloadReader, nil)
					if zErr != nil {
						slog.WarnContext(ctx, "failed to reset zlib reader", slog.Any("error", zErr), slog.Int64("offset", loc.Offset))
						continue
					}
					zReader = poolObj.(io.ReadCloser)
				}
			}
		}

		if zReader == nil {
			zReader, zErr = zlib.NewReader(payloadReader)
			if zErr != nil {
				slog.WarnContext(ctx, "failed to read zlib payload", slog.Any("error", zErr), slog.Int64("offset", loc.Offset))
				continue
			}
		}

		var bReader *bufio.Reader
		if readerPool != nil {
			bReader = readerPool.GetBufioReader()
			bReader.Reset(zReader)
		} else {
			bReader = bufio.NewReader(zReader)
		}

		var slicePool nbt.SlicePool
		if rp, ok := readerPool.(nbt.SlicePool); ok {
			slicePool = rp
		}

		chunk, err := nbt.ParseTargetedChunk(ctx, bReader, pool, slicePool)
		_ = zReader.Close()

		if readerPool != nil {
			readerPool.PutBufioReader(bReader)
			readerPool.PutCompressionReader(zReader)
		}

		if err != nil {
			slog.WarnContext(ctx, "failed to parse chunk NBT", slog.Any("error", err), slog.Int64("offset", loc.Offset))
			continue
		}

		if chunk == nil {
			// happens when status is not `minecraft:full`
			continue
		}

		if err := callback(chunk); err != nil {
			return err
		}
	}

	return nil
}
