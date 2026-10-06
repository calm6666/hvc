package admin

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"hvc/internal/model"
	"hvc/pkg/logx"
)

func parsePageParams(r *http.Request) (int, int) {
	if r == nil {
		return 1, 20
	}
	page, _ := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("page")))
	pageSize, _ := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("page_size")))
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}

func writePageResponse(w http.ResponseWriter, page, pageSize int, total int64, items any) {
	writePageResponseWithMeta(w, page, pageSize, total, items, nil)
}

func writePageResponseWithMeta(w http.ResponseWriter, page, pageSize int, total int64, items any, meta map[string]any) {
	data := map[string]any{
		"page":      page,
		"page_size": pageSize,
		"total":     total,
		"items":     items,
	}
	for key, value := range meta {
		data[key] = value
	}
	logx.WriteJSON(w, http.StatusOK, model.Response{
		Code:    0,
		Message: "ok",
		Data:    data,
	})
}

func writeItemsResponse(w http.ResponseWriter, items any, meta map[string]any) {
	data := map[string]any{
		"items": items,
	}
	for key, value := range meta {
		data[key] = value
	}
	logx.WriteJSON(w, http.StatusOK, model.Response{
		Code:    0,
		Message: "ok",
		Data:    data,
	})
}

func decodeJSONBody(w http.ResponseWriter, r *http.Request, dst any) bool {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		logx.WriteJSON(w, http.StatusOK, model.Response{
			Code:    400,
			Message: "请求体格式无效",
		})
		return false
	}
	return true
}

func parseOptionalBool(value string) *bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return nil
	}
	return &parsed
}

func parseOptionalBoolValue(value string) bool {
	parsed := parseOptionalBool(value)
	if parsed == nil {
		return false
	}
	return *parsed
}
