package admin

import (
	"fmt"
	"net/http"
	"time"

	"hvc/internal/configcenter"
	rediscache "hvc/internal/infra/cache/redis"
	"hvc/internal/infra/db/mysql"
	"hvc/internal/model"
	"hvc/pkg/idgen"
	"hvc/pkg/logx"
)

// NamingTemplateHandler 分片命名模板后台管理处理器。
//
// 提供命名模板的查询、配置和生效接口。
// 六种预置模板方案：
//
//	方案一：{job_id}-{rendition_key}-{media_type}-{number}.m4s
//	方案二：{job_id}-{resolution}-{rendition_key}-{media_type}-{number}.m4s
//	方案三：{job_id}-{quality}-{rendition_key}-{media_type}-{number}.m4s
//	方案四：{job_id}-{rendition_key}-{media_type}-{number}-{timestamp}.m4s
//	方案五：{job_id}-{resolution}-{rendition_key}-{media_type}-{number}-{timestamp}.m4s
//	方案六：{job_id}-{quality}-{rendition_key}-{media_type}-{number}-{timestamp}.m4s
type NamingTemplateHandler struct {
	namingTemplateRepo *mysql.NamingTemplateRepository
	runtimeConfigRepo  *mysql.RuntimeConfigRepository
	runtimeConfigCache *rediscache.RuntimeConfigCache
	effectiveConfig    *configcenter.EffectiveConfig
}

// NewNamingTemplateHandler 创建命名模板管理处理器。
func NewNamingTemplateHandler(namingTemplateRepo *mysql.NamingTemplateRepository, runtimeConfigRepo *mysql.RuntimeConfigRepository, runtimeConfigCache *rediscache.RuntimeConfigCache, effectiveConfig *configcenter.EffectiveConfig) *NamingTemplateHandler {
	return &NamingTemplateHandler{
		namingTemplateRepo: namingTemplateRepo,
		runtimeConfigRepo:  runtimeConfigRepo,
		runtimeConfigCache: runtimeConfigCache,
		effectiveConfig:    effectiveConfig,
	}
}

