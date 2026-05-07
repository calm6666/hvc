package thumbnail

import (
	"context"
	"encoding/binary"
	"fmt"
	"image"
	"image/jpeg"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	ffmpegprocess "hvc/internal/infra/ffmpeg/process"
	"hvc/internal/model"
	"hvc/pkg/logx"
)

// SpriteResult 表示雪碧图生成结果。
type SpriteResult struct {
	SpriteImagePath string
	BinaryIndexPath string
	ThumbCount      int
	ThumbWidth      int
	ThumbHeight     int
	Rows            int
	Cols            int
	IntervalSec     int
}

// GenerateSprite 生成缩略图雪碧图和二进制索引文件。
//
// 流程：
//  1. 使用 FFmpeg 按固定间隔截取视频帧，保存为 JPEG 图片；
//  2. 将所有缩略图按行列排列拼合成一张雪碧图；
//  3. 生成 .bin 二进制索引文件，用于前端快速定位某帧在雪碧图中的位置。
//
// 参数：
//   - sourceURL: 视频源地址
//   - outputDir: 输出目录
//   - opts: 缩略图配置选项
func GenerateSprite(ctx context.Context, sourceURL string, outputDir string, opts model.ThumbnailOptions) (*SpriteResult, error) {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("创建输出目录失败: %w", err)
	}

	rows := opts.SpriteRows
	if rows <= 0 {
		rows = 10
	}
	cols := opts.SpriteCols
	if cols <= 0 {
		cols = 10
	}
	intervalSec := opts.ThumbIntervalSec
	if intervalSec <= 0 {
		intervalSec = 10
	}
	thumbWidth := opts.ThumbWidth
	if thumbWidth <= 0 {
		thumbWidth = 160
	}
	thumbHeight := opts.ThumbHeight
	if thumbHeight <= 0 {
		thumbHeight = 90
	}
	imageFormat := opts.SpriteImageFormat
	if imageFormat == "" {
		imageFormat = "jpg"
	}

	thumbDir := filepath.Join(outputDir, "thumbs")
	if err := os.MkdirAll(thumbDir, 0755); err != nil {
		return nil, fmt.Errorf("创建缩略图目录失败: %w", err)
	}

	thumbPattern := filepath.Join(thumbDir, "thumb_%04d.jpg")
	if err := extractThumbnails(ctx, sourceURL, thumbPattern, intervalSec, thumbWidth, thumbHeight); err != nil {
		return nil, fmt.Errorf("截取缩略图失败: %w", err)
	}

	thumbFiles, err := listThumbFiles(thumbDir)
	if err != nil {
		return nil, fmt.Errorf("列出缩略图文件失败: %w", err)
	}
	if len(thumbFiles) == 0 {
		return nil, fmt.Errorf("未生成任何缩略图")
	}

	spritePath := filepath.Join(outputDir, fmt.Sprintf("sprite.%s", imageFormat))
	if err := composeSpriteImage(thumbFiles, spritePath, thumbWidth, thumbHeight, rows, cols); err != nil {
		return nil, fmt.Errorf("拼合雪碧图失败: %w", err)
	}

	binPath := filepath.Join(outputDir, "sprite.bin")
	if err := generateBinaryIndex(thumbFiles, binPath, thumbWidth, thumbHeight, rows, cols, intervalSec); err != nil {
		return nil, fmt.Errorf("生成二进制索引失败: %w", err)
	}

	logx.Info("thumbnail.sprite.generated", logx.Fields{
		"sprite_path":    spritePath,
		"bin_path":       binPath,
		"thumb_count":    len(thumbFiles),
		"thumb_width":    thumbWidth,
		"thumb_height":   thumbHeight,
		"rows":           rows,
		"cols":           cols,
		"interval_sec":   intervalSec,
	})

	return &SpriteResult{
		SpriteImagePath: spritePath,
		BinaryIndexPath: binPath,
		ThumbCount:      len(thumbFiles),
		ThumbWidth:      thumbWidth,
		ThumbHeight:     thumbHeight,
		Rows:            rows,
		Cols:            cols,
		IntervalSec:     intervalSec,
	}, nil
}

// extractThumbnails 使用 FFmpeg 按固定间隔截取视频帧。
//
// FFmpeg 命令：
//
//	ffmpeg -i <source> -vf fps=1/<interval>,scale=<w>:<h> -q:v 2 <output_pattern>
func extractThumbnails(ctx context.Context, sourceURL string, outputPattern string, intervalSec int, width int, height int) error {
	ffmpegPath := ffmpegprocess.FindFFmpeg()
	if ffmpegPath == "" {
		return fmt.Errorf("未找到 FFmpeg 可执行文件")
	}

	vfFilter := fmt.Sprintf("fps=1/%d,scale=%d:%d", intervalSec, width, height)
	args := []string{
		"-hide_banner",
		"-y",
		"-i", sourceURL,
		"-vf", vfFilter,
		"-q:v", "2",
		outputPattern,
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(timeoutCtx, ffmpegPath, args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("FFmpeg 截取缩略图失败: %s, %w", string(output), err)
	}
	return nil
}

// listThumbFiles 列出缩略图目录中的所有 JPEG 文件，按文件名排序。
func listThumbFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		ext := filepath.Ext(name)
		if ext != ".jpg" && ext != ".jpeg" && ext != ".png" {
			continue
		}
		files = append(files, filepath.Join(dir, name))
	}
	return files, nil
}

