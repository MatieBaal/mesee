package daemon

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"net"
	"os"
	"path/filepath"

	"client/internal/domain"
)

// --- ДЕКЛАРАЦИЯ БИНАРНЫХ СТРУКТУР ---

type ResponseCursor struct {
	Status uint8
	X      int32
	Y      int32
}

type RequestPixels struct {
	Cmd    uint8
	Top    int32
	Right  int32
	Bottom int32
	Left   int32
}

type ResponsePixelHeader struct {
	Status    uint8
	Width     uint32
	Height    uint32
	Stride    uint32
	Timestamp uint64
	DataSize  uint32
}

// --- АДАПТЕР ДЕМОНА ---

type DaemonClient struct {
	socketPath string
}

func NewDaemonClient(socketPath string) *DaemonClient {
	return &DaemonClient{socketPath: socketPath}
}

// GetCursorPos получает координаты мыши через команда 0x01
func (d *DaemonClient) GetCursorPos() (domain.Point, error) {
	conn, err := net.Dial("unix", d.socketPath)
	if err != nil {
		return domain.Point{}, fmt.Errorf("ошибка подключения к сокету: %w", err)
	}
	defer conn.Close()

	if _, err := conn.Write([]byte{0x01}); err != nil {
		return domain.Point{}, fmt.Errorf("ошибка отправки 0x01: %w", err)
	}

	var res ResponseCursor
	if err := binary.Read(conn, binary.LittleEndian, &res); err != nil {
		return domain.Point{}, fmt.Errorf("ошибка чтения ответа 0x01: %w", err)
	}

	if res.Status != 0 {
		return domain.Point{}, fmt.Errorf("демон вернул ошибку получения координат")
	}

	return domain.Point{X: int(res.X), Y: int(res.Y)}, nil
}

// CaptureAreaЗапрашивает пиксели вокруг курсора и сохраняет/возвращает кадр
func (d *DaemonClient) CaptureArea(top, right, bottom, left int32, outputFile string) (image.Image, error) {
	conn, err := net.Dial("unix", d.socketPath)
	if err != nil {
		return nil, fmt.Errorf("ошибка подключения к сокету: %w", err)
	}
	defer conn.Close()

	reqPixels := RequestPixels{
		Cmd:    0x02,
		Top:    top,
		Right:  right,
		Bottom: bottom,
		Left:   left,
	}

	if err := binary.Write(conn, binary.LittleEndian, reqPixels); err != nil {
		return nil, fmt.Errorf("ошибка отправки 0x02: %w", err)
	}

	var header ResponsePixelHeader
	if err := binary.Read(conn, binary.LittleEndian, &header); err != nil {
		return nil, fmt.Errorf("ошибка чтения заголовка 0x02: %w", err)
	}

	if header.Status != 0 || header.DataSize == 0 {
		return nil, fmt.Errorf("ошибка получения пикселей от бэкенда")
	}

	pixelBytes := make([]byte, header.DataSize)
	if _, err := io.ReadFull(conn, pixelBytes); err != nil {
		return nil, fmt.Errorf("ошибка чтения массива пикселей: %w", err)
	}

	img := convertBGRXToImage(pixelBytes, int(header.Width), int(header.Height), int(header.Stride))

	if outputFile != "" {
		if err := saveImageToJPG(img, outputFile); err != nil {
			return nil, fmt.Errorf("ошибка сохранения JPG: %w", err)
		}
	}

	return img, nil
}

// Вспомогательные функции
func convertBGRXToImage(pixels []byte, width, height, stride int) image.Image {
	img := image.NewNRGBA(image.Rect(0, 0, width, height))

	for y := 0; y < height; y++ {
		srcRow := pixels[y*stride:]
		dstRow := img.Pix[y*img.Stride:]

		for x := 0; x < width; x++ {
			srcIdx := x * 4
			dstIdx := x * 4

			b := srcRow[srcIdx]
			g := srcRow[srcIdx+1]
			r := srcRow[srcIdx+2]

			dstRow[dstIdx] = r
			dstRow[dstIdx+1] = g
			dstRow[dstIdx+2] = b
			dstRow[dstIdx+3] = 255
		}
	}
	return img
}

func saveImageToJPG(img image.Image, filename string) error {
	if err := os.MkdirAll(filepath.Dir(filename), 0755); err != nil {
		return err
	}
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	return jpeg.Encode(file, img, &jpeg.Options{Quality: 95})
}