// ListTemplates 返回命名模板方案分页列表。
//
// GET /v1/admin/config/naming-template/list
//
// 返回六种预置模板方案及其示例，供管理员选择。
func (h *NamingTemplateHandler) ListTemplates(w http.ResponseWriter, r *http.Request) {
	templates := buildPresetTemplates()
	currentTemplate := ""
	recentTemplates := make([]map[string]any, 0)
	if h.effectiveConfig != nil {
		cfg := h.effectiveConfig.Snapshot()
		currentTemplate = cfg.Worker.SegmentTemplate
	}
	if h.namingTemplateRepo != nil {
		for _, item := range h.namingTemplateRepo.ListRecent(r.Context(), 20) {
			recentTemplates = append(recentTemplates, map[string]any{
				"id":                item.ID,
				"config_version":    item.ConfigVersion,
				"output_base_tpl":   item.OutputBaseTpl,
				"init_seg_name_tpl": item.InitSegNameTpl,
				"media_seg_name_tpl": item.MediaSegNameTpl,
				"created_at":        item.CreatedAt,
			})
		}
	}
	page, pageSize := parsePageParams(r)
	total := int64(len(templates))
	start := (page - 1) * pageSize
	if start > len(templates) {
		start = len(templates)
	}
	end := start + pageSize
	if end > len(templates) {
		end = len(templates)
	}
	writePageResponseWithMeta(w, page, pageSize, total, templates[start:end], map[string]any{
		"current_template": currentTemplate,
		"saved_templates":  recentTemplates,
	})
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
	if !decodeJSONBody(w, r, &req) {
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
			logx.WriteJSON(w, http.StatusOK, model.Response{Code: 400, Message: "template_id 无效"})
			return
		}
	} else if req.CustomTemplate != "" {
		if err := validateTemplate(req.CustomTemplate); err != nil {
			logx.WriteJSON(w, http.StatusOK, model.Response{Code: 400, Message: err.Error()})
			return
		}
		selectedTemplate = req.CustomTemplate
	} else {
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 400, Message: "template_id 和 custom_template 至少传一个"})
		return
	}

	now := time.Now()
	record := mysql.NamingTemplateRecord{
		ID:              idgen.Next(),
		ConfigVersion:   idgen.Next(),
		OutputBaseTpl:   "{job_id}",
		InitSegNameTpl:  selectedTemplate,
		MediaSegNameTpl: selectedTemplate,
		CreatedAt:       now,
	}
	if h.namingTemplateRepo != nil {
		if err := h.namingTemplateRepo.Save(r.Context(), record); err != nil {
			logx.WriteJSON(w, http.StatusOK, model.Response{Code: 500, Message: "保存命名模板失败"})
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
	if !decodeJSONBody(w, r, &req) {
		return
	}
	if req.Template == "" {
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 400, Message: "template 不能为空"})
		return
	}
	if err := validateTemplate(req.Template); err != nil {
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 400, Message: err.Error()})
		return
	}

	now := time.Now()
	record := mysql.NamingTemplateRecord{
		ID:              idgen.Next(),
		ConfigVersion:   idgen.Next(),
		OutputBaseTpl:   "{job_id}",
		InitSegNameTpl:  req.Template,
		MediaSegNameTpl: req.Template,
		CreatedAt:       now,
	}
	if h.namingTemplateRepo != nil {
		if err := h.namingTemplateRepo.Save(r.Context(), record); err != nil {
			logx.WriteJSON(w, http.StatusOK, model.Response{Code: 500, Message: "保存命名模板失败"})
			return
		}
	}

	var publishedConfigVersion uint64
	if h.effectiveConfig != nil {
		base, ok := h.runtimeConfigRepo.LatestPublished(r.Context())
		if !ok {
			logx.WriteJSON(w, http.StatusOK, model.Response{Code: 400, Message: "当前没有可作为基线的已发布运行时配置"})
			return
		}
		base.ConfigVersion = idgen.Next()
		base.Published = false
		base.PublishedBy = ""
		base.PublishedAt = nil
		base.EffectiveConfigHash = ""
		base.ChangeSummary = "activate naming template"
		base.CreatedAt = now
		base.UpdatedAt = now
		if err := h.runtimeConfigRepo.Save(r.Context(), base); err != nil {
			logx.WriteJSON(w, http.StatusOK, model.Response{Code: 500, Message: "保存运行时配置失败"})
			return
		}
		previous, previousExists, err := h.runtimeConfigRepo.SwitchPublished(r.Context(), base.ConfigVersion, "admin")
		if err != nil {
			logx.WriteJSON(w, http.StatusOK, model.Response{Code: 500, Message: "发布运行时配置失败"})
			return
		}
		publishedConfigVersion = base.ConfigVersion

		cfg := mysql.ApplyLatestNamingTemplate(r.Context(), h.namingTemplateRepo, mysql.ToDynamicRuntimeConfig(base))
		if h.runtimeConfigCache != nil {
			if err := h.runtimeConfigCache.SavePublished(r.Context(), rediscache.RuntimeConfigSnapshot{
				ConfigVersion: base.ConfigVersion,
				Config:        cfg,
			}); err != nil {
				_ = h.runtimeConfigCache.InvalidatePublished(r.Context())
				restoreErr := h.runtimeConfigRepo.RestorePublished(r.Context(), base.ConfigVersion, previous, previousExists)
				if restoreErr != nil {
					logx.Error("admin.naming_template.activate.rollback_failed", restoreErr, logx.Fields{
						"config_version":          base.ConfigVersion,
						"previous_config_version": previous.ConfigVersion,
					})
					logx.WriteJSON(w, http.StatusOK, model.Response{Code: 500, Message: "写入运行时配置缓存失败，且数据库回滚失败"})
					return
				}
				logx.WriteJSON(w, http.StatusOK, model.Response{Code: 500, Message: "写入运行时配置缓存失败，发布已回滚"})
				return
			}
		}
		h.effectiveConfig.ReplaceWithVersion(cfg, base.ConfigVersion)
	}

	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok", Data: map[string]any{
		"template":        req.Template,
		"config_version":  publishedConfigVersion,
		"published":       true,
		"effective_scope": "新提交的转码任务",
		"running_jobs":    "不受影响，继续使用原模板",
	}})
}

// PresetTemplate 预置模板方案。
type PresetTemplate struct {
	ID                int    `json:"id"`
	Name              string `json:"name"`
	Template          string `json:"template"`
	ExampleInitVideo  string `json:"example_init_video"`
	ExampleMediaVideo string `json:"example_media_video"`
	ExampleInitAudio  string `json:"example_init_audio"`
	ExampleMediaAudio string `json:"example_media_audio"`
	Description       string `json:"description"`
}

