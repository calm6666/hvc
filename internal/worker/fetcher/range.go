package fetcher

// RangeRequest 表示 HTTP Range 请求参数。
type RangeRequest struct {
	URL    string
	Offset int64
	Length int64
}

// NewRangeRequest 创建 Range 请求参数。
func NewRangeRequest(url string, offset int64, length int64) RangeRequest {
	return RangeRequest{
		URL:    url,
		Offset: offset,
		Length: length,
	}
}
