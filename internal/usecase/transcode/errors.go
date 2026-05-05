package transcode

import "errors"

var (
	ErrRequestIDRequired = errors.New("request_id 不能为空")
	ErrSourceURLRequired = errors.New("source_url 不能为空")
	ErrJobNotFound       = errors.New("job not found")
)