func buildPresetTemplates() []PresetTemplate {
	return []PresetTemplate{
		{
			ID:                1,
			Name:              "方案一：简洁模式",
			Template:          "{job_id}-{rendition_key}-{media_type}-{number}.m4s",
			ExampleInitVideo:  "1893456789012345678-Ab3kP9xQ-video-0.m4s",
			ExampleMediaVideo: "1893456789012345678-Ab3kP9xQ-video-1.m4s",
			ExampleInitAudio:  "1893456789012345678-Ab3kP9xQ-audio-0.m4s",
			ExampleMediaAudio: "1893456789012345678-Ab3kP9xQ-audio-1.m4s",
			Description:       "最简洁，带稳定 rendition_key，重试时文件名保持不变",
		},
		{
			ID:                2,
			Name:              "方案二：分辨率模式",
			Template:          "{job_id}-{resolution}-{rendition_key}-{media_type}-{number}.m4s",
			ExampleInitVideo:  "1893456789012345678-1920_1080-Ab3kP9xQ-video-0.m4s",
			ExampleMediaVideo: "1893456789012345678-1920_1080-Ab3kP9xQ-video-1.m4s",
			ExampleInitAudio:  "1893456789012345678-1920_1080-Ab3kP9xQ-audio-0.m4s",
			ExampleMediaAudio: "1893456789012345678-1920_1080-Ab3kP9xQ-audio-1.m4s",
			Description:       "分辨率可读性更高，同时保留稳定 rendition_key 防重名",
		},
		{
			ID:                3,
			Name:              "方案三：清晰度标签模式",
			Template:          "{job_id}-{quality}-{rendition_key}-{media_type}-{number}.m4s",
			ExampleInitVideo:  "1893456789012345678-1080p-Ab3kP9xQ-video-0.m4s",
			ExampleMediaVideo: "1893456789012345678-1080p-Ab3kP9xQ-video-1.m4s",
			ExampleInitAudio:  "1893456789012345678-1080p-Ab3kP9xQ-audio-0.m4s",
			ExampleMediaAudio: "1893456789012345678-1080p-Ab3kP9xQ-audio-1.m4s",
			Description:       "使用清晰度标签，兼顾可读性和稳定唯一性",
		},
		{
			ID:                4,
			Name:              "方案四：简洁+时间戳模式",
			Template:          "{job_id}-{rendition_key}-{media_type}-{number}-{timestamp}.m4s",
			ExampleInitVideo:  "1893456789012345678-Ab3kP9xQ-video-0-0.m4s",
			ExampleMediaVideo: "1893456789012345678-Ab3kP9xQ-video-1-6000.m4s",
			ExampleInitAudio:  "1893456789012345678-Ab3kP9xQ-audio-0-0.m4s",
			ExampleMediaAudio: "1893456789012345678-Ab3kP9xQ-audio-1-6000.m4s",
			Description:       "方案一基础上增加时间戳，便于排查分片时间位置",
		},
		{
			ID:                5,
			Name:              "方案五：分辨率+时间戳模式",
			Template:          "{job_id}-{resolution}-{rendition_key}-{media_type}-{number}-{timestamp}.m4s",
			ExampleInitVideo:  "1893456789012345678-1920_1080-Ab3kP9xQ-video-0-0.m4s",
			ExampleMediaVideo: "1893456789012345678-1920_1080-Ab3kP9xQ-video-1-6000.m4s",
			ExampleInitAudio:  "1893456789012345678-1920_1080-Ab3kP9xQ-audio-0-0.m4s",
			ExampleMediaAudio: "1893456789012345678-1920_1080-Ab3kP9xQ-audio-1-6000.m4s",
			Description:       "方案二基础上增加时间戳，适合线上审计和排障",
		},
		{
			ID:                6,
			Name:              "方案六：清晰度标签+时间戳模式",
			Template:          "{job_id}-{quality}-{rendition_key}-{media_type}-{number}-{timestamp}.m4s",
			ExampleInitVideo:  "1893456789012345678-1080p-Ab3kP9xQ-video-0-0.m4s",
			ExampleMediaVideo: "1893456789012345678-1080p-Ab3kP9xQ-video-1-6000.m4s",
			ExampleInitAudio:  "1893456789012345678-1080p-Ab3kP9xQ-audio-0-0.m4s",
			ExampleMediaAudio: "1893456789012345678-1080p-Ab3kP9xQ-audio-1-6000.m4s",
			Description:       "方案三基础上增加时间戳，兼顾可读性、稳定性和索引能力",
		},
	}
}

// validateTemplate 校验自定义模板是否合法。
func validateTemplate(template string) error {
	required := []string{"{job_id}", "{rendition_key}", "{media_type}", "{number}"}
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
