package internaltx

import (
	"context"
	"fmt"

	"golang.org/x/mod/semver"

	"github.com/initia-labs/rollytics/util/querier"
)

const (
	EnableNodeVersion = "v1.1.0"
)

func CheckNodeVersion(ctx context.Context, querier *querier.Querier) error {
	nodeInfo, err := querier.GetNodeInfo(ctx)
	if err != nil {
		return err
	}

	// TODO: if it not compatible with semver, should rotate the endpoint
	// check version higher than minimum required version
	nodeVersion := nodeInfo.AppVersion.Version
	if !semver.IsValid(nodeVersion) {
		nodeVersion = "v" + nodeVersion
	}

	if semver.Compare(nodeVersion, EnableNodeVersion) < 0 {
		return fmt.Errorf("node version %s is lower than required version %s", nodeVersion, EnableNodeVersion)
	}

	return nil
}
