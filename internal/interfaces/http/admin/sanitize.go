package admin

import (
	"strings"

	"hvc/internal/infra/db/mysql"
)

func sanitizeRuntimeConfigRecord(record mysql.RuntimeConfigRecord) mysql.RuntimeConfigRecord {
	record.MQPassword = maskSecret(record.MQPassword)
	record.StorageAccessKeyID = maskSecret(record.StorageAccessKeyID)
	record.StorageSecretAccessKey = maskSecret(record.StorageSecretAccessKey)
	record.CallbackRPCEndpoint = maskSecret(record.CallbackRPCEndpoint)
	return record
}

func sanitizeConfigCenterBindingRecord(record mysql.ConfigCenterBindingRecord) mysql.ConfigCenterBindingRecord {
	record.AccessKey = maskSecret(record.AccessKey)
	record.SecretKey = maskSecret(record.SecretKey)
	record.Token = maskSecret(record.Token)
	return record
}

func maskSecret(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if len(raw) <= 4 {
		return "****"
	}
	if len(raw) <= 8 {
		return raw[:1] + "****" + raw[len(raw)-1:]
	}
	return raw[:2] + "****" + raw[len(raw)-2:]
}
