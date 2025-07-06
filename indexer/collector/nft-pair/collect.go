package nft_pair

import (
	"encoding/base64"
	"encoding/json"

	ibcnfttypes "github.com/initia-labs/initia/x/ibc/nft-transfer/types"
	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/config"
	indexertypes "github.com/initia-labs/rollytics/indexer/types"
	"github.com/initia-labs/rollytics/indexer/util"
	"github.com/initia-labs/rollytics/types"
)

func Collect(block indexertypes.ScrapedBlock, cfg *config.Config, tx *gorm.DB) (err error) {
	collectionPairMap := make(map[string]string) // l2 collection name -> l1 collection name
	events, err := util.ExtractEvents(block, "recv_packet")
	if err != nil {
		return err
	}

	for _, event := range events {
		packetSrcPort, found := event.AttrMap["packet_src_port"]
		if !found || packetSrcPort != "nft-transfer" {
			continue
		}
		packetDstPort, found := event.AttrMap["packet_dst_port"]
		if !found {
			continue
		}
		packetDstChannel, found := event.AttrMap["packet_dst_channel"]
		if !found {
			continue
		}
		packetDataRaw, found := event.AttrMap["packet_data"]
		if !found {
			continue
		}

		var packetData PacketData
		if err := json.Unmarshal([]byte(packetDataRaw), &packetData); err != nil {
			return err
		}
		classDataRaw, err := base64.StdEncoding.DecodeString(packetData.ClassData)
		if err != nil {
			return err
		}
		var classData NftClassData
		if err := json.Unmarshal(classDataRaw, &classData); err != nil {
			return err
		}

		l1CollectionName := classData.Name
		var l2CollectionName string
		if cfg.GetVmType() == types.WasmVM {
			l2CollectionName = ibcnfttypes.GetPrefixedClassId(packetDstPort, packetDstChannel, packetData.ClassId)
		} else {
			l2CollectionName = ibcnfttypes.GetNftTransferClassId(packetDstPort, packetDstChannel, packetData.ClassId)
		}

		collectionPairMap[l2CollectionName] = l1CollectionName
	}

	for l2CollectionName, l1CollectionName := range collectionPairMap {
		if err := tx.Model(&types.CollectedNftCollection{}).
			Where("chain_id = ? AND name = ?", block.ChainId, l2CollectionName).
			Updates(map[string]interface{}{"origin_name": l1CollectionName}).Error; err != nil {
			return err
		}
	}

	return nil
}
