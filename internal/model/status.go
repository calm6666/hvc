package model

// 任务状态常量，表示转码任务的完整生命周期。
//
// A1 回调状态机在 Completed 之后再加两段，语义：
//   - Published    分片全部上传完成、清单已物化、逐片可 GET 且 sha256 校验通过；
//   - CallbackSent 完成回调已由至少一个通道送达（HTTP/gRPC/MQ 任一成功即算送达）。
//
// 只有 CallbackSent 才是"对外承诺已兑现"的终态；Completed 只表示内容已产出。
const (
	JobStatusCreated      = 1  // 已创建
	JobStatusQueued       = 2  // 排队中
	JobStatusAssigned     = 3  // 已分配
	JobStatusRunning      = 4  // 转码中
	JobStatusUploading    = 5  // 上传中
	JobStatusCompleted    = 6  // 已完成（内容已产出，尚未发布/回调）
	JobStatusFailed       = 7  // 失败
	JobStatusCanceled     = 8  // 已取消
	JobStatusPublished    = 9  // 已发布：清单物化 + 分片可 GET + sha256 通过
	JobStatusCallbackSent = 10 // 回调已送达：至少一个通道成功
)

// 任务阶段常量，表示转码任务当前所处的执行阶段。
const (
	StageQueued       = "QUEUED"        // 排队等待
	StageDownloading  = "DOWNLOADING"   // 下载源文件
	StageProbing      = "PROBING"       // 探测源信息
	StageTranscoding  = "TRANSCODING"   // 转码执行中
	StageUploading    = "UPLOADING"     // 分片上传中
	StageFinalizing   = "FINALIZING"    // 收尾处理
	StageCompleted    = "COMPLETED"     // 已完成（内容已产出）
	StagePublished    = "PUBLISHED"     // 已发布（清单物化 + 分片校验通过）
	StageCallbackSent = "CALLBACK_SENT" // 回调已送达
	StageFailed       = "FAILED"        // 已失败
)
