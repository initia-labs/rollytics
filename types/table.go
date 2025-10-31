package types

import (
	"encoding/json"
	"time"
)

// Fast count optimization types (duplicated to avoid circular import)
type CountOptimizationType int

const (
	CountOptimizationTypeMax     CountOptimizationType = iota + 1 // MAX(field) for sequential fields
	CountOptimizationTypePgClass                                  // pg_class.reltuples for table stats
	CountOptimizationTypeCount                                    // Regular COUNT (fallback)
)

// FastCountStrategy defines how each table can optimize COUNT operations
type FastCountStrategy interface {
	TableName() string
	GetOptimizationType() CountOptimizationType
	GetOptimizationField() string // field to use for MAX optimization
	SupportsFastCount() bool
}

type Table struct {
	Model any
	Name  string
}

type CollectedUpgradeHistory struct {
	Version string    `gorm:"type:text;primaryKey"`
	Applied time.Time `gorm:"type:timestamptz"`
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
	Hash     []byte          `gorm:"type:bytea;primaryKey"`
	Height   int64           `gorm:"type:bigint;primaryKey;autoIncrement:false;index:tx_height"`
	Sequence int64           `gorm:"type:bigint;index:tx_sequence_desc,sort:desc;index:tx_account_sequence_partial,sort:desc;index:tx_nft_sequence_partial,sort:desc;index:tx_msg_type_sequence_partial,sort:desc"`
	SignerId int64           `gorm:"type:bigint;index:tx_signer_id"`
	Data     json.RawMessage `gorm:"type:jsonb"`
}

type CollectedTxAccount struct {
	AccountId int64 `gorm:"type:bigint;primaryKey"`
	Sequence  int64 `gorm:"type:bigint;primaryKey"`
	Signer    bool  `gorm:"type:boolean"`
}

type CollectedTxNft struct {
	NftId    int64 `gorm:"type:bigint;primaryKey"`
	Sequence int64 `gorm:"type:bigint;primaryKey"`
}

type CollectedTxMsgType struct {
	MsgTypeId int64 `gorm:"type:bigint;primaryKey"`
	Sequence  int64 `gorm:"type:bigint;primaryKey"`
}

type CollectedTxTypeTag struct {
	TypeTagId int64 `gorm:"type:bigint;primaryKey"`
	Sequence  int64 `gorm:"type:bigint;primaryKey"`
}

type CollectedEvmTxAccount struct {
	AccountId int64 `gorm:"type:bigint;primaryKey"`
	Sequence  int64 `gorm:"type:bigint;primaryKey"`
	Signer    bool  `gorm:"type:boolean"`
}

type CollectedEvmInternalTxAccount struct {
	AccountId int64 `gorm:"type:bigint;primaryKey"`
	Sequence  int64 `gorm:"type:bigint;primaryKey"`
}

type CollectedEvmTx struct {
	Hash     []byte          `gorm:"type:bytea;primaryKey"`
	Height   int64           `gorm:"type:bigint;primaryKey;autoIncrement:false;index:evm_tx_height"`
	Sequence int64           `gorm:"type:bigint;index:evm_tx_sequence_desc,sort:desc;index:evm_tx_account_sequence_partial,sort:desc"`
	SignerId int64           `gorm:"type:bigint;index:evm_tx_signer_id"`
	Data     json.RawMessage `gorm:"type:jsonb"`
}

type CollectedNftCollection struct {
	Addr       []byte    `gorm:"type:bytea;primaryKey"` // hex address
	Height     int64     `gorm:"type:bigint;index:nft_collection_height"`
	Timestamp  time.Time `gorm:"type:timestamptz"`
	Name       string    `gorm:"type:text;index:nft_collection_name"`
	OriginName string    `gorm:"type:text;index:nft_collection_origin_name"`
	CreatorId  int64     `gorm:"type:bigint;index:nft_collection_creator_id"`
	NftCount   int64     `gorm:"type:bigint"`
}

