package model

// 任务状态常量，表示转码任务的完整生命周期。
const (
	JobStatusCreated   = 1 // 已创建
	JobStatusQueued    = 2 // 排队中
	JobStatusAssigned  = 3 // 已分配
	JobStatusRunning   = 4 // 转码中
	JobStatusUploading = 5 // 上传中
	JobStatusCompleted = 6 // 已完成
	JobStatusFailed    = 7 // 失败
	JobStatusCanceled  = 8 // 已取消
)

// 任务阶段常量，表示转码任务当前所处的执行阶段。
const (
	StageQueued      = "QUEUED"      // 排队等待
	StageDownloading = "DOWNLOADING" // 下载源文件
	StageProbing     = "PROBING"     // 探测源信息
	StageTranscoding = "TRANSCODING" // 转码执行中
	StageUploading   = "UPLOADING"   // 分片上传中
	StageFinalizing  = "FINALIZING"  // 收尾处理
	StageCompleted   = "COMPLETED"   // 已完成
	StageFailed      = "FAILED"      // 已失败
)
