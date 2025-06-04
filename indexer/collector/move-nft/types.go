package move_nft

import "strings"

type CacheData struct {
	ColResources map[string]string // collection addr -> collection resource
	NftResources map[string]string // nft addr -> nft resource
}

type QueryMoveResourceResponse struct {
	Resource struct {
		Address      string `json:"address"`
		StructTag    string `json:"struct_tag"`
		MoveResource string `json:"move_resource"`
		RawBytes     string `json:"raw_bytes"`
	} `json:"resource"`
}

type NftMintAndBurnEvent struct {
	Collection string `json:"collection"`
	Index      string `json:"index"`
	Nft        string `json:"nft"`
}

type NftTransferEvent struct {
	Object string `json:"object"`
	From   string `json:"from"`
	To     string `json:"to"`
}

type NftMutationEvent struct {
	Nft              string `json:"nft"`
	MutatedFieldName string `json:"mutated_field_name"`
	OldValue         string `json:"old_value"`
	NewValue         string `json:"new_value"`
}

type CollectionResource struct {
	Type string `json:"type"`
	Data struct {
		Creator     string `json:"creator"`
		Description string `json:"description"`
		Name        string `json:"name"`
		Uri         string `json:"uri"`
		Nfts        struct {
			Handle string `json:"handle"`
			Length string `json:"length"`
		} `json:"nfts"`
	} `json:"data"`
}

type NftResource struct {
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

func (resource *NftResource) Trim() {
	if resource == nil {
		return
	}

	resource.Data.Collection.Inner = strings.ReplaceAll(resource.Data.Collection.Inner, "\x00", "")
	resource.Data.Description = strings.ReplaceAll(resource.Data.Description, "\x00", "")
	resource.Data.TokenId = strings.ReplaceAll(resource.Data.TokenId, "\x00", "")
	resource.Data.Uri = strings.ReplaceAll(resource.Data.Uri, "\x00", "")
}
