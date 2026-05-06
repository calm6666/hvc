package idgen

import (
	"sync"
	"time"

	snowflake "github.com/zlabwork/snowflake"
)

var (
	mu   sync.RWMutex
	node *snowflake.Node
)

// Configure 配置雪花 ID 生成器。
func Configure(start time.Time, configuredNodeID uint64, configuredNodeBits uint8, configuredSequenceBits uint8) {
	mu.Lock()
	defer mu.Unlock()
	snowflake.Epoch = start.UnixMilli()
	if configuredNodeBits > 0 {
		snowflake.NodeBits = configuredNodeBits
	}
	if configuredSequenceBits > 0 {
		snowflake.StepBits = configuredSequenceBits
	}
	generated, err := snowflake.NewNode(int64(configuredNodeID))
	if err != nil {
		panic(err)
	}
	node = generated
}

// Next 返回一个雪花 ID。
func Next() uint64 {
	mu.RLock()
	defer mu.RUnlock()
	if node == nil {
		panic("snowflake node is not configured")
	}
	return uint64(node.Generate())
}
