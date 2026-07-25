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
            raise ConnectionError("Соединение с демоном было закрыто")
        data.extend(packet)
    return bytes(data)


def save_pixels_to_jpg(pixels, width, height, stride, filename):
    """Автоматически создает папку и сохраняет сырые пиксели в JPEG."""
    os.makedirs(OUTPUT_DIR, exist_ok=True)

    # Кадр Wayland/X11 (ARGB8888 / XRGB8888) в памяти Little-Endian лежит как BGRX/BGRA.
    # PIL "raw" декодер с параметром "BGRX" автоматически пропускает паддинги stride.
    image = Image.frombytes("RGB", (width, height), pixels, "raw", "BGRX", stride)

    image.save(filename, "JPEG", quality=95)
    print(f"Изображение успешно сохранено: {filename}")


def main():
    client = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
    client.connect(SOCKET_PATH)

    try:
        # 1. Запрос координат (Команда 0x01)
        print("Отправка запроса GetCursorPos (0x01)...")
        client.sendall(bytes([0x01]))

        res_cursor = recv_exact(client, 9)
        status, x, y = struct.unpack("<Bii", res_cursor)
        print(f"Ответ от демона: Status={status}, X={x}, Y={y}")

        # 2. Запрос пикселей (Команда 0x02)
        print("\nОтправка запроса GetPixels (0x02)...")
        req_pixels = struct.pack("<Biiii", 0x02, 10, 10, 10, 10)
        client.sendall(req_pixels)

        header_bytes = recv_exact(client, 25)
        status, w, h, stride, timestamp, data_size = struct.unpack(
            "<BIIIQI", header_bytes
        )
        print(
            f"Заголовок пикселей: Status={status}, W={w}, H={h}, Stride={stride}, Time={timestamp}, Size={data_size} байт"
        )

        if status == 0 and data_size > 0:
            pixels = recv_exact(client, data_size)
            print(
                f"Успешно получено {len(pixels)} из {data_size} байт сырых пикселей!"
            )

            # Сохраняем в файл
            save_pixels_to_jpg(pixels, w, h, stride, OUTPUT_FILE)
        else:
            print("Ошибка получения пикселей от бэкенда")

    finally:
        client.close()


if __name__ == "__main__":
    main()