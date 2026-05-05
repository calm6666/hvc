package errorsx

import "fmt"

// Wrap 包装错误。
func Wrap(message string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", message, err)
}
