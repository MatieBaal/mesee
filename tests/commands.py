import os
import socket
import struct
from PIL import Image

SOCKET_PATH = "/tmp/mesee.sock"
OUTPUT_DIR = "build/tests"
OUTPUT_FILE = os.path.join(OUTPUT_DIR, "capture.jpg")


def recv_exact(sock, n):
    """Гарантированно вычитывает ровно n байт из сокета."""
    data = bytearray()
    while len(data) < n:
        packet = sock.recv(n - len(data))
        if not packet:
            raise ConnectionError("Соединение с демоном закрыто")
        data.extend(packet)
    return bytes(data)


def save_pixels_to_jpg(pixels, width, height, stride, filename):
    """Сохраняет сырые пиксели в JPEG."""
    os.makedirs(OUTPUT_DIR, exist_ok=True)

    # Защита от пустых/черных кадров в случае ошибки логики
    if sum(pixels) == 0:
        print("\n[Внимание] Получен полностью черный кадр (все байты = 0)!")

    # BGRX корректно пропускает альфа-канал и читает Little-Endian Wayland SHM
    image = Image.frombytes("RGB", (width, height), pixels, "raw", "BGRX", stride)
    image.save(filename, "JPEG", quality=95)
    print(f"Изображение успешно сохранено: {filename}")


def main():
    client = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
    client.connect(SOCKET_PATH)

    try:
        print("Отправка запроса GetCursorPos (0x01)...")
        client.sendall(bytes([0x01]))

        res_cursor = recv_exact(client, 9)
        status, x, y = struct.unpack("<Bii", res_cursor)
        print(f"Ответ от демона: Status={status}, X={x}, Y={y}")

        print("\nОтправка запроса GetPixels (0x02)...")
        
        # 100 пикселей отступа от курсора во все стороны
        offset_left = 100
        offset_top = 10
        offset_right = 10
        offset_bottom = 10
        
        # ВАЖНО: Порядок упаковки должен точно совпадать со структурой в демоне C:
        # uint8_t command, int x, int y, int left, int top, int right, int bottom
        req_pixels = struct.pack(
            "<Biiiiii",
            0x02,
            x,
            y,
            offset_top,
            offset_bottom,
            offset_left,
            offset_right
        )
        client.sendall(req_pixels)

        header_bytes = recv_exact(client, 25)
        status, w, h, stride, timestamp, data_size = struct.unpack(
            "<BIIIQI", header_bytes
        )
        print(
            f"Заголовок пикселей: Status={status}, W={w}, H={h}, Stride={stride}, Size={data_size} байт"
        )

        if status == 0 and data_size > 0:
            pixels = recv_exact(client, data_size)
            print(f"Успешно получено {len(pixels)} байт сырых пикселей.")
            save_pixels_to_jpg(pixels, w, h, stride, OUTPUT_FILE)
        else:
            print("Ошибка получения пикселей от бэкенда")

    finally:
        client.close()


if __name__ == "__main__":
    main()