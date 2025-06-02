package types

import (
	"encoding/json"
	"time"
)

type Table struct {
	Model interface{}
	Name  string
}

type CollectedSeqInfo struct {
	ChainId  string `gorm:"type:text;primaryKey"`
	Name     string `gorm:"type:text;primaryKey"`
	Sequence uint64 `gorm:"type:bigint"`
}

type CollectedBlock struct {
	ChainId   string          `gorm:"type:text;primaryKey"`
	Height    int64           `gorm:"type:bigint;primaryKey;autoIncrement:false;index:block_height"`
	Hash      string          `gorm:"type:text"`
	Timestamp time.Time       `gorm:"type:timestamptz;index:block_timestamp;index:block_timestamp_desc,sort:desc"`
	BlockTime int64           `gorm:"type:bigint"`
	Proposer  string          `gorm:"type:text"`
	GasUsed   int64           `gorm:"type:bigint"`
	GasWanted int64           `gorm:"type:bigint"`
	TxCount   int             `gorm:"type:bigint"`
	TotalFee  json.RawMessage `gorm:"type:jsonb"`
}

type CollectedTx struct {
	ChainId  string          `gorm:"type:text;primaryKey"`
	Hash     string          `gorm:"type:text;primaryKey;index:tx_hash"`
	Height   int64           `gorm:"type:bigint;primaryKey;autoIncrement:false;index:tx_height"`
	Sequence uint64          `gorm:"type:bigint;index:tx_sequence"`
	Signer   string          `gorm:"type:text"`
	Data     json.RawMessage `gorm:"type:jsonb"`
}

type CollectedAccountTx struct {
	ChainId string `gorm:"type:text;primaryKey"`
	Hash    string `gorm:"type:text;primaryKey;index:account_tx_hash"`
	Account string `gorm:"type:text;primaryKey;index:account_tx_account"`
	Height  int64  `gorm:"type:bigint;primaryKey;autoIncrement:false;index:account_tx_height"`
}

type CollectedEvmTx struct {
	ChainId  string          `gorm:"type:text;primaryKey"`
	Hash     string          `gorm:"type:text;primaryKey;index:evm_tx_hash"`
	Height   int64           `gorm:"type:bigint;primaryKey;autoIncrement:false;index:evm_tx_height"`
	Sequence uint64          `gorm:"type:bigint;index:evm_tx_sequence"`
	Data     json.RawMessage `gorm:"type:jsonb"`
}

type CollectedEvmAccountTx struct {
	ChainId string `gorm:"type:text;primaryKey"`
	Hash    string `gorm:"type:text;primaryKey;index:evm_account_tx_hash"`
	Account string `gorm:"type:text;primaryKey;index:evm_account_tx_account"`
	Height  int64  `gorm:"type:bigint;primaryKey;autoIncrement:false;index:evm_account_tx_height"`
}

type CollectedNftCollection struct {
	ChainId    string `gorm:"type:text;primaryKey"`
	Addr       string `gorm:"type:text;primaryKey;index:nft_collection_addr"`
	Height     int64  `gorm:"type:bigint;index:nft_collection_height"`
	Name       string `gorm:"type:text;index:nft_collection_name"`
	OriginName string `gorm:"type:text;index:nft_collection_origin_name"`
	Creator    string `gorm:"type:text"`
	NftCount   int64  `gorm:"type:bigint"`
}

type CollectedNft struct {
	ChainId        string `gorm:"type:text;primaryKey"`
	CollectionAddr string `gorm:"type:text;primaryKey;index:nft_collection_addr"`
	TokenId        string `gorm:"type:text;primaryKey;index:nft_token_id"`
	Addr           string `gorm:"type:text;index:nft_addr"` // only used in move
	Height         int64  `gorm:"type:bigint;index:nft_height"`
	Owner          string `gorm:"type:text;index:nft_owner"`
	Uri            string `gorm:"type:text"`
}

// only for wasm and evm
type CollectedNftTx struct {
	ChainId        string `gorm:"type:text;primaryKey"`
	Hash           string `gorm:"type:text;primaryKey;index:nft_tx_hash"`
	CollectionAddr string `gorm:"type:text;primaryKey;index:nft_tx_collection_addr"`
	TokenId        string `gorm:"type:text;primaryKey;index:nft_tx_token_id"`
	Height         int64  `gorm:"type:bigint;primaryKey;autoIncrement:false;index:nft_tx_height"`
}

// only for move
type CollectedFAStore struct {
	ChainId   string `gorm:"type:text;primaryKey"`
	StoreAddr string `gorm:"type:text;primaryKey"`
	Owner     string `gorm:"type:text"`
}

func (CollectedSeqInfo) TableName() string {
	return "seq_info"
}

func (CollectedBlock) TableName() string {
	return "block"
}

func (CollectedTx) TableName() string {
	return "tx"
}

func (CollectedAccountTx) TableName() string {
	return "account_tx"
}

func (CollectedEvmTx) TableName() string {
	return "evm_tx"
}

func (CollectedEvmAccountTx) TableName() string {
	return "evm_account_tx"
}

func (CollectedNftCollection) TableName() string {
	return "nft_collection"
}

func (CollectedNft) TableName() string {
	return "nft"
}

func (CollectedNftTx) TableName() string {
	return "nft_tx"
}
