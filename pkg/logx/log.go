package logx

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
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
	stdlog  = log.New(os.Stderr, "", 0)
	outlog  = log.New(os.Stdout, "", 0)
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
		cores := make([]zapcore.Core, 0, 2)
		if fileWriter, err := newDailyFileWriteSyncer("log"); err == nil {
			cores = append(cores, zapcore.NewCore(
				zapcore.NewJSONEncoder(encoderConfig),
				fileWriter,
				zap.InfoLevel,
			))
		} else {
			stdlog.Println("log_file_init_failed", err)
		}
		cores = append(cores, zapcore.NewCore(
			zapcore.NewJSONEncoder(encoderConfig),
			zapcore.AddSync(os.Stdout),
			zap.ErrorLevel,
		))
		core := zapcore.NewTee(cores...)
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
	if shouldMirrorInfoToConsole(action) {
		outlog.Println(formatConsoleInfo(action, fields))
	}
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
		safeRequestBody := sanitizeHTTPLogBody(r.URL.Path, requestBody)
		safeResponseBody := sanitizeHTTPLogBody(r.URL.Path, responseBody)
		Info("http.request", Fields{
			"method":             r.Method,
			"path":               r.URL.Path,
			"query":              r.URL.RawQuery,
			"status":             writer.statusCode,
			"latency_ms":         time.Since(start).Milliseconds(),
			"client_ip":          r.RemoteAddr,
			"user_agent":         r.UserAgent(),
			"request_body":       string(safeRequestBody),
			"request_truncated":  requestTruncated,
			"response_body":      string(safeResponseBody),
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

func shouldMirrorInfoToConsole(action string) bool {
	switch action {
	case "netutil.advertise_ip.detected_by_udp",
		"netutil.advertise_ip.detected_by_interface",
		"netutil.advertise_ip.fallback_to_localhost",
		"ffmpeg.process.ffmpeg_found",
		"ffmpeg.process.ffprobe_found",
		"http.server.listening",
		"grpc.public.listening",
		"grpc.internal.listening",
		"mq.consumer.started",
		"runtime.config.synced":
		return true
	default:
		return false
	}
}

func formatConsoleInfo(action string, fields Fields) string {
	if len(fields) == 0 {
		return action
	}
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var buf bytes.Buffer
	buf.WriteString(action)
	for _, key := range keys {
		buf.WriteByte(' ')
		buf.WriteString(key)
		buf.WriteByte('=')
		buf.WriteString(stringifyField(fields[key]))
	}
	return buf.String()
}

func stringifyField(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case time.Duration:
		return v.String()
	default:
		return toCompactJSON(v)
	}
}

func toCompactJSON(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return "<marshal_failed>"
	}
	return string(data)
}

func sanitizeHTTPLogBody(path string, body []byte) []byte {
	if len(body) == 0 {
		return body
	}
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return body
	}
	if bytes.HasPrefix(trimmed, []byte("{")) || bytes.HasPrefix(trimmed, []byte("[")) {
		if sanitized, ok := sanitizeJSONPayload(trimmed); ok {
			return sanitized
		}
	}
	if strings.Contains(path, "/auth/login") || strings.Contains(path, "/auth/logout") {
		if sanitized, ok := sanitizeFormPayload(string(trimmed)); ok {
			return []byte(sanitized)
		}
		return []byte("<redacted>")
	}
	return body
}

func sanitizeJSONPayload(payload []byte) ([]byte, bool) {
	var value any
	if err := json.Unmarshal(payload, &value); err != nil {
		return nil, false
	}
	redactSensitiveValue(&value)
	sanitized, err := json.Marshal(value)
	if err != nil {
		return nil, false
	}
	return sanitized, true
}

func sanitizeFormPayload(payload string) (string, bool) {
	values, err := url.ParseQuery(payload)
	if err != nil {
		return "", false
	}
	for key := range values {
		if isSensitiveKey(key) {
			values.Set(key, "<redacted>")
		}
	}
	return values.Encode(), true
}

func redactSensitiveValue(value *any) {
	if value == nil || *value == nil {
		return
	}
	switch typed := (*value).(type) {
	case map[string]any:
		for key, item := range typed {
			if isSensitiveKey(key) {
				typed[key] = "<redacted>"
				continue
			}
			inner := item
			redactSensitiveValue(&inner)
			typed[key] = inner
		}
	case []any:
		for idx := range typed {
			item := typed[idx]
			redactSensitiveValue(&item)
			typed[idx] = item
		}
	}
}

func isSensitiveKey(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "password",
		"password_hash",
		"secret",
		"secret_key",
		"storage_secret_access_key",
		"mq_password",
		"token",
		"session_token",
		"access_key",
		"access_key_id",
		"authorization":
		return true
	default:
		return false
	}
}

type dailyFileWriteSyncer struct {
	mu         sync.Mutex
	dir        string
	currentDay string
	file       *os.File
}

func newDailyFileWriteSyncer(dir string) (*dailyFileWriteSyncer, error) {
	cleanDir := filepath.Clean(dir)
	if err := os.MkdirAll(cleanDir, 0755); err != nil {
		return nil, err
	}
	return &dailyFileWriteSyncer{dir: cleanDir}, nil
}

func (w *dailyFileWriteSyncer) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := w.rotateLocked(time.Now()); err != nil {
		return 0, err
	}
	return w.file.Write(p)
}

func (w *dailyFileWriteSyncer) Sync() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return nil
	}
	return w.file.Sync()
}

func (w *dailyFileWriteSyncer) rotateLocked(now time.Time) error {
	day := now.Format("2006-01-02")
	if w.file != nil && w.currentDay == day {
		return nil
	}
	if w.file != nil {
		_ = w.file.Sync()
		_ = w.file.Close()
		w.file = nil
	}
	file, err := os.OpenFile(filepath.Join(w.dir, day+".log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	w.file = file
	w.currentDay = day
	return nil
}
