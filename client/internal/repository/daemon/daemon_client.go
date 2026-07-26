package daemon

import (
	"context"
	"encoding/binary"
	"fmt"
	"image"
	"io"
	"net"

	"client/internal/domain"
)

// --- ДЕКЛАРАЦИЯ БИНАРНЫХ СТРУКТУР ---

type ResponseCursor struct {
	Status uint8
	X      int32
	Y      int32
}

// RequestPixels теперь включает X и Y, чтобы пакет ровно собирался в 25 байт
// (binary.Write записывает поля без внутренних отступов языка Go)
type RequestPixels struct {
	Cmd          uint8
	X            int32
	Y            int32
	OffsetTop    int32
	OffsetBottom int32
	OffsetLeft   int32
	OffsetRight  int32
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

// GetCursorPos получает координаты мыши через команду 0x01
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

// CaptureArea реализует интерфейс ScreenCapturer.
// Принимает context.Context, центр захвата, ширину и высоту.
func (d *DaemonClient) CaptureArea(ctx context.Context, center domain.Point, width int, height int) (image.Image, error) {
	// Используем Dialer, чтобы запрос можно было прервать через context
	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, "unix", d.socketPath)
	if err != nil {
		return nil, fmt.Errorf("ошибка подключения к сокету: %w", err)
	}
	defer conn.Close()

	// Вычисляем отступы от центра для передачи демону
	offsetTop := int32(height / 2)
	offsetBottom := int32(height) - offsetTop
	offsetLeft := int32(width / 2)
	offsetRight := int32(width) - offsetLeft

	reqPixels := RequestPixels{
		Cmd:          0x02,
		X:            int32(center.X),
		Y:            int32(center.Y),
		OffsetTop:    offsetTop,
		OffsetBottom: offsetBottom,
		OffsetLeft:   offsetLeft,
		OffsetRight:  offsetRight,
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

	return convertBGRXToImage(pixelBytes, int(header.Width), int(header.Height), int(header.Stride)), nil
}

// Вспомогательная функция (остается без изменений)
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

func (d *DaemonClient) GetBackendType(ctx context.Context) (uint8, error) {
	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, "unix", d.socketPath)
	if err != nil {
		return 0, fmt.Errorf("ошибка подключения к сокету: %w", err)
	}
	defer conn.Close()
	if err := binary.Write(conn, binary.LittleEndian, []byte{0x03}); err != nil {
		return 0, fmt.Errorf("ошибка отправки 0x03: %w", err)
	}
	var backendType uint8
	if err := binary.Read(conn, binary.LittleEndian, &backendType); err != nil {
		return 0, fmt.Errorf("ошибка определения типа графического сервера: %w", err)
	}

	return backendType, nil
}
