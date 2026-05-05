package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// RuntimeConfig 表示运行配置。
type RuntimeConfig struct {
	Server       ServerConfig       `yaml:"server"`
	Mode         ModeConfig         `yaml:"mode"`
	MySQL        MySQLConfig        `yaml:"mysql"`
	Redis        RedisConfig        `yaml:"redis"`
	Scheduler    SchedulerConfig    `yaml:"scheduler"`
	Worker       WorkerConfig       `yaml:"worker"`
	Callback     CallbackConfig     `yaml:"callback"`
	ConfigCenter ConfigCenterConfig `yaml:"config_center"`
	ID           IDConfig           `yaml:"id"`
	Storage      StorageConfig      `yaml:"storage"`
}

// ServerConfig 表示服务监听配置。
type ServerConfig struct {
	ListenAddress string `yaml:"listen_address"`
	ServiceName   string `yaml:"service_name"`
	NodeID        uint64 `yaml:"node_id"`
	WorkerID      string `yaml:"worker_id"`
}

// ModeConfig 表示模块启用配置。
type ModeConfig struct {
	EnableHTTPServer bool `yaml:"enable_http_server"`
	EnableGRPCServer bool `yaml:"enable_grpc_server"`
	EnableMQConsumer bool `yaml:"enable_mq_consumer"`
	EnableScheduler  bool `yaml:"enable_scheduler"`
	EnableWorker     bool `yaml:"enable_worker"`
	EnableCallback   bool `yaml:"enable_callback"`
}

// MySQLConfig 表示 MySQL 配置。
type MySQLConfig struct {
	DSN             string        `yaml:"dsn"`
	ShardingEnabled bool          `yaml:"sharding_enabled"`
	ShardCount      int           `yaml:"shard_count"`
	QueryTimeout    time.Duration `yaml:"query_timeout"`
	ExecTimeout     time.Duration `yaml:"exec_timeout"`
}

// RedisConfig 表示 Redis 配置。
type RedisConfig struct {
	Addrs          []string      `yaml:"addrs"`
	Password       string        `yaml:"password"`
	DB             int           `yaml:"db"`
	ClusterEnabled bool          `yaml:"cluster_enabled"`
	DialTimeout    time.Duration `yaml:"dial_timeout"`
	ReadTimeout    time.Duration `yaml:"read_timeout"`
	WriteTimeout   time.Duration `yaml:"write_timeout"`
}

// SchedulerConfig 表示调度配置。
type SchedulerConfig struct {
	LoopInterval                 time.Duration `yaml:"loop_interval"`
	JobLeaseTTL                  time.Duration `yaml:"job_lease_ttl"`
	WorkerHeartbeatTimeout       time.Duration `yaml:"worker_heartbeat_timeout"`
	RequireHardwareEncode        bool          `yaml:"require_hardware_encode"`
	AllowSoftwareDecodeFallback  bool          `yaml:"allow_software_decode_fallback"`
	RequireHardwareWatermark     bool          `yaml:"require_hardware_watermark"`
	SoftDecodeCPULimitPercent    int           `yaml:"soft_decode_cpu_limit_percent"`
	NodeCPUSafetyLimitPercent    int           `yaml:"node_cpu_safety_limit_percent"`
	NodeMemorySafetyLimitPercent int           `yaml:"node_memory_safety_limit_percent"`
	NodeGPUSafetyLimitPercent    int           `yaml:"node_gpu_safety_limit_percent"`
	MaxGlobalTranscodeSessions   int           `yaml:"max_global_transcode_sessions"`
	MaxNodeTranscodeSessions     int           `yaml:"max_node_transcode_sessions"`
	MaxNodeUploadConcurrency     int           `yaml:"max_node_upload_concurrency"`
	DynamicConcurrencyControl    bool          `yaml:"dynamic_concurrency_control"`
}

// WorkerConfig 表示执行配置。
type WorkerConfig struct {
	LoopInterval               time.Duration `yaml:"loop_interval"`
	SingleJobUploadConcurrency int           `yaml:"single_job_upload_concurrency"`
	SourceReadTimeout          time.Duration `yaml:"source_read_timeout"`
	UploadRetryBaseDelay       time.Duration `yaml:"upload_retry_base_delay"`
	UploadRetryMaxDelay        time.Duration `yaml:"upload_retry_max_delay"`
	UploadMaxRetryCount        int           `yaml:"upload_max_retry_count"`
}

// CallbackConfig 表示回调配置。
type CallbackConfig struct {
	HTTPTimeout  time.Duration `yaml:"http_timeout"`
	GRPCTimeout  time.Duration `yaml:"grpc_timeout"`
	MQTimeout    time.Duration `yaml:"mq_timeout"`
	RetryBackoff time.Duration `yaml:"retry_backoff"`
}

// ConfigCenterConfig 表示配置中心配置。
type ConfigCenterConfig struct {
	Enabled   bool   `yaml:"enabled"`
	Endpoint  string `yaml:"endpoint"`
	Namespace string `yaml:"namespace"`
	AuthMode  string `yaml:"auth_mode"`
	Token     string `yaml:"token"`
}

// IDConfig 表示 ID 生成配置。
type IDConfig struct {
	StartTime        string `yaml:"start_time"`
	NodeBits         uint8  `yaml:"node_bits"`
	SequenceBits     uint8  `yaml:"sequence_bits"`
	MaxJSIntegerSafe uint64 `yaml:"max_js_integer_safe"`
}

// StorageConfig 表示对象存储配置。
type StorageConfig struct {
	Endpoint        string `yaml:"endpoint"`
	Bucket          string `yaml:"bucket"`
	AccessKeyID     string `yaml:"access_key_id"`
	SecretAccessKey string `yaml:"secret_access_key"`
	UseSSL          bool   `yaml:"use_ssl"`
	BasePrefix      string `yaml:"base_prefix"`
}

