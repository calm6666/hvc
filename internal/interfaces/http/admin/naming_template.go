package admin

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"hvc/internal/configcenter"
	"hvc/internal/infra/db/mysql"
	"hvc/internal/model"
	"hvc/pkg/idgen"
	"hvc/pkg/logx"
)

// NamingTemplateHandler 分片命名模板后台管理处理器。
//
// 提供命名模板的查询、配置和生效接口。
// 六种预置模板方案：
//   方案一：{job_id}-{media_type}-{number}.m4s
//   方案二：{job_id}-{resolution}-{media_type}-{number}.m4s
//   方案三：{job_id}-{quality}-{media_type}-{number}.m4s
//   方案四：{job_id}-{media_type}-{number}-{timestamp}.m4s
//   方案五：{job_id}-{resolution}-{media_type}-{number}-{timestamp}.m4s
//   方案六：{job_id}-{quality}-{media_type}-{number}-{timestamp}.m4s
type NamingTemplateHandler struct {
	namingTemplateRepo *mysql.NamingTemplateRepository
	runtimeConfigRepo  *mysql.RuntimeConfigRepository
	effectiveConfig    *configcenter.EffectiveConfig
}

// NewNamingTemplateHandler 创建命名模板管理处理器。
func NewNamingTemplateHandler(namingTemplateRepo *mysql.NamingTemplateRepository, runtimeConfigRepo *mysql.RuntimeConfigRepository, effectiveConfig *configcenter.EffectiveConfig) *NamingTemplateHandler {
	return &NamingTemplateHandler{
		namingTemplateRepo: namingTemplateRepo,
		runtimeConfigRepo:  runtimeConfigRepo,
		effectiveConfig:    effectiveConfig,
	}
}

// ListTemplates 返回全部命名模板方案列表。
//
// GET /v1/admin/config/naming-template/list
//
// 返回六种预置模板方案及其示例，供管理员选择。
func (h *NamingTemplateHandler) ListTemplates(w http.ResponseWriter, r *http.Request) {
	templates := buildPresetTemplates()
	currentTemplate := ""
	if h.effectiveConfig != nil {
		cfg := h.effectiveConfig.Snapshot()
		currentTemplate = cfg.Worker.SegmentTemplate
	}
	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok", Data: map[string]any{
		"templates":        templates,
		"current_template": currentTemplate,
	}})
}

// ConfigureTemplate 配置命名模板。
//
// POST /v1/admin/config/naming-template/configure
//
// 管理员选择一种模板方案或自定义模板，写入待发布配置。
// 需要通过 /v1/admin/config/publish 发布后才生效。
func (h *NamingTemplateHandler) ConfigureTemplate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TemplateID     int    `json:"template_id"`
		CustomTemplate string `json:"custom_template"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logx.WriteJSON(w, http.StatusBadRequest, model.Response{Code: 400, Message: "invalid request"})
		return
	}

	selectedTemplate := ""
	if req.TemplateID > 0 {
		templates := buildPresetTemplates()
		for _, t := range templates {
			if t.ID == req.TemplateID {
				selectedTemplate = t.Template
				break
			}
		}
		if selectedTemplate == "" {
			logx.WriteJSON(w, http.StatusBadRequest, model.Response{Code: 400, Message: "invalid template_id"})
			return
		}
	} else if req.CustomTemplate != "" {
		if err := validateTemplate(req.CustomTemplate); err != nil {
			logx.WriteJSON(w, http.StatusBadRequest, model.Response{Code: 400, Message: err.Error()})
			return
		}
		selectedTemplate = req.CustomTemplate
	} else {
		logx.WriteJSON(w, http.StatusBadRequest, model.Response{Code: 400, Message: "template_id or custom_template required"})
		return
	}

	now := time.Now()
	record := mysql.NamingTemplateRecord{
		ID:              idgen.Next(),
		ConfigVersion:   idgen.Next(),
		InitSegNameTpl:  selectedTemplate,
		MediaSegNameTpl: selectedTemplate,
		CreatedAt:       now,
	}
	if h.namingTemplateRepo != nil {
		if err := h.namingTemplateRepo.Save(r.Context(), record); err != nil {
			logx.WriteJSON(w, http.StatusInternalServerError, model.Response{Code: 500, Message: "save naming template failed"})
			return
		}
	}

	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok", Data: map[string]any{
		"template":        selectedTemplate,
		"config_version":  record.ConfigVersion,
		"published":       false,
		"effective_scope": "需通过 /v1/admin/config/publish 发布后生效",
	}})
}

// ActivateTemplate 立即生效命名模板。
//
// POST /v1/admin/config/naming-template/activate
//
// 直接修改当前运行时配置中的分片命名模板，立即对新任务生效。
// 已运行中的任务不受影响。
func (h *NamingTemplateHandler) ActivateTemplate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Template string `json:"template"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logx.WriteJSON(w, http.StatusBadRequest, model.Response{Code: 400, Message: "invalid request"})
		return
	}
	if req.Template == "" {
		logx.WriteJSON(w, http.StatusBadRequest, model.Response{Code: 400, Message: "template required"})
		return
	}
	if err := validateTemplate(req.Template); err != nil {
		logx.WriteJSON(w, http.StatusBadRequest, model.Response{Code: 400, Message: err.Error()})
		return
	}

	if h.effectiveConfig != nil {
		cfg := h.effectiveConfig.Snapshot()
		cfg.Worker.SegmentTemplate = req.Template
		h.effectiveConfig.Replace(cfg)
	}

	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok", Data: map[string]any{
		"template":         req.Template,
		"published":        true,
		"effective_scope":  "新提交的转码任务",
		"running_jobs":     "不受影响，继续使用原模板",
	}})
}

