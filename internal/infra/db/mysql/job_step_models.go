package mysql

import "time"

// 步骤状态（t_transcode_job_step.state）。语义与 sql/109_transcode_job_step_schema.sql 一致。
const (
	JobStepNotStarted = 0 // 未开始
	JobStepRunning    = 1 // 进行中（已开始本次尝试，attempt 已 +1）
	JobStepDone       = 2 // 已完成（续跑时跳过）
	JobStepFailed     = 3 // 失败（可结合 attempt 与重试上限决定是否再试）
)

// 步骤名。顺序即流水线顺序：续跑从第一个"未完成"的步骤开始，前面的前缀步骤直接跳过。
const (
	JobStepProbe    = "PROBE"    // 探测源信息
	JobStepPlan     = "PLAN"     // 生成转码计划（清晰度/编码参数）
	JobStepSegment  = "SEGMENT"  // 切片
	JobStepUpload   = "UPLOAD"   // 分片上传（detail 里记已上传分片游标）
	JobStepPublish  = "PUBLISH"  // 发布（清单物化 + 逐片校验）
	JobStepCallback = "CALLBACK" // 完成回调送达
)

// JobStepRecord 是 t_transcode_job_step 的持久化形态：B2 断点续跑的进度锚点。
//
// 为什么要有这张表：PROBE/PLAN/SEGMENT/UPLOAD 几个阶段的产出原先只活在内存里，进程重启或换节点
// 只能整任务重跑。把每个步骤的 state 与输入指纹 input_hash 落库之后，只要指纹一致（源与规格没变），
// 就能从第一个未完成步骤续跑；指纹不一致必须整任务重跑，否则会把上一版输入的中间产物当成本版结果。
type JobStepRecord struct {
	ID         uint64     `gorm:"column:id;primaryKey;autoIncrement"`
	JobID      uint64     `gorm:"column:job_id"`
	Step       string     `gorm:"column:step"`
	State      int        `gorm:"column:state"`
	InputHash  string     `gorm:"column:input_hash"`
	Detail     *string    `gorm:"column:detail"` // JSON 列：指针为 nil 时写 NULL（空串不是合法 JSON）
	Attempt    int        `gorm:"column:attempt"`
	StartedAt  *time.Time `gorm:"column:started_at"`
	FinishedAt *time.Time `gorm:"column:finished_at"`
	CreatedAt  time.Time  `gorm:"column:created_at"`
	UpdatedAt  time.Time  `gorm:"column:updated_at"`
}

func (JobStepRecord) TableName() string { return "t_transcode_job_step" }
