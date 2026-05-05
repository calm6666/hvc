package logx

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Fields 表示日志字段。
type Fields map[string]any

type entry struct {
	level  zapcore.Level
	msg    string
	fields []zap.Field
}

var (
	logger  *zap.Logger
	queue   chan entry
	once    sync.Once
	dropped atomic.Uint64
	stdlog  = log.New(os.Stdout, "", 0)
)

// Init 初始化日志组件。
func Init(serviceName string) {
	once.Do(func() {
		encoderConfig := zap.NewProductionEncoderConfig()
		encoderConfig.TimeKey = "time"
		encoderConfig.MessageKey = "message"
		encoderConfig.LevelKey = "level"
		encoderConfig.CallerKey = "caller"
		encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		encoderConfig.EncodeLevel = zapcore.LowercaseLevelEncoder
		core := zapcore.NewCore(
			zapcore.NewJSONEncoder(encoderConfig),
			zapcore.AddSync(os.Stdout),
			zap.InfoLevel,
		)
		logger = zap.New(core, zap.AddCaller(), zap.Fields(zap.String("service", serviceName)))
		queue = make(chan entry, 8192)
		go consume()
	})
}

// Sync 刷新日志。
func Sync() {
	if logger != nil {
		_ = logger.Sync()
	}
}

// Info 输出信息日志。
func Info(action string, fields Fields) {
	enqueue(zapcore.InfoLevel, action, toZapFields(fields))
}

// Error 输出错误日志。
func Error(action string, err error, fields Fields) {
	zapFields := toZapFields(fields)
	if err != nil {
		zapFields = append(zapFields, zap.String("error", err.Error()))
	}
	enqueue(zapcore.ErrorLevel, action, zapFields)
}

// Middleware 返回标准 net/http 日志中间件。
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		requestBody, requestTruncated := readBody(r.Body)
		if r.Body != nil {
			r.Body = io.NopCloser(bytes.NewBuffer(requestBody))
		}
		writer := &bodyWriter{ResponseWriter: w, body: bytes.NewBuffer(nil), statusCode: http.StatusOK}
		next.ServeHTTP(writer, r)
		responseBody := writer.body.Bytes()
		responseTruncated := false
		if len(responseBody) > 65536 {
			responseBody = responseBody[:65536]
			responseTruncated = true
		}
		Info("http.request", Fields{
			"method":             r.Method,
			"path":               r.URL.Path,
			"query":              r.URL.RawQuery,
			"status":             writer.statusCode,
			"latency_ms":         time.Since(start).Milliseconds(),
			"client_ip":          r.RemoteAddr,
			"user_agent":         r.UserAgent(),
			"request_body":       string(requestBody),
			"request_truncated":  requestTruncated,
			"response_body":      string(responseBody),
			"response_truncated": responseTruncated,
		})
		if droppedValue := dropped.Load(); droppedValue > 0 {
			stdlog.Println("log_queue_dropped", droppedValue)
			dropped.Store(0)
		}
	})
}

type bodyWriter struct {
	http.ResponseWriter
	body       *bytes.Buffer
	statusCode int
}

func (w *bodyWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *bodyWriter) Write(data []byte) (int, error) {
	w.body.Write(data)
	return w.ResponseWriter.Write(data)
}

func consume() {
	for item := range queue {
		switch item.level {
		case zapcore.ErrorLevel:
			logger.Error(item.msg, item.fields...)
		default:
			logger.Info(item.msg, item.fields...)
		}
	}
}

func enqueue(level zapcore.Level, msg string, fields []zap.Field) {
	if logger == nil || queue == nil {
		return
	}
	select {
	case queue <- entry{level: level, msg: msg, fields: fields}:
	default:
		dropped.Add(1)
	}
}

func toZapFields(fields Fields) []zap.Field {
	if len(fields) == 0 {
		return nil
	}
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]zap.Field, 0, len(keys))
	for _, key := range keys {
		result = append(result, zap.Any(key, fields[key]))
	}
	return result
}

func readBody(body io.ReadCloser) ([]byte, bool) {
	if body == nil {
		return nil, false
	}
	data, _ := io.ReadAll(io.LimitReader(body, 65537))
	if len(data) > 65536 {
		return data[:65536], true
	}
	return data, false
}

func WriteJSON(w http.ResponseWriter, statusCode int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(value)
}
