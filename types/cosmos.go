package types

type NodeInfo struct {
	AppVersion struct {
		Version string `json:"version"`
	} `json:"application_version"`
}
