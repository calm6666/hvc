package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
	"hvc/pkg/netutil"
)

const defaultRuntimeConfigPath = "configs/config.yaml"

// RuntimeConfig 表示服务启动时必须加载的基础配置。
//
// 约定：
//  1. server/mysql/redis/config_center/id 属于 bootstrap 配置；
//  2. scheduler/worker/callback/storage/grpc/mq 不允许再出现在启动配置中；
//     这些业务运行参数只能通过后台 runtime config 维护；
//  3. 数据库表结构必须先通过 sql/*.sql 显式初始化，服务启动时不会自动建表；
//  4. 空库首启时使用程序内置默认值生成第一版 runtime config；
//  5. 首次落库后由后台动态配置接管，不再从本地文件热更新。
type RuntimeConfig struct {
	Server       ServerConfig       `yaml:"server"`
	MySQL        MySQLConfig        `yaml:"mysql"`
	Redis        RedisConfig        `yaml:"redis"`
	InternalGRPC InternalGRPCConfig `yaml:"internal_grpc"`
	// 以下字段只保留为“启动配置误用检测”入口：
	// 如果 YAML 中仍然写入这些动态业务段，ValidateRuntimeConfig 会直接报错，
	// 防止再次出现“配置文件也能驱动运行时参数”的双源问题。
	MQ           MQConfig           `yaml:"mq"`
	Storage      StorageConfig      `yaml:"storage"`
	Callback     CallbackConfig     `yaml:"callback"`
	Scheduler    SchedulerConfig    `yaml:"scheduler"`
	Worker       WorkerConfig       `yaml:"worker"`
	GRPC         GRPCConfig         `yaml:"grpc"`
	ConfigCenter ConfigCenterConfig `yaml:"config_center"`
	ID           IDConfig           `yaml:"id"`
}

// MQConfig 表示运行期 MQ 配置结构。
//
// 在 RuntimeConfig 中只用于检测“错误地把动态业务配置写进启动 YAML”。
type MQConfig struct {
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

// InternalGRPCConfig 表示集群内部 gRPC 通信的 bootstrap 配置。
//
// 这一组配置不属于运行期动态配置，节点启动后即建立监听。
// 这样可以避免对外 public gRPC 的热启停误伤集群内部通信。
type InternalGRPCConfig struct {
	Enabled             bool          `yaml:"enabled"`
	ListenAddress       string        `yaml:"listen_address"`
	SharedToken         string        `yaml:"shared_token"`
	MaxRecvMsgSizeMB    int           `yaml:"max_recv_msg_size"`
	MaxSendMsgSizeMB    int           `yaml:"max_send_msg_size"`
	GracefulStopTimeout time.Duration `yaml:"graceful_stop_timeout"`
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
	NodeMode      string `yaml:"node_mode"`
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
	configPath := ResolveRuntimeConfigPath(os.Args[1:])
	cfg, err := LoadRuntimeConfigFromYAML(configPath)
	if err != nil {
		return RuntimeConfig{}, err
	}
	if cfg.ConfigCenter.Enabled {
		if resolvedCfg, resolved, err := LoadRuntimeConfigFromConfigCenter(cfg, configPath); err != nil {
			return RuntimeConfig{}, err
		} else if resolved {
			cfg = resolvedCfg
		}
	}
	if err := ValidateRuntimeConfig(cfg); err != nil {
		return RuntimeConfig{}, err
	}
	return cfg, nil
}

// ResolveRuntimeConfigPath 按“命令行参数 > 环境变量 > 默认路径”的优先级解析启动配置文件路径。
//
// 这样可以保证 systemd、Docker、开发脚本都能复用同一套入口约定，
// 避免部署清单里写了 `--config` 或 `HVC_CONFIG_PATH`，但主程序实际上没有消费。
func ResolveRuntimeConfigPath(args []string) string {
	for idx := 0; idx < len(args); idx++ {
		arg := strings.TrimSpace(args[idx])
		if arg == "--config" && idx+1 < len(args) {
			if value := strings.TrimSpace(args[idx+1]); value != "" {
				return value
			}
			continue
		}
		if value, ok := strings.CutPrefix(arg, "--config="); ok {
			if value = strings.TrimSpace(value); value != "" {
				return value
			}
		}
	}
	if envPath := strings.TrimSpace(os.Getenv("HVC_CONFIG_PATH")); envPath != "" {
		return envPath
	}
	return defaultRuntimeConfigPath
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
	switch strings.TrimSpace(cfg.Server.NodeMode) {
	case "", "standalone", "cluster-control", "cluster-worker", "cluster-allinone":
	default:
		return fmt.Errorf("server.node_mode 只能是 standalone / cluster-control / cluster-worker / cluster-allinone")
	}
	if strings.TrimSpace(cfg.MySQL.DSN) == "" {
		return fmt.Errorf("mysql.dsn 不能为空")
	}
	if len(cfg.Redis.Addrs) == 0 {
		return fmt.Errorf("redis.addrs 不能为空")
	}
	if cfg.InternalGRPC.Enabled {
		if strings.TrimSpace(cfg.InternalGRPC.ListenAddress) == "" {
			return fmt.Errorf("internal_grpc.listen_address 不能为空")
		}
		if strings.TrimSpace(cfg.InternalGRPC.SharedToken) == "" {
			return fmt.Errorf("internal_grpc.shared_token 不能为空")
		}
	}
	if cfg.Scheduler != (SchedulerConfig{}) {
		return fmt.Errorf("scheduler 不允许出现在启动配置中，请改为通过后台 runtime config 维护")
	}
	if cfg.Worker != (WorkerConfig{}) {
		return fmt.Errorf("worker 不允许出现在启动配置中，请改为通过后台 runtime config 维护")
	}
	if cfg.Callback != (CallbackConfig{}) {
		return fmt.Errorf("callback 不允许出现在启动配置中，请改为通过后台 runtime config 维护")
	}
	if cfg.Storage != (StorageConfig{}) {
		return fmt.Errorf("storage 不允许出现在启动配置中，请改为通过后台 runtime config 维护")
	}
	if cfg.GRPC != (GRPCConfig{}) {
		return fmt.Errorf("grpc 不允许出现在启动配置中，请改为通过后台 runtime config 维护")
	}
	if cfg.MQ != (MQConfig{}) {
		return fmt.Errorf("mq 不允许出现在启动配置中，请改为通过后台 runtime config 维护")
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
