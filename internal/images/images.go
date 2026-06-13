package images

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
)

const BodyImageLimit = 1024 * 1024

var ErrUnsupported = errors.New("unsupported image format for automatic processing")

type Info struct {
	Path             string   `json:"path"`
	Size             int64    `json:"size"`
	Extension        string   `json:"extension"`
	MIME             string   `json:"mime"`
	NeedsProcessing  bool     `json:"needs_processing"`
	SupportedForAuto bool     `json:"supported_for_auto_processing"`
	Notes            []string `json:"notes,omitempty"`
}

type Prepared struct {
	Path         string   `json:"path"`
	Filename     string   `json:"filename"`
	ContentType  string   `json:"content_type"`
	Size         int      `json:"size"`
	Processed    bool     `json:"processed"`
	Notes        []string `json:"notes,omitempty"`
	Data         []byte   `json:"-"`
	OriginalSize int64    `json:"original_size"`
}

func Inspect(path string) (*Info, error) {
	stat, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	ext := strings.ToLower(filepath.Ext(path))
	mimeType := mime.TypeByExtension(ext)
	needs := NeedsProcessing(path, stat.Size())
	return &Info{
		Path:             filepath.Clean(path),
		Size:             stat.Size(),
		Extension:        ext,
		MIME:             mimeType,
		NeedsProcessing:  needs,
		SupportedForAuto: supportedForAuto(ext),
		Notes:            processingNotes(ext, stat.Size(), needs),
	}, nil
}

func Prepare(path string, uploadType string) (*Prepared, error) {
	info, err := Inspect(path)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if uploadType != "body" && !info.NeedsProcessing {
		return &Prepared{
			Path:         info.Path,
			Filename:     filepath.Base(path),
			ContentType:  contentType(path, data),
			Size:         len(data),
			Data:         data,
			OriginalSize: info.Size,
		}, nil
	}
	if !info.NeedsProcessing {
		return &Prepared{
			Path:         info.Path,
			Filename:     filepath.Base(path),
			ContentType:  contentType(path, data),
			Size:         len(data),
			Data:         data,
			OriginalSize: info.Size,
		}, nil
	}
	if !info.SupportedForAuto {
		return nil, fmt.Errorf("%w: %s", ErrUnsupported, info.Extension)
	}
	img, err := decodeImage(bytes.NewReader(data), info.Extension)
	if err != nil {
		return nil, err
	}
	preparedData, notes, err := encodeJPEGUnderLimit(img, BodyImageLimit)
	if err != nil {
		return nil, err
	}
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)) + ".jpg"
	return &Prepared{
		Path:         info.Path,
		Filename:     base,
		ContentType:  "image/jpeg",
		Size:         len(preparedData),
		Processed:    true,
		Notes:        append(info.Notes, notes...),
		Data:         preparedData,
		OriginalSize: info.Size,
	}, nil
}

func NeedsProcessing(path string, size int64) bool {
	ext := strings.ToLower(filepath.Ext(path))
	if size > BodyImageLimit {
		return true
	}
	switch ext {
	case ".jpg", ".jpeg", ".png":
		return false
	default:
		return true
	}
}

func supportedForAuto(ext string) bool {
	switch strings.ToLower(ext) {
	case ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".tif", ".tiff", ".webp":
		return true
	default:
		return false
	}
}

func processingNotes(ext string, size int64, needs bool) []string {
	if !needs {
		return nil
	}
	notes := []string{}
	if size > BodyImageLimit {
		notes = append(notes, "larger_than_1mb")
	}
	switch strings.ToLower(ext) {
	case ".jpg", ".jpeg", ".png":
	default:
		notes = append(notes, "converted_to_jpeg")
	}
	return notes
}

func decodeImage(reader io.Reader, ext string) (image.Image, error) {
	if strings.ToLower(ext) == ".gif" {
		img, err := gif.Decode(reader)
		if err != nil {
			return nil, err
		}
		return img, nil
	}
	img, _, err := image.Decode(reader)
	return img, err
}

func encodeJPEGUnderLimit(img image.Image, limit int) ([]byte, []string, error) {
	notes := []string{}
	working := flattenOnWhite(img)
	quality := 88
	for scaleSteps := 0; scaleSteps < 8; scaleSteps++ {
		for _, q := range []int{quality, 82, 74, 66, 58, 50} {
			data, err := encodeJPEG(working, q)
			if err != nil {
				return nil, nil, err
			}
			if len(data) <= limit || scaleSteps == 7 {
				if q != quality {
					notes = append(notes, fmt.Sprintf("jpeg_quality_%d", q))
				}
				if scaleSteps > 0 {
					notes = append(notes, "downscaled")
				}
				return data, notes, nil
			}
		}
		working = resizeNearest(working, int(float64(working.Bounds().Dx())*0.85), int(float64(working.Bounds().Dy())*0.85))
		if working.Bounds().Dx() < 64 || working.Bounds().Dy() < 64 {
			break
		}
	}
	data, err := encodeJPEG(working, 50)
	return data, notes, err
}

func encodeJPEG(img image.Image, quality int) ([]byte, error) {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func flattenOnWhite(src image.Image) image.Image {
	bounds := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	draw.Draw(dst, dst.Bounds(), &image.Uniform{C: color.White}, image.Point{}, draw.Src)
	draw.Draw(dst, dst.Bounds(), src, bounds.Min, draw.Over)
	return dst
}

func resizeNearest(src image.Image, width, height int) image.Image {
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}
	srcBounds := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		sy := srcBounds.Min.Y + y*srcBounds.Dy()/height
		for x := 0; x < width; x++ {
			sx := srcBounds.Min.X + x*srcBounds.Dx()/width
			dst.Set(x, y, src.At(sx, sy))
		}
	}
	return dst
}

func contentType(path string, data []byte) string {
	if t := mime.TypeByExtension(strings.ToLower(filepath.Ext(path))); t != "" {
		return t
	}
	if len(data) > 512 {
		return http.DetectContentType(data[:512])
	}
	return http.DetectContentType(data)
}

func WritePrepared(path string, prepared *Prepared) error {
	if prepared == nil {
		return errors.New("prepared image is nil")
	}
	return os.WriteFile(path, prepared.Data, 0o600)
}

func EncodePNG(path string, img image.Image) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	return png.Encode(file, img)
}
