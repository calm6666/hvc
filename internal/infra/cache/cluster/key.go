package cache

// NamespacedKey 返回统一缓存键。
func NamespacedKey(app string, env string, module string, key string) string {
	return app + ":" + env + ":" + module + ":" + key
}
