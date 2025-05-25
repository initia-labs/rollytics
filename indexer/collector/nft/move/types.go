package move

import "strings"

type QueryMoveResourceResponse struct {
	Resource struct {
		Address      string `json:"address"`
		StructTag    string `json:"struct_tag"`
		MoveResource string `json:"move_resource"`
		RawBytes     string `json:"raw_bytes"`
	} `json:"resource"`
}

type NftMintAndBurnEventData struct {
	Collection string `json:"collection"`
	Index      string `json:"index"`
	Nft        string `json:"nft"`
}

type NftTransferEventData struct {
	Object string `json:"object"`
	From   string `json:"from"`
	To     string `json:"to"`
}

type NftMutationEventData struct {
	Nft              string `json:"nft,omitempty"`
	MutatedFieldName string `json:"mutated_field_name"`
	OldValue         string `json:"old_value,omitempty"`
	NewValue         string `json:"new_value,omitempty"`
}

type NftCollectionData struct {
	Type string `json:"type"`
	Data struct {
		Creator     string `json:"creator"`
		Description string `json:"description"`
		Name        string `json:"name"`
		Uri         string `json:"uri"`
		Nfts        struct {
			Handle string `json:"handle"`
			Length string `json:"length"` // it's defined as string in the move contract
		} `json:"nfts"`
	} `json:"data"`
}

type NftData struct {
	Type string `json:"type"`
	Data struct {
		Collection struct {
			Inner string `json:"inner"`
		} `json:"collection"`
		Description string `json:"description"`
		TokenId     string `json:"token_id"`
		Uri         string `json:"uri"`
	}
}

func (nftData *NftData) Trim() {
	if nftData == nil {
		return
	}
	nftData.Data.Collection.Inner = strings.ReplaceAll(nftData.Data.Collection.Inner, "\x00", "")
	nftData.Data.Description = strings.ReplaceAll(nftData.Data.Description, "\x00", "")
	nftData.Data.TokenId = strings.ReplaceAll(nftData.Data.TokenId, "\x00", "")
	nftData.Data.Uri = strings.ReplaceAll(nftData.Data.Uri, "\x00", "")
}
