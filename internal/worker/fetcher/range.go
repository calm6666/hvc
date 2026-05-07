package fetcher

type RangeRequest struct {
	URL    string
	Offset int64
	Length int64
}

func NewRangeRequest(url string, offset int64, length int64) RangeRequest {
	return RangeRequest{
		URL:    url,
		Offset: offset,
		Length: length,
	}
}
