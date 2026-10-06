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
	raw    Fields
	at     time.Time
}

type httpLogPolicy struct {
	captureRequestBody  bool
	captureResponseBody bool
}

var (
	logger         *zap.Logger
	queue          chan entry
	persistQueue   chan PersistedEntry
	persistSink    PersistenceSink
	serviceLabel   string
	once           sync.Once
	dropped        atomic.Uint64
	persistDropped atomic.Uint64
	stdlog         = log.New(os.Stderr, "", 0)
	outlog         = log.New(os.Stdout, "", 0)
)

// PersistedEntry 表示用于异步持久化的结构化日志实体。
type PersistedEntry struct {
	Service  string
	Level    string
	Action   string
	Fields   map[string]any
	LoggedAt time.Time
}

// PersistenceSink 表示日志持久化落点。
type PersistenceSink interface {
	SavePersistedEntry(item PersistedEntry) error
}

// BatchPersistenceSink 表示支持批量写入的日志持久化落点。
type BatchPersistenceSink interface {
	PersistenceSink
	SavePersistedEntries(items []PersistedEntry) error
}

const (
	persistBatchSize     = 100
	persistFlushInterval = 500 * time.Millisecond
	persistRetryMax      = 3
)

// Init 初始化日志组件。
func Init(serviceName string) {
	once.Do(func() {
		serviceLabel = strings.TrimSpace(serviceName)
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
		persistQueue = make(chan PersistedEntry, 8192)
		go consume()
		go consumePersistence()
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
	enqueue(zapcore.InfoLevel, action, fields)
	if shouldMirrorInfoToConsole(action) {
		outlog.Println(formatConsoleInfo(action, fields))
	}
}

// Error 输出错误日志。
func Error(action string, err error, fields Fields) {
	raw := cloneFields(fields)
	if err != nil {
		if raw == nil {
			raw = Fields{}
		}
		raw["error"] = err.Error()
	}
	enqueue(zapcore.ErrorLevel, action, raw)
}

// Middleware 返回标准 net/http 日志中间件。
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		policy := resolveHTTPLogPolicy(r.Method, r.URL.Path)
		requestBody := []byte(nil)
		requestTruncated := false
		if policy.captureRequestBody {
			requestBody, requestTruncated = readBody(r.Body)
		}
		if policy.captureRequestBody && r.Body != nil {
			r.Body = io.NopCloser(bytes.NewBuffer(requestBody))
		}
		writer := &bodyWriter{
			ResponseWriter: w,
			body:           bytes.NewBuffer(nil),
			statusCode:     http.StatusOK,
			captureBody:    policy.captureResponseBody,
		}
		next.ServeHTTP(writer, r)
		responseBody := writer.body.Bytes()
		responseTruncated := false
		if policy.captureResponseBody && len(responseBody) > 65536 {
			responseBody = responseBody[:65536]
			responseTruncated = true
		}
		safeRequestBody := sanitizeHTTPLogBody(r.URL.Path, requestBody)
		safeResponseBody := sanitizeHTTPLogBody(r.URL.Path, responseBody)
		fields := Fields{
			"method":             r.Method,
			"path":               r.URL.Path,
			"query":              r.URL.RawQuery,
			"status":             writer.statusCode,
			"latency_ms":         time.Since(start).Milliseconds(),
			"client_ip":          r.RemoteAddr,
			"user_agent":         r.UserAgent(),
			"request_body_size":  resolveRequestBodySize(r, requestBody),
			"response_body_size": writer.bytesWritten,
		}
		if policy.captureRequestBody {
			fields["request_body"] = string(safeRequestBody)
			fields["request_truncated"] = requestTruncated
		} else {
			fields["request_body_omitted"] = true
		}
		if policy.captureResponseBody {
			fields["response_body"] = string(safeResponseBody)
			fields["response_truncated"] = responseTruncated
		} else {
			fields["response_body_omitted"] = true
		}
		Info("http.request", fields)
		if droppedValue := dropped.Load(); droppedValue > 0 {
			stdlog.Println("log_queue_dropped", droppedValue)
			dropped.Store(0)
		}
	})
}

type bodyWriter struct {
	http.ResponseWriter
	body         *bytes.Buffer
	statusCode   int
	captureBody  bool
	bytesWritten int
}

func (w *bodyWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *bodyWriter) Write(data []byte) (int, error) {
	if w.captureBody {
		w.body.Write(data)
	}
	n, err := w.ResponseWriter.Write(data)
	w.bytesWritten += n
	return n, err
}

func consume() {
	for item := range queue {
		switch item.level {
		case zapcore.ErrorLevel:
			logger.Error(item.msg, item.fields...)
		default:
			logger.Info(item.msg, item.fields...)
		}
		enqueuePersistence(item)
	}
}

