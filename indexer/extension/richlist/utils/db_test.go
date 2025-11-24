package utils

import (
	"context"
	"testing"
	"time"

	"github.com/initia-labs/rollytics/types"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	assert.NoError(t, err)

	err = db.AutoMigrate(&types.CollectedBlock{})
	assert.NoError(t, err)

	return db
}

func TestGetCollectedBlock_BlockDelay(t *testing.T) {
	db := setupTestDB(t)
	chainId := "test-chain"

	// Insert blocks
	// We want to test fetching block 100.
	// BlockDelay is 3.
	// So we need latest height > 100 + 3 => latest >= 104.

	// Case 1: Latest height is 102. Block 100 is NOT ready. (102 - 100 = 2 < 3)
	// We expect it to retry until context timeout or until new block appears.
	// Here we just test that it doesn't return immediately with success if we only have up to 102.

	blocks := []types.CollectedBlock{
		{ChainId: chainId, Height: 100, Hash: []byte("hash100"), Timestamp: time.Time{}},
		{ChainId: chainId, Height: 101, Hash: []byte("hash101"), Timestamp: time.Time{}},
		{ChainId: chainId, Height: 102, Hash: []byte("hash102"), Timestamp: time.Time{}},
	}
	for _, b := range blocks {
		err := db.Create(&b).Error
		assert.NoError(t, err)
	}

	// Use a short timeout to verify it blocks/retries
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	start := time.Now()
	block, err := GetCollectedBlock(ctx, db, chainId, 100)
	duration := time.Since(start)

	// It should fail with context deadline exceeded because it keeps retrying
	assert.Error(t, err)
	if err != nil {
		assert.Contains(t, err.Error(), "context deadline exceeded")
	}
	assert.Nil(t, block)
	assert.True(t, duration >= 200*time.Millisecond, "Should have waited until timeout")

	// Case 2: Insert block 104. Now 104 - 100 = 4 > 3. Should succeed.
	err = db.Create(&types.CollectedBlock{ChainId: chainId, Height: 100 + RICH_LIST_BLOCK_DELAY + 1, Hash: []byte("hash104"), Timestamp: time.Time{}}).Error
	assert.NoError(t, err)

	ctx2, cancel2 := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel2()

	block, err = GetCollectedBlock(ctx2, db, chainId, 100)
	assert.NoError(t, err)
	if assert.NotNil(t, block) {
		assert.Equal(t, int64(100), block.Height)
	}
}
