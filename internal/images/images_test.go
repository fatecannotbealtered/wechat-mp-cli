package images

import (
	"image"
	"image/color"
	"image/png"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
)

func TestPrepareSmallPNGNoProcessing(t *testing.T) {
	path := writePNG(t, 40, 40)
	prepared, err := Prepare(path, "body")
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	if prepared.Processed {
		t.Fatalf("small png should not be processed: %#v", prepared)
	}
	if prepared.ContentType != "image/png" {
		t.Fatalf("ContentType = %q", prepared.ContentType)
	}
}

func TestPrepareLargePNGProcessesUnderLimit(t *testing.T) {
	path := writePNG(t, 1600, 1600)
	prepared, err := Prepare(path, "body")
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	if !prepared.Processed {
		t.Fatalf("large png should be processed")
	}
	if prepared.Size > BodyImageLimit {
		t.Fatalf("prepared size = %d, limit = %d", prepared.Size, BodyImageLimit)
	}
	if prepared.ContentType != "image/jpeg" {
		t.Fatalf("ContentType = %q", prepared.ContentType)
	}
}

func writePNG(t *testing.T, width, height int) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	rng := rand.New(rand.NewSource(42))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{R: uint8(rng.Intn(256)), G: uint8(rng.Intn(256)), B: uint8(rng.Intn(256)), A: 255})
		}
	}
	path := filepath.Join(t.TempDir(), "image.png")
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	defer file.Close()
	if err := png.Encode(file, img); err != nil {
		t.Fatalf("png.Encode() error = %v", err)
	}
	return path
}
