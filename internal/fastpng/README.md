# internal/fastpng

This package is a direct clone of Go's standard library `image/png` (Go 1.22+ structure).

## Why though?

Go's native `image/png` package is hard-coded to use the standard library `compress/zlib`. While the standard zlib is stable, it is single-threaded and lacks modern SIMD hardware acceleration.

For high-resolution map generation, the PNG encoding phase is a huge wall-clock bottleneck. Since there's no good way to "swap" the zlib implementation used by the standard library (and since I really REALLY would rather not use CGO with libdeflate), I've vendored the `image/png` source code here.

## Modifications

1.  **SIMD Acceleration**: The import of `"compress/zlib"` in `writer.go` (and `reader.go`) has been replaced with `"github.com/klauspost/compress/zlib"`.
2.  **Performance**: This change allows the PNG encoder to utilize SIMD instructions, resulting in a 2x-3x speedup for the final encoding phase of large maps.

## Maintenance

When upgrading the project's Go version, this package should be reviewed against the current standard library `image/png` source to incorporate any upstream fixes or security patches, ensuring the `klauspost` import replacement is maintained.
