package logx

import (
	"io"
	"log"
	"os"
	"time"

	gormlogger "gorm.io/gorm/logger"
)

// NewGormLogger 创建 GORM 日志器。
//
// 约束：
// 1. SQL 错误日志写入按日滚动文件；
// 2. 正常 record not found 不输出；
// 3. 不向控制台刷屏。
func NewGormLogger() gormlogger.Interface {
	var writer io.Writer = os.Stderr
	if fileWriter, err := newDailyFileWriteSyncer("log"); err == nil {
		writer = fileWriter
	}
	return gormlogger.New(log.New(writer, "", 0), gormlogger.Config{
		SlowThreshold:             500 * time.Millisecond,
		LogLevel:                  gormlogger.Error,
		IgnoreRecordNotFoundError: true,
		ParameterizedQueries:      true,
		Colorful:                  false,
	})
}
