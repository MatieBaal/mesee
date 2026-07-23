package ocr

import (
	"context"
	"fmt"
	"image"
	"mesee/internal/domain"

	"github.com/kbinani/screenshot"
)

type Screenshoter struct{}

func NewScreenshoter() *Screenshoter {
	return &Screenshoter{}
}
func (s *Screenshoter) CaptureArea(ctx context.Context, center domain.Point, width int, height int) (image.Image, error) {
	x := center.X - (width / 2)
	y := center.Y - (height / 2)
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}

	bounds := image.Rect(x, y, x+width, y+height)
	img, err := screenshot.CaptureRect(bounds)
	if err != nil {
		return nil, fmt.Errorf("failed to capture screen rect:%w", err)
	}

	return img, nil
}
