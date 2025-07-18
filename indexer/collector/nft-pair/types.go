package nft_pair

type PacketData struct {
	ClassData string `json:"classData"`
	ClassId   string `json:"classId"`
}

type NftClassData struct {
	Description string `json:"description"`
	Name        string `json:"name"`
}
