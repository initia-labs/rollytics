package da

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"sort"

	abci "github.com/cometbft/cometbft/abci/types"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	comettypes "github.com/cometbft/cometbft/types"
	gogoproto "github.com/cosmos/gogoproto/proto"

	"github.com/initia-labs/rollytics/indexer/types"
)

// RecoveredBlock holds a block and its txs recovered from the DA layer.
// TxResults are placeholder (code=0, empty events) when not available from DA.
type RecoveredBlock struct {
	Scraped types.ScrapedBlock
	RawTxs  [][]byte // raw tx bytes for building RestTx without REST API
}

// RecoverBlockFromBlobs assembles batches from blobs, decompresses, and returns the block at targetHeight.
// Blobs should be from Celestia (MsgPayForBlobs) for the same chain ID namespace.
// RawTxs is set so the indexer can build RestTx from raw bytes when REST is unavailable.
func RecoverBlockFromBlobs(ctx context.Context, blobs [][]byte, targetHeight int64) (*RecoveredBlock, error) {
	batches := make(map[string]*batchInfo)

	for _, blobData := range blobs {
		if len(blobData) == 0 {
			continue
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		dataType := blobData[0]
		switch dataType {
		case BatchDataTypeHeader:
			header, err := UnmarshalBatchDataHeader(blobData)
			if err != nil {
				continue
			}
			key := batchKey(header.Start, header.End)
			if batches[key] == nil {
				batches[key] = &batchInfo{Start: header.Start, End: header.End, Chunks: nil}
			}
			batches[key].Header = &header
			batches[key].Checksums = header.Checksums

		case BatchDataTypeChunk:
			chunk, err := UnmarshalBatchDataChunk(blobData)
			if err != nil {
				continue
			}
			key := batchKey(chunk.Start, chunk.End)
			if batches[key] == nil {
				batches[key] = &batchInfo{Start: chunk.Start, End: chunk.End, Chunks: nil}
			}
			batches[key].Chunks = append(batches[key].Chunks, &chunk)
		}
	}

	// Find batch that contains targetHeight
	for _, batch := range batches {
		if batch.Header == nil || len(batch.Chunks) == 0 {
			continue
		}
		if !batch.Header.ContainsHeight(targetHeight) {
			continue
		}

		sort.Slice(batch.Chunks, func(i, j int) bool {
			return batch.Chunks[i].Index < batch.Chunks[j].Index
		})
		expectedChunks := batch.Chunks[0].Length
		if uint64(len(batch.Chunks)) != expectedChunks {
			continue
		}

		var combined bytes.Buffer
		for i, c := range batch.Chunks {
			if c.Index != uint64(i) {
				continue
			}
			combined.Write(c.ChunkData)
		}
		batchData := combined.Bytes()
		if !VerifyChecksums(batchData, batch.Checksums) {
			continue
		}

		blocks, err := DecompressBatch(batchData)
		if err != nil {
			continue
		}

		for _, blockBz := range blocks {
			block, err := unmarshalBlock(blockBz)
			if err != nil {
				continue
			}
			if block.Height == targetHeight {
				return blockToRecovered(block)
			}
		}
	}

	return nil, fmt.Errorf("block height %d not found in DA blobs", targetHeight)
}

func batchKey(start, end uint64) string {
	return fmt.Sprintf("%d-%d", start, end)
}

type batchInfo struct {
	Start     uint64
	End       uint64
	Header    *BatchDataHeader
	Chunks    []*BatchDataChunk
	Checksums [][]byte
}

func unmarshalBlock(blockBz []byte) (*comettypes.Block, error) {
	pbb := new(cmtproto.Block)
	if err := gogoproto.Unmarshal(blockBz, pbb); err != nil {
		return nil, err
	}
	return blockFromProto(pbb)
}

func blockFromProto(bp *cmtproto.Block) (*comettypes.Block, error) {
	if bp == nil {
		return nil, fmt.Errorf("nil block")
	}
	b := new(comettypes.Block)
	h, err := comettypes.HeaderFromProto(&bp.Header)
	if err != nil {
		return nil, err
	}
	b.Header = h
	data, err := comettypes.DataFromProto(&bp.Data)
	if err != nil {
		return nil, err
	}
	b.Data = data
	if err := b.Evidence.FromProto(&bp.Evidence); err != nil {
		return nil, err
	}
	if bp.LastCommit != nil {
		lc, err := comettypes.CommitFromProto(bp.LastCommit)
		if err != nil {
			return nil, err
		}
		b.LastCommit = lc
	}
	return b, nil
}

func blockToRecovered(block *comettypes.Block) (*RecoveredBlock, error) {
	timestamp := block.Header.Time
	proposerStr := fmt.Sprintf("%X", block.Header.ProposerAddress)
	if proposerStr == "" {
		proposerStr = "unknown"
	}

	txs := make([]string, 0, len(block.Txs))
	rawTxs := make([][]byte, 0, len(block.Txs))
	for _, tx := range block.Txs {
		txs = append(txs, base64.StdEncoding.EncodeToString(tx))
		rawTxs = append(rawTxs, tx)
	}

	// Placeholder tx results (DA does not provide ExecTxResult)
	txResults := make([]abci.ExecTxResult, len(block.Txs))
	for i := range txResults {
		txResults[i] = abci.ExecTxResult{
			Code:      0,
			Data:      nil,
			Log:       "",
			Info:      "",
			GasWanted: 0,
			GasUsed:   0,
			Events:    nil,
			Codespace: "",
		}
	}

	return &RecoveredBlock{
		Scraped: types.ScrapedBlock{
			ChainId:    block.Header.ChainID,
			Height:     block.Height,
			Timestamp:  timestamp,
			Hash:       block.Header.Hash().String(),
			Proposer:   proposerStr,
			Txs:        txs,
			TxResults:  txResults,
			PreBlock:   nil,
			BeginBlock: nil,
			EndBlock:   nil,
		},
		RawTxs: rawTxs,
	}, nil
}
