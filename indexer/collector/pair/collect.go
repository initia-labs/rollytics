package pair

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	abci "github.com/cometbft/cometbft/abci/types"
	ibcnfttypes "github.com/initia-labs/initia/x/ibc/nft-transfer/types"
	"github.com/initia-labs/rollytics/indexer/config"
	indexertypes "github.com/initia-labs/rollytics/indexer/types"
	"github.com/initia-labs/rollytics/types"
	"gorm.io/gorm"
)

func Collect(block indexertypes.ScrappedBlock, cfg *config.Config, tx *gorm.DB) (err error) {
	collectionPairMap := make(map[string]string) // l2 collection name -> l1 collection name

	for _, event := range extractEvents(block) {
		if event.Type != "recv_packet" {
			continue
		}

		packetSrcPort, found := event.Attributes["packet_src_port"]
		if !found || packetSrcPort != "nft_transfer" {
			continue
		}
		packetDstPort, found := event.Attributes["packet_dst_port"]
		if !found {
			continue
		}
		packetDstChannel, found := event.Attributes["packet_dst_channel"]
		if !found {
			continue
		}

		packetDataRaw, found := event.Attributes["packet_data"]
		if !found {
			continue
		}
		var packetData PacketData
		if err := json.Unmarshal([]byte(packetDataRaw), &packetData); err != nil {
			return err
		}
		baseClassId := packetData.ClassId

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
		if cfg.GetChainConfig().VmType == types.WasmVM {
			l2CollectionName = fmt.Sprintf("%s/%s/%s", packetDstPort, packetDstChannel, baseClassId)
		} else {
			l2CollectionName = ibcnfttypes.GetNftTransferClassId(packetDstPort, packetDstChannel, baseClassId)
		}

		collectionPairMap[l2CollectionName] = l1CollectionName
	}

	for l2CollectionName, l1CollectionName := range collectionPairMap {
		if res := tx.Model(&types.CollectedNftCollection{}).Where("chain_id = ? AND name = ?", block.ChainId, l2CollectionName).Updates(map[string]interface{}{"origin_name": l1CollectionName}); res.Error != nil {
			return res.Error
		}
	}

	return nil
}

func extractEvents(block indexertypes.ScrappedBlock) []indexertypes.ParsedEvent {
	events := parseEvents(block.BeginBlock)

	for _, res := range block.TxResults {
		events = append(events, parseEvents(res.Events)...)
	}

	events = append(events, parseEvents(block.EndBlock)...)

	return events
}

func parseEvents(evts []abci.Event) (parsedEvts []indexertypes.ParsedEvent) {
	for _, evt := range evts {
		parsedEvts = append(parsedEvts, parseEvent(evt))
	}

	return
}

func parseEvent(evt abci.Event) indexertypes.ParsedEvent {
	attributes := make(map[string]string)
	for _, attr := range evt.Attributes {
		attributes[attr.Key] = attr.Value
	}
	return indexertypes.ParsedEvent{
		Type:       evt.Type,
		Attributes: attributes,
	}
}
