package fa

type PrimaryStoreCreatedEvent struct {
	OwnerAddr    string `json:"owner_addr"`
	StoreAddr    string `json:"store_addr"`
	MetadataAddr string `json:"metadata_addr"`
}