// composeSpriteImage 将缩略图按行列拼合成一张雪碧图。
//
// 布局：从左到右、从上到下排列，不足的位置留空（黑色填充）。
func composeSpriteImage(thumbFiles []string, outputPath string, thumbWidth, thumbHeight, rows, cols int) error {
	spriteWidth := thumbWidth * cols
	spriteHeight := thumbHeight * rows

	sprite := image.NewRGBA(image.Rect(0, 0, spriteWidth, spriteHeight))

	for idx, thumbPath := range thumbFiles {
		row := idx / cols
		col := idx % cols
		if row >= rows {
			break
		}

		thumbImg, err := loadJPEGImage(thumbPath)
		if err != nil {
			logx.Error("thumbnail.sprite.load_thumb_failed", err, logx.Fields{
				"thumb_path": thumbPath,
				"index":      idx,
			})
			continue
		}

		xOffset := col * thumbWidth
		yOffset := row * thumbHeight

		for y := 0; y < thumbHeight && y < thumbImg.Bounds().Dy(); y++ {
			for x := 0; x < thumbWidth && x < thumbImg.Bounds().Dx(); x++ {
				sprite.Set(xOffset+x, yOffset+y, thumbImg.At(x, y))
			}
		}
	}

	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("创建雪碧图文件失败: %w", err)
	}
	defer outFile.Close()

	if err := jpeg.Encode(outFile, sprite, &jpeg.Options{Quality: 85}); err != nil {
		return fmt.Errorf("编码雪碧图失败: %w", err)
	}
	return nil
}

// loadJPEGImage 加载 JPEG 图片文件。
func loadJPEGImage(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, err := jpeg.Decode(f)
	if err != nil {
		return nil, err
	}
	return img, nil
}

// generateBinaryIndex 生成 .bin 二进制索引文件。
//
// 二进制索引格式（每条记录 32 字节）：
//
//	[0:8]   时间戳偏移（毫秒，int64 大端序）
//	[8:12]  X 坐标偏移（像素，int32 大端序）
//	[12:16] Y 坐标偏移（像素，int32 大端序）
//	[16:20] 缩略图宽度（像素，int32 大端序）
//	[20:24] 缩略图高度（像素，int32 大端序）
//	[24:28] 行索引（int32 大端序）
//	[28:32] 列索引（int32 大端序）
//
// 文件头（32 字节）：
//
//	[0:4]   魔数 0x48564354 ("HVCT")
//	[4:8]   版本号（1，int32 大端序）
//	[8:12]  缩略图总数（int32 大端序）
//	[12:16] 行数（int32 大端序）
//	[16:20] 列数（int32 大端序）
//	[20:24] 单帧宽度（int32 大端序）
//	[24:28] 单帧高度（int32 大端序）
//	[28:32] 间隔秒数（int32 大端序）
func generateBinaryIndex(thumbFiles []string, outputPath string, thumbWidth, thumbHeight, rows, cols, intervalSec int) error {
	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("创建二进制索引文件失败: %w", err)
	}
	defer outFile.Close()

	header := make([]byte, 32)
	binary.BigEndian.PutUint32(header[0:4], 0x48564354)
	binary.BigEndian.PutUint32(header[4:8], 1)
	binary.BigEndian.PutUint32(header[8:12], uint32(len(thumbFiles)))
	binary.BigEndian.PutUint32(header[12:16], uint32(rows))
	binary.BigEndian.PutUint32(header[16:20], uint32(cols))
	binary.BigEndian.PutUint32(header[20:24], uint32(thumbWidth))
	binary.BigEndian.PutUint32(header[24:28], uint32(thumbHeight))
	binary.BigEndian.PutUint32(header[28:32], uint32(intervalSec))

	if _, err := outFile.Write(header); err != nil {
		return fmt.Errorf("写入文件头失败: %w", err)
	}

	for idx, thumbPath := range thumbFiles {
		row := idx / cols
		col := idx % cols
		if row >= rows {
			break
		}

		timeOffsetMS := int64(idx * intervalSec * 1000)
		xOffset := int32(col * thumbWidth)
		yOffset := int32(row * thumbHeight)

		fi, err := os.Stat(thumbPath)
		if err != nil {
			continue
		}
		_ = fi

		record := make([]byte, 32)
		binary.BigEndian.PutUint64(record[0:8], uint64(timeOffsetMS))
		binary.BigEndian.PutUint32(record[8:12], uint32(xOffset))
		binary.BigEndian.PutUint32(record[12:16], uint32(yOffset))
		binary.BigEndian.PutUint32(record[16:20], uint32(thumbWidth))
		binary.BigEndian.PutUint32(record[20:24], uint32(thumbHeight))
		binary.BigEndian.PutUint32(record[24:28], uint32(row))
		binary.BigEndian.PutUint32(record[28:32], uint32(col))

		if _, err := outFile.Write(record); err != nil {
			return fmt.Errorf("写入索引记录失败: %w", err)
		}
	}

	return nil
}

