package types

import "encoding/json"

const (
	MoveMetadataTypeTag           = "0x1::fungible_asset::Metadata"
	MoveDepositEventTypeTag       = "0x1::fungible_asset::DepositEvent"
	MoveDepositOwnerEventTypeTag  = "0x1::fungible_asset::DepositOwnerEvent"
	MoveWithdrawEventTypeTag      = "0x1::fungible_asset::WithdrawEvent"
	MoveWithdrawOwnerEventTypeTag = "0x1::fungible_asset::WithdrawOwnerEvent"
)

type QueryMoveResourceResponse struct {
	Resource struct {
		Address      string `json:"address"`
		StructTag    string `json:"struct_tag"`
		MoveResource string `json:"move_resource"`
		RawBytes     string `json:"raw_bytes"`
	} `json:"resource"`
}

type MoveResource struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

type MoveFungibleAssetMetadata struct {
	Decimals   uint8  `json:"decimals"`
	IconUri    string `json:"icon_uri"`
	Name       string `json:"name"`
	ProjectUri string `json:"project_uri"`
	Symbol     string `json:"symbol"`
}
