package da

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"fmt"
	"io"
)

// DecompressBatch decompresses batch data and splits into raw block bytes.
// Uses 8-byte little-endian length prefix per block (matches opinit batcher).
func DecompressBatch(b []byte) ([][]byte, error) {
	br := bytes.NewReader(b)
	r, err := gzip.NewReader(br)
	if err != nil {
		return nil, fmt.Errorf("gzip reader: %w", err)
	}
	defer r.Close()

	res, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("decompress: %w", err)
	}

	var blocks [][]byte
	for offset := 0; offset < len(res); {
		if offset+8 > len(res) {
			break
		}
		blockLen := int(binary.LittleEndian.Uint64(res[offset : offset+8]))
		offset += 8
		if blockLen <= 0 || offset+blockLen > len(res) {
			return nil, fmt.Errorf("invalid block length %d at offset %d", blockLen, offset-8)
		}
		blocks = append(blocks, res[offset:offset+blockLen])
		offset += blockLen
	}
	return blocks, nil
}