// ParseBinaryIndex 解析 .bin 二进制索引文件。
//
// 返回文件头信息和所有索引记录。
func ParseBinaryIndex(binPath string) (header *BinaryIndexHeader, records []BinaryIndexRecord, err error) {
	data, err := os.ReadFile(binPath)
	if err != nil {
		return nil, nil, fmt.Errorf("读取二进制索引文件失败: %w", err)
	}

	if len(data) < 32 {
		return nil, nil, fmt.Errorf("二进制索引文件过小")
	}

	magic := binary.BigEndian.Uint32(data[0:4])
	if magic != 0x48564354 {
		return nil, nil, fmt.Errorf("无效的文件魔数: %x", magic)
	}

	header = &BinaryIndexHeader{
		Version:      int(binary.BigEndian.Uint32(data[4:8])),
		ThumbCount:   int(binary.BigEndian.Uint32(data[8:12])),
		Rows:         int(binary.BigEndian.Uint32(data[12:16])),
		Cols:         int(binary.BigEndian.Uint32(data[16:20])),
		ThumbWidth:   int(binary.BigEndian.Uint32(data[20:24])),
		ThumbHeight:  int(binary.BigEndian.Uint32(data[24:28])),
		IntervalSec:  int(binary.BigEndian.Uint32(data[28:32])),
	}

	records = make([]BinaryIndexRecord, 0, header.ThumbCount)
	offset := 32
	for i := 0; i < header.ThumbCount && offset+32 <= len(data); i++ {
		record := BinaryIndexRecord{
			TimeOffsetMS: int64(binary.BigEndian.Uint64(data[offset+0 : offset+8])),
			XOffset:      int(binary.BigEndian.Uint32(data[offset+8 : offset+12])),
			YOffset:      int(binary.BigEndian.Uint32(data[offset+12 : offset+16])),
			ThumbWidth:   int(binary.BigEndian.Uint32(data[offset+16 : offset+20])),
			ThumbHeight:  int(binary.BigEndian.Uint32(data[offset+20 : offset+24])),
			RowIndex:     int(binary.BigEndian.Uint32(data[offset+24 : offset+28])),
			ColIndex:     int(binary.BigEndian.Uint32(data[offset+28 : offset+32])),
		}
		records = append(records, record)
		offset += 32
	}

	return header, records, nil
}

// BinaryIndexHeader 表示二进制索引文件头。
type BinaryIndexHeader struct {
	Version     int
	ThumbCount  int
	Rows        int
	Cols        int
	ThumbWidth  int
	ThumbHeight int
	IntervalSec int
}

// BinaryIndexRecord 表示二进制索引文件中的一条记录。
type BinaryIndexRecord struct {
	TimeOffsetMS int64
	XOffset      int
	YOffset      int
	ThumbWidth   int
	ThumbHeight  int
	RowIndex     int
	ColIndex     int
}

// LookupByTime 根据时间戳在索引记录中查找对应的缩略图位置。
//
// 使用二分查找定位到时间戳对应的缩略图记录。
func LookupByTime(records []BinaryIndexRecord, timeOffsetMS int64) (BinaryIndexRecord, bool) {
	if len(records) == 0 {
		return BinaryIndexRecord{}, false
	}
	lo, hi := 0, len(records)-1
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if records[mid].TimeOffsetMS <= timeOffsetMS {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return records[lo], true
}

// BuildThumbnailOptions 从任务模型中提取缩略图配置。
func BuildThumbnailOptions(job model.TranscodeJob) model.ThumbnailOptions {
	return model.ThumbnailOptions{
		EnableSprite:        job.EnableThumbnailSprite,
		SpriteRows:          job.ThumbRows,
		SpriteCols:          job.ThumbCols,
		ThumbIntervalSec:    job.ThumbIntervalSec,
		ThumbWidth:          job.ThumbWidth,
		ThumbHeight:         job.ThumbHeight,
		SpriteImageFormat:   job.ThumbImageFormat,
		SpriteStoragePrefix: job.ThumbStoragePrefix,
		EnableBinaryIndex:   job.EnableThumbnailBinaryIndex,
		BinaryStoragePrefix: job.ThumbBinaryStoragePrefix,
		BinaryMaxSizeBytes:  job.ThumbBinaryMaxSizeBytes,
	}
}

// parseThumbIndexFromName 从缩略图文件名中解析序号。
func parseThumbIndexFromName(name string) int {
	base := filepath.Base(name)
	ext := filepath.Ext(base)
	numStr := base[len("thumb_") : len(base)-len(ext)]
	idx, _ := strconv.Atoi(numStr)
	return idx
}
