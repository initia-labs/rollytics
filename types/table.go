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
	Hash      []byte          `gorm:"type:bytea"`
	Timestamp time.Time       `gorm:"type:timestamptz;index:block_timestamp_desc,sort:desc"`
	BlockTime int64           `gorm:"type:bigint"`
	Proposer  string          `gorm:"type:text"`
	GasUsed   int64           `gorm:"type:bigint"`
	GasWanted int64           `gorm:"type:bigint"`
	TxCount   int             `gorm:"type:smallint;index:block_tx_count"`
	TotalFee  json.RawMessage `gorm:"type:jsonb"`
}

type CollectedTx struct {
	Hash       []byte          `gorm:"type:bytea;primaryKey"`
	Height     int64           `gorm:"type:bigint;primaryKey;autoIncrement:false;index:tx_height"`
	Sequence   int64           `gorm:"type:bigint;index:tx_sequence_desc,sort:desc"`
	SignerId   int64           `gorm:"type:bigint;index:tx_signer_id"`
	Data       json.RawMessage `gorm:"type:jsonb"`
	AccountIds pq.Int64Array   `gorm:"type:bigint[];index:tx_account_ids,type:gin"`
	NftIds     pq.Int64Array   `gorm:"type:bigint[];index:tx_nft_ids,type:gin"`
	MsgTypeIds pq.Int64Array   `gorm:"type:bigint[];index:tx_msg_type_ids,type:gin"`
	TypeTagIds pq.Int64Array   `gorm:"type:bigint[];index:tx_type_tag_ids,type:gin"`
}

type CollectedEvmTx struct {
	Hash       []byte          `gorm:"type:bytea;primaryKey"`
	Height     int64           `gorm:"type:bigint;primaryKey;autoIncrement:false;index:evm_tx_height"`
	Sequence   int64           `gorm:"type:bigint;index:evm_tx_sequence_desc,sort:desc"`
	SignerId   int64           `gorm:"type:bigint;index:evm_tx_signer_id"`
	Data       json.RawMessage `gorm:"type:jsonb"`
	AccountIds pq.Int64Array   `gorm:"type:bigint[];index:evm_tx_account_ids,type:gin"`
}

type CollectedNftCollection struct {
	Addr       []byte `gorm:"type:bytea;primaryKey"` // hex address
	Height     int64  `gorm:"type:bigint;index:nft_collection_height"`
	Name       string `gorm:"type:text;index:nft_collection_name"`
	OriginName string `gorm:"type:text;index:nft_collection_origin_name"`
	CreatorId  int64  `gorm:"type:bigint;index:nft_collection_creator_id"`
	NftCount   int64  `gorm:"type:bigint"`
}

type CollectedNft struct {
	CollectionAddr []byte `gorm:"type:bytea;primaryKey"` // hex address
	TokenId        string `gorm:"type:text;primaryKey;index:nft_token_id"`
	Addr           []byte `gorm:"type:bytea;index:nft_addr,type:hash"` // only used in move // hex address
	Height         int64  `gorm:"type:bigint;index:nft_height"`
	OwnerId        int64  `gorm:"type:bigint;index:nft_owner_id"`
	Uri            string `gorm:"type:text"`
}

// only for move
type CollectedFAStore struct {
	StoreAddr []byte `gorm:"type:bytea;primaryKey"`                     // hex address
	Owner     []byte `gorm:"type:bytea;index:fa_store_owner,type:hash"` // hex address
}

type CollectedAccountDict struct {
	Id      int64  `gorm:"type:bigint;primaryKey"`
	Account []byte `gorm:"type:bytea;uniqueIndex:account_dict_account"` // acc address
}

type CollectedNftDict struct {
	Id             int64  `gorm:"type:bigint;primaryKey"`
	CollectionAddr []byte `gorm:"type:bytea;uniqueIndex:nft_dict_collection_addr_token_id"`
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

// Extension: Table related to internal transaction
type CollectedEvmInternalTx struct {
	Height      int64         `gorm:"type:bigint;primaryKey"`
	HashId      int64         `gorm:"type:bigint;primaryKey"` // use hash id from evm_tx_hash_dict
	Index       int64         `gorm:"type:bigint;primaryKey;index:evm_internal_tx_index"`
	ParentIndex int64         `gorm:"type:bigint;index:evm_internal_tx_parent_index"`
	Sequence    int64         `gorm:"type:bigint;index:evm_internal_tx_sequence_desc,sort:desc"`
	Type        string        `gorm:"type:text;index:evm_internal_tx_type"`
	FromId      int64         `gorm:"type:bigint;index:evm_internal_tx_from_id"`
	ToId        int64         `gorm:"type:bigint;index:evm_internal_tx_to_id"`
	Input       []byte        `gorm:"type:bytea"`
	Output      []byte        `gorm:"type:bytea"`
	Value       []byte        `gorm:"type:bytea"`
	Gas         []byte        `gorm:"type:bytea"`
	GasUsed     []byte        `gorm:"type:bytea"`
	AccountIds  pq.Int64Array `gorm:"type:bigint[];index:evm_internal_tx_account_ids,type:gin"`
}

type CollectedEvmTxHashDict struct {
	Id   int64  `gorm:"type:bigint;primaryKey"`
	Hash []byte `gorm:"type:bytea;uniqueIndex:evm_tx_hash_dict_hash"`
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

func (CollectedEvmInternalTx) TableName() string {
	return "evm_internal_tx"
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

func (CollectedEvmTxHashDict) TableName() string {
	return "evm_tx_hash_dict"
}
