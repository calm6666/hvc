package idgen

import (
	"context"
	"fmt"
	"time"

	snowflake "github.com/crosscode-nl/snowflake"
)

const defaultMaxJSIntegerSafe uint64 = 9007199254740991

var (
	generator        *snowflake.Generator
	maxJSIntegerSafe uint64 = defaultMaxJSIntegerSafe
)

// Configure 配置雪花 ID 生成器。
func Configure(start time.Time, configuredNodeID uint64, configuredNodeBits uint8, configuredSequenceBits uint8, configuredMaxJSIntegerSafe uint64) {
	if configuredMaxJSIntegerSafe > 0 {
		maxJSIntegerSafe = configuredMaxJSIntegerSafe
	}
	options := []snowflake.Option{snowflake.WithEpoch(start)}
	if configuredNodeBits > 0 {
		options = append(options, snowflake.WithMachineIDBits(uint64(configuredNodeBits)))
	}
	gen, err := snowflake.NewGenerator(configuredNodeID, options...)
	if err != nil {
		panic(err)
	}
	generator = gen
}

// Next 返回一个雪花 ID。
func Next() uint64 {
	if generator == nil {
		panic("snowflake generator is not configured")
	}
	id, err := generator.BlockingNextID(context.Background())
	if err != nil {
		panic(err)
	}
	value := uint64(id)
	if value > maxJSIntegerSafe {
		panic(fmt.Sprintf("generated id %d exceeds max safe js integer %d", value, maxJSIntegerSafe))
	}
	return value
}
