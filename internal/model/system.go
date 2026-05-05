package model

// Response 表示统一响应结构。
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// HealthData 表示健康检查返回内容。
type HealthData struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Mode   string `json:"mode"`
}
