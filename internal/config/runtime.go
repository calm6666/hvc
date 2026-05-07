package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
	"hvc/pkg/netutil"
)

// RuntimeConfig 表示服务启动时必须加载的基础配置。
//
// 约定：
//  1. server/mysql/redis/config_center/id 属于 bootstrap 配置；
//  2. scheduler/worker/callback/storage/grpc/mq 只作为首启初始化默认值，
//     首次落库后由后台动态配置接管，不再从本地文件热更新。
type RuntimeConfig struct {
	Server       ServerConfig       `yaml:"server"`
	MySQL        MySQLConfig        `yaml:"mysql"`
	Redis        RedisConfig        `yaml:"redis"`
	MQ           MQConfig           `yaml:"mq"`
	Storage      StorageConfig      `yaml:"storage"`
	Callback     CallbackConfig     `yaml:"callback"`
	Scheduler    SchedulerConfig    `yaml:"scheduler"`
	Worker       WorkerConfig       `yaml:"worker"`
	GRPC         GRPCConfig         `yaml:"grpc"`
	ConfigCenter ConfigCenterConfig `yaml:"config_center"`
	ID           IDConfig           `yaml:"id"`
}

// MQConfig 表示消息队列连接配置。
type MQConfig struct {
	RabbitMQDSN      string        `yaml:"rabbitmq_dsn"`
	RabbitMQHost     string        `yaml:"rabbitmq_host"`
	RabbitMQPort     int           `yaml:"rabbitmq_port"`
	RabbitMQUser     string        `yaml:"rabbitmq_user"`
	RabbitMQPassword string        `yaml:"rabbitmq_password"`
	RabbitMQVHost    string        `yaml:"rabbitmq_vhost"`
	QueueName        string        `yaml:"queue_name"`
	ConsumerTag      string        `yaml:"consumer_tag"`
	PrefetchCount    int           `yaml:"prefetch_count"`
	LoopInterval     time.Duration `yaml:"loop_interval"`
	CallbackTopic    string        `yaml:"callback_topic"`
}

// DynamicRuntimeConfig 表示服务启动完成后，通过后台接口维护、持久化到 MySQL、缓存到 Redis 的动态业务配置。
type DynamicRuntimeConfig struct {
	Mode      ModeConfig      `json:"mode"`
	Scheduler SchedulerConfig `json:"scheduler"`
	Worker    WorkerConfig    `json:"worker"`
	Callback  CallbackConfig  `json:"callback"`
	Storage   StorageConfig   `json:"storage"`
	GRPC      GRPCConfig      `json:"grpc"`
	MQ        MQRuntimeConfig `json:"mq"`
}

// ServerConfig 表示服务监听与节点身份配置。
type ServerConfig struct {
	ListenAddress string `yaml:"listen_address"`
	AdvertiseIP   string `yaml:"advertise_ip"`
	ServiceName   string `yaml:"service_name"`
	NodeID        uint64 `yaml:"node_id"`
	WorkerID      string `yaml:"worker_id"`
}

// ModeConfig 表示运行时模块开关配置。
type ModeConfig struct {
	EnableHTTPServer bool `json:"enable_http_server"`
	EnableGRPCServer bool `json:"enable_grpc_server"`
	EnableMQConsumer bool `json:"enable_mq_consumer"`
	EnableScheduler  bool `json:"enable_scheduler"`
	EnableWorker     bool `json:"enable_worker"`
	EnableCallback   bool `json:"enable_callback"`
}

// GRPCConfig 表示 gRPC 监听与连接限制配置。
type GRPCConfig struct {
	ListenAddress     string        `yaml:"listen_address" json:"listen_address"`
	MaxRecvMsgSizeMB  int           `yaml:"max_recv_msg_size" json:"max_recv_msg_size"`
	MaxSendMsgSizeMB  int           `yaml:"max_send_msg_size" json:"max_send_msg_size"`
	ConnectionTimeout time.Duration `yaml:"connection_timeout" json:"connection_timeout"`
}