type CollectedNft struct {
	CollectionAddr []byte    `gorm:"type:bytea;primaryKey"`
	TokenId        string    `gorm:"type:text;primaryKey;index:nft_token_id;index:nft_token_id_height,priority:1;index:nft_height_token_id,priority:2"`
	Addr           []byte    `gorm:"type:bytea;index:nft_addr,type:hash"` // only used in move // hex address
	Height         int64     `gorm:"type:bigint;index:nft_height;index:nft_token_id_height,priority:2;index:nft_height_token_id,priority:1"`
	Timestamp      time.Time `gorm:"type:timestamptz"`
	OwnerId        int64     `gorm:"type:bigint;index:nft_owner_id"`
	Uri            string    `gorm:"type:text"`
}

// only for move
type CollectedFAStore struct {
	StoreAddr []byte `gorm:"type:bytea;primaryKey"`
	Owner     []byte `gorm:"type:bytea;index:fa_store_owner,type:hash"`
}

type CollectedAccountDict struct {
	Id      int64  `gorm:"type:bigint;primaryKey"`
	Account []byte `gorm:"type:bytea;uniqueIndex:account_dict_account"`
}

type CollectedNftDict struct {
	Id             int64  `gorm:"type:bigint;primaryKey"`
	CollectionAddr []byte `gorm:"type:bytea;uniqueIndex:nft_dict_collection_addr_token_id"` // hex address bytes
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
	Height      int64  `gorm:"type:bigint;primaryKey;index:evm_internal_tx_height_sequence_desc,priority:1"`
	HashId      int64  `gorm:"type:bigint;primaryKey;index:evm_internal_tx_hash_sequence_desc,priority:1"` // use hash id from evm_tx_hash_dict
	Index       int64  `gorm:"type:bigint;primaryKey;index:evm_internal_tx_index"`
	ParentIndex int64  `gorm:"type:bigint;index:evm_internal_tx_parent_index"`
	Sequence    int64  `gorm:"type:bigint;index:evm_internal_tx_sequence_desc,sort:desc;index:evm_internal_tx_account_sequence_partial,sort:desc;index:evm_internal_tx_hash_sequence_desc,priority:2,sort:desc;index:evm_internal_tx_height_sequence_desc,priority:2,sort:desc"`
	Type        string `gorm:"type:text;index:evm_internal_tx_type"`
	FromId      int64  `gorm:"type:bigint;index:evm_internal_tx_from_id"`
	ToId        int64  `gorm:"type:bigint;index:evm_internal_tx_to_id"`
	Input       []byte `gorm:"type:bytea"`
	Output      []byte `gorm:"type:bytea"`
	Value       []byte `gorm:"type:bytea"`
	Gas         []byte `gorm:"type:bytea"`
	GasUsed     []byte `gorm:"type:bytea"`
}

type CollectedEvmTxHashDict struct {
	Id   int64  `gorm:"type:bigint;primaryKey"`
	Hash []byte `gorm:"type:bytea;uniqueIndex:evm_tx_hash_dict_hash"`
}