func enqueue(level zapcore.Level, msg string, raw Fields) {
	if logger == nil || queue == nil {
		return
	}
	now := time.Now()
	cloned := cloneFields(raw)
	fields := toZapFields(cloned)
	select {
	case queue <- entry{level: level, msg: msg, fields: fields, raw: cloned, at: now}:
	default:
		dropped.Add(1)
	}
}

// RegisterPersistenceSink 注册异步日志持久化 sink。
func RegisterPersistenceSink(sink PersistenceSink) {
	persistSink = sink
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

func resolveHTTPLogPolicy(method string, path string) httpLogPolicy {
	policy := httpLogPolicy{
		captureRequestBody:  true,
		captureResponseBody: true,
	}
	method = strings.ToUpper(strings.TrimSpace(method))
	path = strings.TrimSpace(path)
	if path == "" {
		return policy
	}
	if method == http.MethodGet {
		switch {
		case path == "/healthz",
			strings.HasPrefix(path, "/v1/manifest/"),
			path == "/v1/transcode/job/progress",
			path == "/v1/admin/transcode/job/progress",
			strings.HasPrefix(path, "/v1/admin/transcode/monitor/"),
			path == "/v1/admin/cluster/overview",
			path == "/v1/admin/cluster/realtime",
			path == "/v1/admin/cluster/topology",
			path == "/v1/admin/cluster/resource/distribution",
			path == "/v1/admin/cluster/node/metrics":
			policy.captureRequestBody = false
			policy.captureResponseBody = false
			return policy
		}
	}
	switch {
	case strings.HasPrefix(path, "/v1/internal/worker/"),
		path == "/v1/internal/jobs/lease/renew",
		strings.HasPrefix(path, "/v1/internal/segments/"),
		path == "/v1/admin/transcode/monitor/ws":
		policy.captureRequestBody = false
		policy.captureResponseBody = false
	}
	return policy
}

func resolveRequestBodySize(r *http.Request, captured []byte) int {
	if len(captured) > 0 {
		return len(captured)
	}
	if r != nil && r.ContentLength > 0 {
		return int(r.ContentLength)
	}
	return 0
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

func consumePersistence() {
	ticker := time.NewTicker(persistFlushInterval)
	defer ticker.Stop()

	batch := make([]PersistedEntry, 0, persistBatchSize)
	flush := func() {
		if len(batch) == 0 {
			return
		}
		flushPersistenceBatch(batch)
		batch = batch[:0]
	}

	for {
		select {
		case item, ok := <-persistQueue:
			if !ok {
				flush()
				return
			}
			batch = append(batch, item)
			if len(batch) >= persistBatchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}

func enqueuePersistence(item entry) {
	if persistQueue == nil {
		return
	}
	fields := make(map[string]any, len(item.raw))
	for key, value := range item.raw {
		fields[key] = value
	}
	select {
	case persistQueue <- PersistedEntry{
		Service:  serviceLabel,
		Level:    strings.ToLower(item.level.String()),
		Action:   item.msg,
		Fields:   fields,
		LoggedAt: item.at,
	}:
	default:
		persistDropped.Add(1)
		if droppedValue := persistDropped.Load(); droppedValue > 0 {
			stdlog.Println("log_persist_queue_dropped", droppedValue)
			persistDropped.Store(0)
		}
	}
}

func flushPersistenceBatch(items []PersistedEntry) {
	sink := persistSink
	if sink == nil || len(items) == 0 {
		return
	}
	if batchSink, ok := sink.(BatchPersistenceSink); ok {
		if err := savePersistedEntriesWithRetry(batchSink.SavePersistedEntries, items); err != nil {
			stdlog.Println("log_persist_batch_failed", "size", len(items), "error", err)
		}
		return
	}

	for _, item := range items {
		entry := item
		if err := savePersistedEntryWithRetry(sink.SavePersistedEntry, entry); err != nil {
			stdlog.Println("log_persist_failed", "action", entry.Action, "error", err)
		}
	}
}

func savePersistedEntriesWithRetry(save func([]PersistedEntry) error, items []PersistedEntry) error {
	var err error
	for attempt := 0; attempt < persistRetryMax; attempt++ {
		err = save(items)
		if err == nil {
			return nil
		}
		time.Sleep(resolvePersistRetryBackoff(attempt))
	}
	return err
}

func savePersistedEntryWithRetry(save func(PersistedEntry) error, item PersistedEntry) error {
	var err error
	for attempt := 0; attempt < persistRetryMax; attempt++ {
		err = save(item)
		if err == nil {
			return nil
		}
		time.Sleep(resolvePersistRetryBackoff(attempt))
	}
	return err
}

func resolvePersistRetryBackoff(attempt int) time.Duration {
	switch attempt {
	case 0:
		return 100 * time.Millisecond
	case 1:
		return 300 * time.Millisecond
	default:
		return time.Second
	}
}

func cloneFields(fields Fields) Fields {
	if len(fields) == 0 {
		return nil
	}
	cloned := make(Fields, len(fields))
	for key, value := range fields {
		cloned[key] = value
	}
	return cloned
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
