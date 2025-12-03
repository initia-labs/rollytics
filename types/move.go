package types

type QueryMoveResourceResponse struct {
	Resource struct {
		Address      string `json:"address"`
		StructTag    string `json:"struct_tag"`
		MoveResource string `json:"move_resource"`
		RawBytes     string `json:"raw_bytes"`
	} `json:"resource"`
}
