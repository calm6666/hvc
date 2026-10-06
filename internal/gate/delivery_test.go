package gate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"

	"hvc/internal/model"
)

/* fakeReader 用内存里的对象表模拟对象存储，覆盖"取得到/摘要不符/取不到"三种情形。 */
type fakeReader struct {
	objects map[string][]byte
	errs    map[string]error
	reads   int
}

func (f *fakeReader) ReadObject(_ context.Context, objectKey string) ([]byte, error) {
	f.reads++

	if err := f.errs[objectKey]; err != nil {
		return nil, err
	}

	body, ok := f.objects[objectKey]
	if !ok {
		return nil, errors.New("no such object")
	}

	return body, nil
}

func digestOf(body string) string {
	sum := sha256.Sum256([]byte(body))
	return hex.EncodeToString(sum[:])
}

func fetchSegments() []model.Segment {
	return []model.Segment{
		{
			RenditionName: "720p", SegmentType: "init", IsInitSegment: true, SequenceNo: 0,
			ObjectKey: "out/720p/init.mp4", ObjectSizeBytes: uint64(len("INIT")),
			/* init 段也是上传到对象存储的对象：A6 要求它同样带回执（ETag），这里必须给。 */
			ObjectETag: "etag-init", SHA256: digestOf("INIT"),
			UploadStatus: model.SegmentUploaded,
		},
		{
			RenditionName: "720p", SequenceNo: 1,
			ObjectKey: "out/720p/1.m4s", ObjectSizeBytes: uint64(len("SEG1")),
			ObjectETag: "etag-1", SHA256: digestOf("SEG1"),
			UploadStatus: model.SegmentUploaded,
		},
	}
}

func TestSegmentFetchAndDigestPasses(t *testing.T) {
	reader := &fakeReader{
		objects: map[string][]byte{"out/720p/init.mp4": []byte("INIT"), "out/720p/1.m4s": []byte("SEG1")},
		errs:    map[string]error{},
	}

	result := RunSegmentFetchAndDigest(context.Background(), reader, fetchSegments())

	if !result.Passed || result.Skipped {
		t.Fatalf("应通过：passed=%v skipped=%v detail=%s", result.Passed, result.Skipped, result.Detail)
	}

	if reader.reads != 2 {
		t.Fatalf("init 段与媒体分片都应被取回校验，实际读取 %d 次", reader.reads)
	}
}

func TestSegmentFetchAndDigestMismatchDetected(t *testing.T) {
	reader := &fakeReader{
		/* 对象里的内容与记录不符（模拟存坏了/被覆盖）。 */
		objects: map[string][]byte{"out/720p/init.mp4": []byte("INIT"), "out/720p/1.m4s": []byte("TAMPERED")},
		errs:    map[string]error{},
	}

	result := RunSegmentFetchAndDigest(context.Background(), reader, fetchSegments())

	if result.Passed || result.Skipped {
		t.Fatalf("摘要不符必须判失败：passed=%v skipped=%v", result.Passed, result.Skipped)
	}
}

func TestSegmentFetchAndDigestFetchErrorDetected(t *testing.T) {
	reader := &fakeReader{
		objects: map[string][]byte{"out/720p/init.mp4": []byte("INIT")},
		errs:    map[string]error{"out/720p/1.m4s": errors.New("403 forbidden")},
	}

	result := RunSegmentFetchAndDigest(context.Background(), reader, fetchSegments())

	if result.Passed || result.Skipped {
		t.Fatalf("取不到分片必须判失败：passed=%v skipped=%v", result.Passed, result.Skipped)
	}
}

func TestSegmentFetchAndDigestMissingRowDigestFails(t *testing.T) {
	segments := fetchSegments()
	segments[1].SHA256 = ""

	reader := &fakeReader{
		objects: map[string][]byte{"out/720p/init.mp4": []byte("INIT"), "out/720p/1.m4s": []byte("SEG1")},
		errs:    map[string]error{},
	}

	result := RunSegmentFetchAndDigest(context.Background(), reader, segments)

	if result.Passed || result.Skipped {
		t.Fatalf("分片行没有摘要时必须判失败（不能当通过）：passed=%v skipped=%v", result.Passed, result.Skipped)
	}
}

func TestSegmentFetchAndDigestWithoutReaderSkips(t *testing.T) {
	result := RunSegmentFetchAndDigest(context.Background(), nil, fetchSegments())

	if !result.Skipped || result.Passed {
		t.Fatalf("没有读取器时必须 Skipped 且不算通过：passed=%v skipped=%v", result.Passed, result.Skipped)
	}
}
