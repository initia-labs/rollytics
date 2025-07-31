package types

import (
	"encoding/json"
	"time"

	"github.com/lib/pq"
)

type Table struct {
	Model interface{}
	Name  string
}

type CollectedSeqInfo struct {
	Name     string `gorm:"type:text;primaryKey"`
	Sequence int64  `gorm:"type:bigint"`
}

type CollectedBlock struct {
	ChainId   string          `gorm:"type:text;primaryKey"`
	Height    int64           `gorm:"type:bigint;primaryKey;autoIncrement:false;index:block_height_desc,sort:desc"`
	Hash      string          `gorm:"type:text"`
	Timestamp time.Time       `gorm:"type:timestamptz;index:block_timestamp_desc,sort:desc"`
	BlockTime int64           `gorm:"type:bigint"`
	Proposer  string          `gorm:"type:text"`
	GasUsed   int64           `gorm:"type:bigint"`
	GasWanted int64           `gorm:"type:bigint"`
	TxCount   int             `gorm:"type:bigint;index:block_tx_count"`
	TotalFee  json.RawMessage `gorm:"type:jsonb"`
}

type CollectedTx struct {
	Hash       string          `gorm:"type:text;primaryKey"`
	Height     int64           `gorm:"type:bigint;primaryKey;autoIncrement:false;index:tx_height"`
	Sequence   int64           `gorm:"type:bigint;index:tx_sequence_desc,sort:desc"`
	Signer     string          `gorm:"type:text;index:tx_signer"`
	Data       json.RawMessage `gorm:"type:jsonb"`
	AccountIds pq.Int64Array   `gorm:"type:bigint[];index:tx_account_ids,type:gin"`
	NftIds     pq.Int64Array   `gorm:"type:bigint[];index:tx_nft_ids,type:gin"`
	MsgTypeIds pq.Int64Array   `gorm:"type:bigint[];index:tx_msg_type_ids,type:gin"`
	TypeTagIds pq.Int64Array   `gorm:"type:bigint[];index:tx_type_tag_ids,type:gin"`
}

type CollectedEvmTx struct {
	Hash       string          `gorm:"type:text;primaryKey"`
	Height     int64           `gorm:"type:bigint;primaryKey;autoIncrement:false;index:evm_tx_height"`
	Sequence   int64           `gorm:"type:bigint;index:evm_tx_sequence_desc,sort:desc"`
	Signer     string          `gorm:"type:text;index:evm_tx_signer"`
	Data       json.RawMessage `gorm:"type:jsonb"`
	AccountIds pq.Int64Array   `gorm:"type:bigint[];index:evm_tx_account_ids,type:gin"`
}

type CollectedNftCollection struct {
	Addr       string `gorm:"type:text;primaryKey"`
	Height     int64  `gorm:"type:bigint;index:nft_collection_height"`
	Name       string `gorm:"type:text;index:nft_collection_name"`
	OriginName string `gorm:"type:text;index:nft_collection_origin_name"`
	Creator    string `gorm:"type:text"`
	NftCount   int64  `gorm:"type:bigint"`
}

type CollectedNft struct {
	CollectionAddr string `gorm:"type:text;primaryKey"`
	TokenId        string `gorm:"type:text;primaryKey;index:nft_token_id"`
	Addr           string `gorm:"type:text;index:nft_addr"` // only used in move
	Height         int64  `gorm:"type:bigint;index:nft_height"`
	Owner          string `gorm:"type:text;index:nft_owner"`
	Uri            string `gorm:"type:text"`
}

// only for move
type CollectedFAStore struct {
	StoreAddr string `gorm:"type:text;primaryKey"`
	Owner     string `gorm:"type:text;index:fa_store_owner"`
}

type CollectedAccountDict struct {
	Id      int64  `gorm:"type:bigint;primaryKey"`
	Account string `gorm:"type:text;uniqueIndex:account_dict_account"`
}

type CollectedNftDict struct {
	Id             int64  `gorm:"type:bigint;primaryKey"`
	CollectionAddr string `gorm:"type:text;uniqueIndex:nft_dict_collection_addr_token_id"`
	TokenId        string `gorm:"type:text;uniqueIndex:nft_dict_collection_addr_token_id"`
}

type CollectedMsgTypeDict struct {
	Id      int64  `gorm:"type:bigint;primaryKey"`
	MsgType string `gorm:"type:text;uniqueIndex:msg_type_dict_msg_type"`
}

type CollectedTypeTagDict struct {
	Id      int64  `gorm:"type:bigint;primaryKey"`
	TypeTag string `gorm:"type:text;uniqueIndex:type_tag_dict_type_tag"`
}

type CollectedEvmInternalTx struct {
	Height      int64         `gorm:"type:bigint;primaryKey"`
	Hash        string        `gorm:"type:text;primaryKey"`
	Sequence    int64         `gorm:"type:bigint;index:evm_internal_tx_sequence_desc,sort:desc"`
	ParentIndex int64         `gorm:"type:bigint;index:evm_internal_tx_parent_index"`
	Index       int64         `gorm:"type:bigint;primaryKey;index:evm_internal_tx_index"`
	Type        string        `gorm:"type:text;index:evm_internal_tx_type"`
	From        string        `gorm:"type:text;index:evm_internal_tx_from"`
	To          string        `gorm:"type:text;index:evm_internal_tx_to"`
	Input       string        `gorm:"type:text"`
	Output      string        `gorm:"type:text"`
	Value       string        `gorm:"type:text"`
	Gas         string        `gorm:"type:text"`
	GasUsed     string        `gorm:"type:text"`
	AccountIds  pq.Int64Array `gorm:"type:bigint[];index:evm_internal_tx_account_ids,type:gin"`
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

func (CollectedEvmTx) TableName() string {
	return "evm_tx"
}

func (CollectedNftCollection) TableName() string {
	return "nft_collection"
}

func (CollectedNft) TableName() string {
	return "nft"
}

func (CollectedFAStore) TableName() string {
	return "fa_store"
}

func (CollectedAccountDict) TableName() string {
	return "account_dict"
}

func (CollectedNftDict) TableName() string {
	return "nft_dict"
}

func (CollectedMsgTypeDict) TableName() string {
	return "msg_type_dict"
}

func (CollectedTypeTagDict) TableName() string {
	return "type_tag_dict"
}

func (CollectedEvmInternalTx) TableName() string {
	return "evm_internal_tx"
}