// PresetTemplate 预置模板方案。
type PresetTemplate struct {
	ID                  int    `json:"id"`
	Name                string `json:"name"`
	Template            string `json:"template"`
	ExampleInitVideo    string `json:"example_init_video"`
	ExampleMediaVideo   string `json:"example_media_video"`
	ExampleInitAudio    string `json:"example_init_audio"`
	ExampleMediaAudio   string `json:"example_media_audio"`
	Description         string `json:"description"`
}

func buildPresetTemplates() []PresetTemplate {
	return []PresetTemplate{
		{
			ID:                1,
			Name:              "方案一：简洁模式",
			Template:          "{job_id}-{media_type}-{number}.m4s",
			ExampleInitVideo:  "1893456789012345678-video-0.m4s",
			ExampleMediaVideo: "1893456789012345678-video-1.m4s",
			ExampleInitAudio:  "1893456789012345678-audio-0.m4s",
			ExampleMediaAudio: "1893456789012345678-audio-1.m4s",
			Description:       "最简洁，仅包含任务ID、媒体类型和序号",
		},
		{
			ID:                2,
			Name:              "方案二：分辨率模式",
			Template:          "{job_id}-{resolution}-{media_type}-{number}.m4s",
			ExampleInitVideo:  "1893456789012345678-1920_1080-video-0.m4s",
			ExampleMediaVideo: "1893456789012345678-1920_1080-video-1.m4s",
			ExampleInitAudio:  "1893456789012345678-1920_1080-audio-0.m4s",
			ExampleMediaAudio: "1893456789012345678-1920_1080-audio-1.m4s",
			Description:       "分辨率使用视频真实宽高（下划线连接），文件名自描述",
		},
		{
			ID:                3,
			Name:              "方案三：清晰度标签模式",
			Template:          "{job_id}-{quality}-{media_type}-{number}.m4s",
			ExampleInitVideo:  "1893456789012345678-1080p-video-0.m4s",
			ExampleMediaVideo: "1893456789012345678-1080p-video-1.m4s",
			ExampleInitAudio:  "1893456789012345678-1080p-audio-0.m4s",
			ExampleMediaAudio: "1893456789012345678-1080p-audio-1.m4s",
			Description:       "使用清晰度标签（1080p/720p/480p），简洁易读",
		},
		{
			ID:                4,
			Name:              "方案四：简洁+时间戳模式",
			Template:          "{job_id}-{media_type}-{number}-{timestamp}.m4s",
			ExampleInitVideo:  "1893456789012345678-video-0-0.m4s",
			ExampleMediaVideo: "1893456789012345678-video-1-6000.m4s",
			ExampleInitAudio:  "1893456789012345678-audio-0-0.m4s",
			ExampleMediaAudio: "1893456789012345678-audio-1-6000.m4s",
			Description:       "方案一基础上增加分片起始时间戳（毫秒）",
		},
		{
			ID:                5,
			Name:              "方案五：分辨率+时间戳模式",
			Template:          "{job_id}-{resolution}-{media_type}-{number}-{timestamp}.m4s",
			ExampleInitVideo:  "1893456789012345678-1920_1080-video-0-0.m4s",
			ExampleMediaVideo: "1893456789012345678-1920_1080-video-1-6000.m4s",
			ExampleInitAudio:  "1893456789012345678-1920_1080-audio-0-0.m4s",
			ExampleMediaAudio: "1893456789012345678-1920_1080-audio-1-6000.m4s",
			Description:       "方案二基础上增加时间戳，适合生产环境",
		},
		{
			ID:                6,
			Name:              "方案六：清晰度标签+时间戳模式",
			Template:          "{job_id}-{quality}-{media_type}-{number}-{timestamp}.m4s",
			ExampleInitVideo:  "1893456789012345678-1080p-video-0-0.m4s",
			ExampleMediaVideo: "1893456789012345678-1080p-video-1-6000.m4s",
			ExampleInitAudio:  "1893456789012345678-1080p-audio-0-0.m4s",
			ExampleMediaAudio: "1893456789012345678-1080p-audio-1-6000.m4s",
			Description:       "方案三基础上增加时间戳，兼顾可读性和时间索引",
		},
	}
}

// validateTemplate 校验自定义模板是否合法。
func validateTemplate(template string) error {
	required := []string{"{job_id}", "{media_type}", "{number}"}
	for _, r := range required {
		if !contains(template, r) {
			return fmt.Errorf("模板必须包含 %s 占位符", r)
		}
	}
	return nil
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