// MySQLConfig 表示 MySQL 基础连接配置。
type MySQLConfig struct {
	DSN             string        `yaml:"dsn"`
	ShardingEnabled bool          `yaml:"sharding_enabled"`
	ShardCount      int           `yaml:"shard_count"`
	QueryTimeout    time.Duration `yaml:"query_timeout"`
	ExecTimeout     time.Duration `yaml:"exec_timeout"`
}

// RedisConfig 表示 Redis 基础连接配置。
type RedisConfig struct {
	Addrs          []string      `yaml:"addrs"`
	Password       string        `yaml:"password"`
	DB             int           `yaml:"db"`
	ClusterEnabled bool          `yaml:"cluster_enabled"`
	DialTimeout    time.Duration `yaml:"dial_timeout"`
	ReadTimeout    time.Duration `yaml:"read_timeout"`
	WriteTimeout   time.Duration `yaml:"write_timeout"`
}

// SchedulerConfig 表示调度器动态配置。
type SchedulerConfig struct {
	LoopInterval                 time.Duration `yaml:"loop_interval" json:"loop_interval"`
	JobLeaseTTL                  time.Duration `yaml:"job_lease_ttl" json:"job_lease_ttl"`
	WorkerHeartbeatTimeout       time.Duration `yaml:"worker_heartbeat_timeout" json:"worker_heartbeat_timeout"`
	RequireHardwareEncode        bool          `yaml:"require_hardware_encode" json:"require_hardware_encode"`
	AllowSoftwareDecodeFallback  bool          `yaml:"allow_software_decode_fallback" json:"allow_software_decode_fallback"`
	RequireHardwareWatermark     bool          `yaml:"require_hardware_watermark" json:"require_hardware_watermark"`
	SoftDecodeCPULimitPercent    int           `yaml:"soft_decode_cpu_limit_percent" json:"soft_decode_cpu_limit_percent"`
	NodeCPUSafetyLimitPercent    int           `yaml:"node_cpu_safety_limit_percent" json:"node_cpu_safety_limit_percent"`
	NodeMemorySafetyLimitPercent int           `yaml:"node_memory_safety_limit_percent" json:"node_memory_safety_limit_percent"`
	NodeGPUSafetyLimitPercent    int           `yaml:"node_gpu_safety_limit_percent" json:"node_gpu_safety_limit_percent"`
	MaxGlobalTranscodeSessions   int           `yaml:"max_global_transcode_sessions" json:"max_global_transcode_sessions"`
	MaxNodeTranscodeSessions     int           `yaml:"max_node_transcode_sessions" json:"max_node_transcode_sessions"`
	MaxNodeUploadConcurrency     int           `yaml:"max_node_upload_concurrency" json:"max_node_upload_concurrency"`
	DynamicConcurrencyControl    bool          `yaml:"dynamic_concurrency_control" json:"dynamic_concurrency_control"`
}

// WorkerConfig 表示执行器动态配置。
type WorkerConfig struct {
	LoopInterval               time.Duration `yaml:"loop_interval" json:"loop_interval"`
	SingleJobUploadConcurrency int           `yaml:"single_job_upload_concurrency" json:"single_job_upload_concurrency"`
	SourceReadTimeout          time.Duration `yaml:"source_read_timeout" json:"source_read_timeout"`
	UploadRetryBaseDelay       time.Duration `yaml:"upload_retry_base_delay" json:"upload_retry_base_delay"`
	UploadRetryMaxDelay        time.Duration `yaml:"upload_retry_max_delay" json:"upload_retry_max_delay"`
	UploadMaxRetryCount        int           `yaml:"upload_max_retry_count" json:"upload_max_retry_count"`
	SegmentTemplate            string        `yaml:"segment_template" json:"segment_template"`
}

// CallbackConfig 表示回调动态配置。
type CallbackConfig struct {
	HTTPURL      string        `yaml:"http_url" json:"http_url"`
	HTTPTimeout  time.Duration `yaml:"http_timeout" json:"http_timeout"`
	GRPCTimeout  time.Duration `yaml:"grpc_timeout" json:"grpc_timeout"`
	MQTimeout    time.Duration `yaml:"mq_timeout" json:"mq_timeout"`
	RetryBackoff time.Duration `yaml:"retry_backoff" json:"retry_backoff"`
}

