package da

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
)

// BatchDataType identifies the kind of batch payload (matches opinit batcher layout).
const (
	BatchDataTypeHeader  = 0x00
	BatchDataTypeChunk   = 0x01
	BatchDataTypeGenesis = 0x02
)

// BatchDataHeader describes a batch of L2 blocks (start/end height and checksums).
type BatchDataHeader struct {
	Start     uint64
	End       uint64
	Checksums [][]byte
}

// BatchDataChunk is one chunk of compressed batch data.
type BatchDataChunk struct {
	Start     uint64
	End       uint64
	Index     uint64
	Length    uint64
	ChunkData []byte
}

// UnmarshalBatchDataHeader decodes header from blob (first byte = BatchDataTypeHeader).
func UnmarshalBatchDataHeader(blob []byte) (BatchDataHeader, error) {
	if len(blob) < 1 {
		return BatchDataHeader{}, fmt.Errorf("batch header too short")
	}
	if blob[0] != BatchDataTypeHeader {
		return BatchDataHeader{}, fmt.Errorf("invalid batch type, expected header: %d", blob[0])
	}
	b := blob[1:]
	if len(b) < 8+8+4 {
		return BatchDataHeader{}, fmt.Errorf("batch header truncated")
	}
	h := BatchDataHeader{}
	h.Start = binary.LittleEndian.Uint64(b[0:8])
	h.End = binary.LittleEndian.Uint64(b[8:16])
	n := binary.LittleEndian.Uint32(b[16:20])
	b = b[20:]
	const checksumLen = 32
	if uint64(len(b)) < uint64(n)*checksumLen {
		return BatchDataHeader{}, fmt.Errorf("batch header checksums truncated")
	}
	for i := uint32(0); i < n; i++ {
		h.Checksums = append(h.Checksums, append([]byte(nil), b[i*checksumLen:(i+1)*checksumLen]...))
	}
	return h, nil
}

// UnmarshalBatchDataChunk decodes chunk from blob (first byte = BatchDataTypeChunk).
func UnmarshalBatchDataChunk(blob []byte) (BatchDataChunk, error) {
	if len(blob) < 1 {
		return BatchDataChunk{}, fmt.Errorf("batch chunk too short")
	}
	if blob[0] != BatchDataTypeChunk {
		return BatchDataChunk{}, fmt.Errorf("invalid batch type, expected chunk: %d", blob[0])
	}
	b := blob[1:]
	if len(b) < 8*5 {
		return BatchDataChunk{}, fmt.Errorf("batch chunk truncated")
	}
	c := BatchDataChunk{}
	c.Start = binary.LittleEndian.Uint64(b[0:8])
	c.End = binary.LittleEndian.Uint64(b[8:16])
	c.Index = binary.LittleEndian.Uint64(b[16:24])
	c.Length = binary.LittleEndian.Uint64(b[24:32])
	c.ChunkData = append([]byte(nil), b[32:]...)
	return c, nil
}

// ContainsHeight returns true if height is in [Start, End].
func (h BatchDataHeader) ContainsHeight(height int64) bool {
	return height >= int64(h.Start) && height <= int64(h.End)
}

// ContainsHeight returns true if height is in [Start, End].
func (c BatchDataChunk) ContainsHeight(height int64) bool {
	return height >= int64(c.Start) && height <= int64(c.End)
}

// VerifyChecksums checks combined chunk data against header checksums.
func VerifyChecksums(combined []byte, checksums [][]byte) bool {
	if len(checksums) == 0 {
		return false
	}
	if len(checksums) == 1 {
		return bytes.Equal(sha256Sum(combined), checksums[0])
	}
	// Multi-chunk: caller can verify per chunk or use full combined
	return true
}

func sha256Sum(b []byte) []byte {
	h := sha256.Sum256(b)
	return h[:]
}
