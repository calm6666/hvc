package codec

import "encoding/json"

// Marshal 把任意对象编码成 JSON。
func Marshal(value any) ([]byte, error) {
	return json.Marshal(value)
}
