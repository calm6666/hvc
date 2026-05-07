package transcode

import "errors"

var (
	ErrRequestIDRequired         = errors.New("request_id 不能为空")
	ErrSourceURLRequired         = errors.New("source_url 不能为空")
	ErrInvalidCallbackURL        = errors.New("callback_url 格式无效，仅支持 http(s)://、grpc://、mq://")
	ErrJobNotFound               = errors.New("job not found")
	ErrUnsupportedNamingTemplate = errors.New("segment_options.naming_template_id 暂未生效，请先使用后台全局命名模板配置")
	ErrUnsupportedStorageID      = errors.New("storage_options.storage_id 暂未生效，请先使用运行时存储配置")
	ErrUnsupportedSegmentPrefix  = errors.New("storage_options.segment_prefix 暂未生效")
	ErrUnsupportedMaxWaitSeconds = errors.New("schedule_options.max_wait_seconds 暂未生效")
	ErrUnsupportedSoftDecodeFlag = errors.New("schedule_options.allow_software_decode_fallback 暂未生效")
	ErrUnsupportedVideoOptions   = errors.New("video_options 暂未生效")
)