// MQRuntimeConfig 表示运行期 MQ 消费与投递配置。
type MQRuntimeConfig struct {
	QueueName     string        `yaml:"queue_name" json:"queue_name"`
	Host          string        `yaml:"host" json:"host"`
	Port          int           `yaml:"port" json:"port"`
	Username      string        `yaml:"username" json:"username"`
	Password      string        `yaml:"password" json:"password"`
	VHost         string        `yaml:"vhost" json:"vhost"`
	ConsumerTag   string        `yaml:"consumer_tag" json:"consumer_tag"`
	PrefetchCount int           `yaml:"prefetch_count" json:"prefetch_count"`
	LoopInterval  time.Duration `yaml:"loop_interval" json:"loop_interval"`
	CallbackTopic string        `yaml:"callback_topic" json:"callback_topic"`
}

// ConfigCenterConfig 表示配置中心连接配置。
type ConfigCenterConfig struct {
	Enabled   bool   `yaml:"enabled"`
	Endpoint  string `yaml:"endpoint"`
	Namespace string `yaml:"namespace"`
	AuthMode  string `yaml:"auth_mode"`
	Token     string `yaml:"token"`
}

// IDConfig 表示 ID 生成配置。
type IDConfig struct {
	StartTime    string `yaml:"start_time"`
	NodeBits     uint8  `yaml:"node_bits"`
	SequenceBits uint8  `yaml:"sequence_bits"`
}

// StorageConfig 表示对象存储动态配置。
type StorageConfig struct {
	Endpoint        string `yaml:"endpoint" json:"endpoint"`
	Bucket          string `yaml:"bucket" json:"bucket"`
	AccessKeyID     string `yaml:"access_key_id" json:"access_key_id"`
	SecretAccessKey string `yaml:"secret_access_key" json:"secret_access_key"`
	UseSSL          bool   `yaml:"use_ssl" json:"use_ssl"`
	BasePrefix      string `yaml:"base_prefix" json:"base_prefix"`
	PlayDomain      string `yaml:"play_domain" json:"play_domain"`
	FLVDomain       string `yaml:"flv_domain" json:"flv_domain"`
	// StorageType 存储类型："s3" 对象存储，"local" 本地存储。
	StorageType string `yaml:"storage_type" json:"storage_type"`
	// LocalBasePath 本地存储根路径，StorageType="local" 时使用。
	LocalBasePath string `yaml:"local_base_path" json:"local_base_path"`
	// DefaultStorageID 默认存储配置 ID，关联 t_storage_config 表。
	DefaultStorageID uint64 `yaml:"default_storage_id" json:"default_storage_id"`
}

// LoadRuntimeConfig 加载启动配置。
func LoadRuntimeConfig() (RuntimeConfig, error) {
	cfg, err := LoadRuntimeConfigFromYAML("configs/config.yaml")
	if err != nil {
		return RuntimeConfig{}, err
	}
	if err := ValidateRuntimeConfig(cfg); err != nil {
		return RuntimeConfig{}, err
	}
	return cfg, nil
}

// LoadRuntimeConfigFromYAML 从 YAML 文件加载启动配置。
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

// ValidateRuntimeConfig 校验启动配置。
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
	return nil
}

// ResolveAdvertiseIP 解析对外广播 IP。
//
// 如果配置文件中指定了 advertise_ip，优先使用配置值；
// 否则自动检测本机局域网 IP 地址。
// 集群模式下其他节点通过此 IP 访问本节点。
func (cfg RuntimeConfig) ResolveAdvertiseIP() string {
	if cfg.Server.AdvertiseIP != "" && cfg.Server.AdvertiseIP != "127.0.0.1" && cfg.Server.AdvertiseIP != "localhost" {
		return cfg.Server.AdvertiseIP
	}
	return netutil.DetectAdvertiseIP()
}
