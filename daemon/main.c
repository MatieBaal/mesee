#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdbool.h>
#include <unistd.h>
#include <dlfcn.h>
#include <signal.h>
#include <errno.h>
#include <sys/socket.h>
#include <sys/un.h>

#include "backend/mesee_backend.h"

#define SOCKET_PATH "/tmp/mesee.sock"

// --- ДЕКЛАРАЦИЯ БИНАРНОГО ПРОТОКОЛА (убираем выравнивание структуры) ---
#pragma pack(push, 1)

typedef struct {
    uint8_t cmd; // 1 = Cursor Pos, 2 = Pixels
    MESEERectOffsets offsets; // Заполняется только если cmd == 2
} RequestHeader;

typedef struct {
    uint8_t status; // 0 = OK, 1 = Error
    int32_t x;
    int32_t y;
} ResponseCursor;

typedef struct {
    uint8_t status; // 0 = OK, 1 = Error
    int32_t width;
    int32_t height;
    uint32_t stride;
    uint64_t timestamp;
    uint32_t data_size; // Размер сырых байт, идущих следом за этой структурой
} ResponsePixelHeader;

#pragma pack(pop)
// ----------------------------------------------------------------------

// Глобальные переменные для корректной очистки по SIGINT/SIGTERM
static int g_server_fd = -1;
static void *g_dl_handle = NULL;
static MESEEBackendAPI *g_api = NULL;

void cleanup_and_exit(int code) {
    printf("\n[Daemon] Завершение работы...\n");
    if (g_api && g_api->cleanup) {
        g_api->cleanup();
    }
    if (g_dl_handle) {
        dlclose(g_dl_handle);
    }
    if (g_server_fd != -1) {
        close(g_server_fd);
    }
    unlink(SOCKET_PATH);
    exit(code);
}

void signal_handler(int sig) {
    (void)sig;
    cleanup_and_exit(0);
}

// Определение графического сервера
const char* detect_backend_library() {
    const char *session = getenv("XDG_SESSION_TYPE");
    const char *wayland_display = getenv("WAYLAND_DISPLAY");

    if ((session && strcmp(session, "wayland") == 0) || wayland_display != NULL) {
        return "./libmesee_backend_wayland.so";
    }
    return "./libmesee_backend_x11.so";
}

void handle_client(int client_fd) {
    RequestHeader req;
    
    // Считываем заголовок запроса
    ssize_t bytes_read = read(client_fd, &req, sizeof(req));
    if (bytes_read < 1) {
        return; // Ошибка или пустой запрос
    }

    if (req.cmd == 1) { // Запрос координат
        ResponseCursor res = { .status = 1, .x = 0, .y = 0 };
        
        if (g_api->get_cursor_pos(&res.x, &res.y)) {
            res.status = 0; // Success
        }
        
        write(client_fd, &res, sizeof(res));

    } else if (req.cmd == 2) { // Запрос пикселей
        int x = 0, y = 0;
        ResponsePixelHeader res = {0};

        if (!g_api->get_cursor_pos(&x, &y)) {
            res.status = 1;
            write(client_fd, &res, sizeof(res));
            return;
        }

        MESEEPixelBuffer pb = {0};
        if (g_api->get_pixels(x, y, req.offsets, &pb)) {
            res.status = 0;
            res.width = pb.width;
            res.height = pb.height;
            res.stride = pb.stride;
            res.timestamp = pb.timestamp;
            res.data_size = (uint32_t)pb.data_size;

            // 1. Отправляем бинарный заголовок с размером кадра
            write(client_fd, &res, sizeof(res));
            // 2. Отправляем сами пиксели
            if (pb.data && pb.data_size > 0) {
                write(client_fd, pb.data, pb.data_size);
            }

            // Освобождаем память с помощью бэкенда
            g_api->free_pixels(&pb);
        } else {
            res.status = 1; // Ошибка снимка
            write(client_fd, &res, sizeof(res));
        }
    }
}

int main() {
    // 1. Настройка обработки сигналов
    signal(SIGINT, signal_handler);  // Ctrl+C
    signal(SIGTERM, signal_handler); // kill
    signal(SIGPIPE, SIG_IGN);        // Игнорируем разрыв соединения клиентом

    // 2. Определение и динамическая загрузка бэкенда
    const char *lib_path = detect_backend_library();
    printf("[Daemon] Обнаружен сеанс, загрузка: %s\n", lib_path);

    g_dl_handle = dlopen(lib_path, RTLD_LAZY);
    if (!g_dl_handle) {
        fprintf(stderr, "[Error] dlopen: %s\n", dlerror());
        return 1;
    }

    BackendEntryFunc get_api = (BackendEntryFunc)dlsym(g_dl_handle, BACKEND_ENTRY_POINT);
    if (!get_api) {
        fprintf(stderr, "[Error] dlsym: %s\n", dlerror());
        cleanup_and_exit(1);
    }

    g_api = get_api();
    printf("[Daemon] Подключен бэкенд: '%s'\n", g_api->name);

    if (!g_api->init()) {
        fprintf(stderr, "[Error] Ошибка инициализации бэкенда.\n");
        cleanup_and_exit(1);
    }

    // 3. Создание Unix Domain Socket
    g_server_fd = socket(AF_UNIX, SOCK_STREAM, 0);
    if (g_server_fd == -1) {
        perror("socket");
        cleanup_and_exit(1);
    }

    struct sockaddr_un addr;
    memset(&addr, 0, sizeof(addr));
    addr.sun_family = AF_UNIX;
    strncpy(addr.sun_path, SOCKET_PATH, sizeof(addr.sun_path) - 1);

    unlink(SOCKET_PATH); // Удаляем файл сокета, если он остался от прошлых запусков

    if (bind(g_server_fd, (struct sockaddr*)&addr, sizeof(addr)) == -1) {
        perror("bind");
        cleanup_and_exit(1);
    }

    if (listen(g_server_fd, 10) == -1) {
        perror("listen");
        cleanup_and_exit(1);
    }

    printf("[Daemon] Готов к приему подключений через %s\n", SOCKET_PATH);

    // 4. Главный цикл обработки входящих соединений
    while (1) {
        int client_fd = accept(g_server_fd, NULL, NULL);
        if (client_fd == -1) {
            if (errno == EINTR) continue; // Прервано сигналом
            perror("accept");
            break;
        }

        handle_client(client_fd);
        close(client_fd);
    }

    cleanup_and_exit(0);
    return 0;
}