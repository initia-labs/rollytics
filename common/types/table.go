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
	Txs       []CollectedTx   `gorm:"foreignKey:ChainId,Height"`
}

type CollectedTx struct {
	ChainId    string               `gorm:"type:text;primaryKey"`
	Hash       string               `gorm:"type:text;primaryKey;index:tx_hash"`
	Height     int64                `gorm:"type:bigint;primaryKey;autoIncrement:false;index:tx_height"`
	Sequence   uint64               `gorm:"type:bigint;index:tx_sequence"`
	Data       json.RawMessage      `gorm:"type:jsonb"`
	AccountTxs []CollectedAccountTx `gorm:"foreignKey:ChainId,Hash,Height"`
}

type CollectedAccountTx struct {
	ChainId  string `gorm:"type:text;primaryKey"`
	Hash     string `gorm:"type:text;primaryKey;index:account_tx_hash"`
	Account  string `gorm:"type:text;primaryKey;index:account_tx_account"`
	Height   int64  `gorm:"type:bigint;primaryKey;autoIncrement:false;index:account_tx_height"`
	Sequence uint64 `gorm:"type:bigint;index:account_tx_sequence"`
}

type CollectedNftCollection struct {
	ChainId     string         `gorm:"type:text;primaryKey"`
	Addr        string         `gorm:"type:text;primaryKey;index:nft_collection_addr"`
	Height      int64          `gorm:"type:bigint;index:nft_collection_height"`
	Name        string         `gorm:"type:text;index:nft_collection_name"`
	Creator     string         `gorm:"type:text"`
	Description string         `gorm:"type:text"`
	NftCount    int64          `gorm:"type:bigint"`
	Nfts        []CollectedNft `gorm:"foreignKey:ChainId,CollectionAddr"`
}

type CollectedNft struct {
	ChainId        string `gorm:"type:text;primaryKey"`
	CollectionAddr string `gorm:"type:text;primaryKey;index:nft_collection_addr"`
	TokenId        string `gorm:"type:text;primaryKey;index:nft_token_id"`
	Height         int64  `gorm:"type:bigint;index:nft_height"`
	Owner          string `gorm:"type:text;index:nft_owner"`
	Description    string `gorm:"type:text"`
	Uri            string `gorm:"type:text"`
}

type CollectedNftPair struct {
	L1Collection string `gorm:"type:text;primaryKey"`
	L2Collection string `gorm:"type:text;primaryKey"`
	L2ChainId    string `gorm:"type:text;primaryKey"`
	Path         string `gorm:"type:text;primaryKey"`
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

func (CollectedNftCollection) TableName() string {
	return "nft_collection"
}

func (CollectedNft) TableName() string {
	return "nft"
}

func (CollectedNftPair) TableName() string {
	return "nft_pair"
}