// DefaultRuntimeConfig 返回默认运行配置。
func DefaultRuntimeConfig() RuntimeConfig {
	return RuntimeConfig{
		Server: ServerConfig{
			ListenAddress: ":8080",
			ServiceName:   "hvc",
			NodeID:        1,
			WorkerID:      "worker-local-1",
		},
		Mode: ModeConfig{
			EnableHTTPServer: true,
			EnableGRPCServer: true,
			EnableMQConsumer: true,
			EnableScheduler:  true,
			EnableWorker:     true,
			EnableCallback:   true,
		},
		MySQL: MySQLConfig{
			DSN:          "root:root@tcp(127.0.0.1:3306)/hvc?parseTime=true&charset=utf8mb4",
			QueryTimeout: 3 * time.Second,
			ExecTimeout:  5 * time.Second,
		},
		Redis: RedisConfig{
			Addrs:        []string{"127.0.0.1:6379"},
			DialTimeout:  2 * time.Second,
			ReadTimeout:  2 * time.Second,
			WriteTimeout: 2 * time.Second,
		},
		Scheduler: SchedulerConfig{
			LoopInterval:                 5 * time.Second,
			JobLeaseTTL:                  60 * time.Second,
			WorkerHeartbeatTimeout:       20 * time.Second,
			RequireHardwareEncode:        true,
			AllowSoftwareDecodeFallback:  true,
			RequireHardwareWatermark:     true,
			SoftDecodeCPULimitPercent:    50,
			NodeCPUSafetyLimitPercent:    85,
			NodeMemorySafetyLimitPercent: 85,
			NodeGPUSafetyLimitPercent:    90,
			MaxGlobalTranscodeSessions:   32,
			MaxNodeTranscodeSessions:     8,
			MaxNodeUploadConcurrency:     16,
			DynamicConcurrencyControl:    true,
		},
		Worker: WorkerConfig{
			LoopInterval:               5 * time.Second,
			SingleJobUploadConcurrency: 4,
			SourceReadTimeout:          30 * time.Second,
			UploadRetryBaseDelay:       500 * time.Millisecond,
			UploadRetryMaxDelay:        30 * time.Second,
			UploadMaxRetryCount:        5,
		},
		Callback: CallbackConfig{
			HTTPTimeout:  3 * time.Second,
			GRPCTimeout:  3 * time.Second,
			MQTimeout:    3 * time.Second,
			RetryBackoff: 2 * time.Second,
		},
		ConfigCenter: ConfigCenterConfig{
			Enabled: false,
		},
		ID: IDConfig{
			StartTime:        "2025-01-01T00:00:00Z",
			NodeBits:         10,
			SequenceBits:     12,
			MaxJSIntegerSafe: 9007199254740991,
		},
		Storage: StorageConfig{
			Endpoint:        "127.0.0.1:9000",
			Bucket:          "hvc",
			AccessKeyID:     "minioadmin",
			SecretAccessKey: "minioadmin",
			UseSSL:          false,
			BasePrefix:      "transcode",
		},
	}
}

// LoadRuntimeConfig 加载运行配置。
func LoadRuntimeConfig() (RuntimeConfig, error) {
	cfg, err := LoadRuntimeConfigFromYAML("configs/base.yaml")
	if err != nil {
		return RuntimeConfig{}, err
	}
	cfg, err = MergeConfigCenterRuntimeConfig(cfg)
	if err != nil {
		return RuntimeConfig{}, err
	}
	if err := ValidateRuntimeConfig(cfg); err != nil {
		return RuntimeConfig{}, err
	}
	return cfg, nil
}

// LoadRuntimeConfigFromYAML 从 YAML 文件加载运行配置。
func LoadRuntimeConfigFromYAML(path string) (RuntimeConfig, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return RuntimeConfig{}, err
	}
	var cfg RuntimeConfig
	if err := yaml.Unmarshal(content, &cfg); err != nil {
		return RuntimeConfig{}, err
	}
	return cfg, nil
}

// ValidateRuntimeConfig 校验运行配置。
func ValidateRuntimeConfig(cfg RuntimeConfig) error {
	if strings.TrimSpace(cfg.Server.ListenAddress) == "" {
		return fmt.Errorf("server.listen_address 不能为空")
	}
	if strings.TrimSpace(cfg.MySQL.DSN) == "" {
		return fmt.Errorf("mysql.dsn 不能为空")
	}
	if len(cfg.Redis.Addrs) == 0 {
		return fmt.Errorf("redis.addrs 不能为空")
	}
	if !cfg.Scheduler.RequireHardwareEncode {
		return fmt.Errorf("scheduler.require_hardware_encode 必须开启")
	}
	if cfg.Scheduler.SoftDecodeCPULimitPercent <= 0 || cfg.Scheduler.SoftDecodeCPULimitPercent > 50 {
		return fmt.Errorf("scheduler.soft_decode_cpu_limit_percent 必须在 1 到 50 之间")
	}
	if cfg.Worker.SingleJobUploadConcurrency <= 0 {
		return fmt.Errorf("worker.single_job_upload_concurrency 必须大于 0")
	}
	if cfg.ID.MaxJSIntegerSafe == 0 {
		return fmt.Errorf("id.max_js_integer_safe 不能为空")
	}
	if strings.TrimSpace(cfg.Storage.Endpoint) == "" {
		return fmt.Errorf("storage.endpoint 不能为空")
	}
	if strings.TrimSpace(cfg.Storage.Bucket) == "" {
		return fmt.Errorf("storage.bucket 不能为空")
	}
	return nil
}