func (CollectedUpgradeHistory) TableName() string {
	return "upgrade_history"
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

func (CollectedTxAccount) TableName() string {
	return "tx_accounts"
}

func (CollectedTxNft) TableName() string {
	return "tx_nfts"
}

func (CollectedTxMsgType) TableName() string {
	return "tx_msg_types"
}

func (CollectedTxTypeTag) TableName() string {
	return "tx_type_tags"
}

func (CollectedEvmTxAccount) TableName() string {
	return "evm_tx_accounts"
}

func (CollectedEvmInternalTxAccount) TableName() string {
	return "evm_internal_tx_accounts"
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

// CursorRecord interface implementations

// Sequence-based tables
func (t CollectedTx) GetCursorFields() []string {
	return []string{"sequence"}
}

func (t CollectedTx) GetCursorValue(field string) any {
	switch field {
	case "sequence":
		return t.Sequence
	default:
		return nil
	}
}

func (t CollectedTx) GetCursorData() map[string]any {
	return map[string]any{
		"sequence": t.Sequence,
	}
}

func (t CollectedEvmTx) GetCursorFields() []string {
	return []string{"sequence"}
}

func (t CollectedEvmTx) GetCursorValue(field string) any {
	switch field {
	case "sequence":
		return t.Sequence
	default:
		return nil
	}
}

func (t CollectedEvmTx) GetCursorData() map[string]any {
	return map[string]any{
		"sequence": t.Sequence,
	}
}

func (t CollectedEvmInternalTx) GetCursorFields() []string {
	return []string{"sequence"}
}

func (t CollectedEvmInternalTx) GetCursorValue(field string) any {
	switch field {
	case "sequence":
		return t.Sequence
	default:
		return nil
	}
}

func (t CollectedEvmInternalTx) GetCursorData() map[string]any {
	return map[string]any{
		"sequence": t.Sequence,
	}
}

// Height-based tables
func (b CollectedBlock) GetCursorFields() []string {
	return []string{"height"}
}

func (b CollectedBlock) GetCursorValue(field string) any {
	switch field {
	case "height":
		return b.Height
	default:
		return nil
	}
}

func (b CollectedBlock) GetCursorData() map[string]any {
	return map[string]any{
		"height": b.Height,
	}
}

func (c CollectedNftCollection) GetCursorFields() []string {
	return []string{"height"}
}

func (c CollectedNftCollection) GetCursorValue(field string) any {
	switch field {
	case "height":
		return c.Height
	default:
		return nil
	}
}

func (c CollectedNftCollection) GetCursorData() map[string]any {
	return map[string]any{
		"height": c.Height,
	}
}

// Composite cursor (height + token_id)
func (n CollectedNft) GetCursorFields() []string {
	return []string{"height", "token_id"}
}

func (n CollectedNft) GetCursorValue(field string) any {
	switch field {
	case "height":
		return n.Height
	case "token_id":
		return n.TokenId
	default:
		return nil
	}
}

func (n CollectedNft) GetCursorData() map[string]any {
	return map[string]any{
		"height":   n.Height,
		"token_id": n.TokenId,
	}
}

// FastCountStrategy implementations for each table

// TX tables - use MAX(sequence) for fast counting
func (t CollectedTx) GetOptimizationType() CountOptimizationType { return CountOptimizationTypeMax }
func (t CollectedTx) GetOptimizationField() string               { return "sequence" }
func (t CollectedTx) SupportsFastCount() bool                    { return true }

func (t CollectedEvmTx) GetOptimizationType() CountOptimizationType { return CountOptimizationTypeMax }
func (t CollectedEvmTx) GetOptimizationField() string               { return "sequence" }
func (t CollectedEvmTx) SupportsFastCount() bool                    { return true }

func (t CollectedEvmInternalTx) GetOptimizationType() CountOptimizationType {
	return CountOptimizationTypeMax
}
func (t CollectedEvmInternalTx) GetOptimizationField() string { return "sequence" }
func (t CollectedEvmInternalTx) SupportsFastCount() bool      { return true }

// Block table - use MAX(height)
func (b CollectedBlock) GetOptimizationType() CountOptimizationType { return CountOptimizationTypeMax }
func (b CollectedBlock) GetOptimizationField() string               { return "height" }
func (b CollectedBlock) SupportsFastCount() bool                    { return true }

// NFT Collection - use PostgreSQL statistics
func (c CollectedNftCollection) GetOptimizationType() CountOptimizationType {
	return CountOptimizationTypePgClass
}
func (c CollectedNftCollection) GetOptimizationField() string { return "" } // not used for pg_class
func (c CollectedNftCollection) SupportsFastCount() bool      { return true }

// NFT tokens - use PostgreSQL statistics (large table)
func (n CollectedNft) GetOptimizationType() CountOptimizationType {
	return CountOptimizationTypePgClass
}
func (n CollectedNft) GetOptimizationField() string { return "" } // not used for pg_class
func (n CollectedNft) SupportsFastCount() bool      { return true }

// TX edge tables - use MAX(sequence) for fast counting
func (t CollectedTxMsgType) GetOptimizationType() CountOptimizationType {
	return CountOptimizationTypeMax
}
func (t CollectedTxMsgType) GetOptimizationField() string { return "sequence" }
func (t CollectedTxMsgType) SupportsFastCount() bool      { return true }
