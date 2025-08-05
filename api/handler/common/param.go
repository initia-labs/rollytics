package common

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/util"
)

func GetParams(c *fiber.Ctx, key string) (string, error) {
	value := c.Params(key)
	if value == "" {
		return "", fmt.Errorf("missing parameter: %s", key)
	}
	return value, nil
}

func GetHeightParam(c *fiber.Ctx) (int64, error) {
	value, err := GetParams(c, "height")
	if err != nil {
		return 0, err
	}

	intValue, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid height: %s", err.Error())
	}

	if intValue < 1 {
		return 0, errors.New("height must be positive integer")
	}

	return intValue, nil
}

func GetAccountParam(c *fiber.Ctx) (string, error) {
	account, err := GetParams(c, "account")
	if err != nil {
		return "", err
	}

	accAddr, err := util.AccAddressFromString(account)
	if err != nil {
		return "", fmt.Errorf("invalid account: %s", err.Error())
	}
	return accAddr.String(), nil
}

func GetCollectionAddrParam(c *fiber.Ctx, config *config.ChainConfig) ([]byte, error) {
	collectionAddr, err := GetParams(c, "collection_addr")
	if err != nil {
		return nil, err
	}

	if err := validateCollectionAddr(collectionAddr, config); err != nil {
		return nil, err
	}

	return util.HexToBytes(strings.ToLower(collectionAddr))
}

func GetMsgsQuery(c *fiber.Ctx) (msgs []string) {
	raw := c.Request().URI().QueryArgs().PeekMulti("msgs")
	for _, bytes := range raw {
		msgs = append(msgs, string(bytes))
	}
	return msgs
}

func GetCollectionAddrQuery(c *fiber.Ctx, config *config.ChainConfig) ([]byte, error) {
	collectionAddr := c.Query("collection_addr")
	if collectionAddr == "" {
		return nil, nil
	}

	if err := validateCollectionAddr(collectionAddr, config); err != nil {
		return nil, err
	}

	return util.HexToBytes(strings.ToLower(collectionAddr))
}
