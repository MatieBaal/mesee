#define _GNU_SOURCE // Для strcasestr и memfd_create в Linux

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdbool.h>
#include <unistd.h>
#include <time.h>
#include <fcntl.h>
#include <sys/socket.h>
#include <sys/un.h>
#include <sys/mman.h>

#include <wayland-client.h>
#include "wlr-screencopy-unstable-v1-client-protocol.h"
#include "xdg-output-unstable-v1-client-protocol.h"
#include "mesee_backend.h"

// ============================================================================
// Типы данных и структура мониторов
// ============================================================================

typedef enum {
    COMPOSITOR_UNKNOWN = 0,
    COMPOSITOR_HYPRLAND,
    COMPOSITOR_SWAY,
    COMPOSITOR_GNOME,
    COMPOSITOR_KDE
} WaylandCompositorType;

// Узел связного списка для хранения информации о каждом мониторе (wl_output)
typedef struct OutputNode {
    struct wl_output *output;
    struct zxdg_output_v1 *xdg_output; // Для получения реальных глобальных координат
    uint32_t global_id;
    int x;          // Глобальная позиция X в композиторе (из xdg_output)
    int y;          // Глобальная позиция Y в композиторе (из xdg_output)
    int width;      // Физическая ширина (px)
    int height;     // Физическая высота (px)
    int logical_w;  // Логическая ширина на рабочем столе
    int logical_h;  // Логическая высота на рабочем столе
    int scale;      // Целочисленный масштаб экрана (DPI scaling, fallback)
    struct OutputNode *next;
} OutputNode;

// Контекст синхронной задачи захвата кадра
typedef struct {
    struct wl_buffer *buffer;
    uint8_t *data;
    size_t size;
    uint32_t width;
    uint32_t height;
    uint32_t stride;
    uint32_t format;
    int fd;
    bool done;
    bool failed;
} ScreencopyTask;

// --- Глобальные переменные бэкенда ---
static WaylandCompositorType g_compositor_type = COMPOSITOR_UNKNOWN;
static char g_hypr_socket_path[sizeof(((struct sockaddr_un *)0)->sun_path)] = {0};

static struct wl_display *g_wl_display = NULL;
static struct wl_registry *g_wl_registry = NULL;
static struct wl_shm *g_wl_shm = NULL;
static struct zwlr_screencopy_manager_v1 *g_screencopy_manager = NULL;
static struct zxdg_output_manager_v1 *g_xdg_output_manager = NULL;
static OutputNode *g_output_list = NULL;

// ============================================================================
// 1. Управление xdg_output (Реальные координаты и размеры)
// ============================================================================

static void zxdg_output_handle_logical_position(void *data, struct zxdg_output_v1 *zxdg_output, int32_t x, int32_t y) {
    (void)zxdg_output;
    OutputNode *node = (OutputNode *)data;
    node->x = x;
    node->y = y;
}

static void zxdg_output_handle_logical_size(void *data, struct zxdg_output_v1 *zxdg_output, int32_t width, int32_t height) {
    (void)zxdg_output;
    OutputNode *node = (OutputNode *)data;
    node->logical_w = width;
    node->logical_h = height;
}

static void zxdg_output_handle_done(void *data, struct zxdg_output_v1 *zxdg_output) {
    (void)data; (void)zxdg_output;
}

static void zxdg_output_handle_name(void *data, struct zxdg_output_v1 *zxdg_output, const char *name) {
    (void)data; (void)zxdg_output; (void)name;
}

static void zxdg_output_handle_description(void *data, struct zxdg_output_v1 *zxdg_output, const char *description) {
    (void)data; (void)zxdg_output; (void)description;
}

static const struct zxdg_output_v1_listener zxdg_output_listener = {
    .logical_position = zxdg_output_handle_logical_position,
    .logical_size = zxdg_output_handle_logical_size,
    .done = zxdg_output_handle_done,
    .name = zxdg_output_handle_name,
    .description = zxdg_output_handle_description,
};

// ============================================================================
// 2. Управление wl_output (Мультимониторность базовая)
// ============================================================================

static void output_handle_geometry(void *data, struct wl_output *wl_output,
                                   int32_t x, int32_t y,
                                   int32_t physical_width, int32_t physical_height,
                                   int32_t subpixel, const char *make,
                                   const char *model, int32_t transform) {
    (void)wl_output; (void)physical_width; (void)physical_height;
    (void)subpixel; (void)make; (void)model; (void)transform;
    OutputNode *node = (OutputNode *)data;
    // Fallback: Устанавливаем x/y из геометрии, если xdg_output недоступен. 
    // Современные композиторы передадут тут 0,0, но xdg_output позже перезапишет их правильными.
    if (!node->xdg_output) {
        node->x = x;
        node->y = y;
    }
}

static void output_handle_mode(void *data, struct wl_output *wl_output,
                               uint32_t flags, int32_t width,
                               int32_t height, int32_t refresh) {
    (void)wl_output; (void)refresh;
    OutputNode *node = (OutputNode *)data;
    if (flags & WL_OUTPUT_MODE_CURRENT) {
        node->width = width;
        node->height = height;
    }
}

static void output_handle_done(void *data, struct wl_output *wl_output) {
    (void)data; (void)wl_output;
}

static void output_handle_scale(void *data, struct wl_output *wl_output, int32_t factor) {
    (void)wl_output;
    OutputNode *node = (OutputNode *)data;
    node->scale = (factor > 0) ? factor : 1;
}

static const struct wl_output_listener output_listener = {
    .geometry = output_handle_geometry,
    .mode = output_handle_mode,
    .done = output_handle_done,
    .scale = output_handle_scale,
};

static void add_output(struct wl_registry *registry, uint32_t name, uint32_t version) {
    OutputNode *node = (OutputNode *)calloc(1, sizeof(OutputNode));
    if (!node) return;

    node->global_id = name;
    node->scale = 1;
    
    uint32_t bind_version = (version < 3) ? version : 3;
    node->output = wl_registry_bind(registry, name, &wl_output_interface, bind_version);
    wl_output_add_listener(node->output, &output_listener, node);

    node->next = g_output_list;
    g_output_list = node;
}

static void remove_output(uint32_t name) {
    OutputNode **curr = &g_output_list;
    while (*curr) {
        if ((*curr)->global_id == name) {
            OutputNode *to_free = *curr;
            *curr = (*curr)->next;
            if (to_free->xdg_output) zxdg_output_v1_destroy(to_free->xdg_output);
            if (to_free->output) wl_output_destroy(to_free->output);
            free(to_free);
            return;
        }
        curr = &(*curr)->next;
    }
}

static void free_output_list(void) {
    OutputNode *curr = g_output_list;
    while (curr) {
        OutputNode *next = curr->next;
        if (curr->xdg_output) zxdg_output_v1_destroy(curr->xdg_output);
        if (curr->output) wl_output_destroy(curr->output);
        free(curr);
        curr = next;
    }
    g_output_list = NULL;
}

// Поиск монитора, содержащего целевую точку (X, Y)
static OutputNode* find_output_for_point(int x, int y) {
    if (!g_output_list) return NULL;

    OutputNode *curr = g_output_list;

    while (curr) {
        // Используем логические размеры от xdg_output, если они есть. 
        // Иначе fallback на вычисление через scale (менее точно при fractional scaling).
        int lw = (curr->logical_w > 0) ? curr->logical_w : ((curr->scale > 0) ? (curr->width / curr->scale) : curr->width);
        int lh = (curr->logical_h > 0) ? curr->logical_h : ((curr->scale > 0) ? (curr->height / curr->scale) : curr->height);

        printf("[Wayland Backend] Проверка монитора: ID=%u, Pos=(%d,%d), LogicalSize=(%dx%d), PhysSize=(%dx%d), Scale=%d | Курсор: x=%d, y=%d\n",
            curr->global_id, curr->x, curr->y, lw, lh, curr->width, curr->height, curr->scale, x, y);

        // Проверяем, попадает ли точка в границы этого монитора
        if (x >= curr->x && x < (curr->x + lw) &&
            y >= curr->y && y < (curr->y + lh)) {
            return curr;
        }

        curr = curr->next;
    }
    
    // Если точка за пределами всех экранов — возвращаем NULL, чтобы не пытаться захватить пустоту.
    return NULL;
}

// ============================================================================
// 3. Обработчики wlr-screencopy и работа с памятью
// ============================================================================

static int create_shm_fd(off_t size) {
    int fd = memfd_create("mesee_screencopy", MFD_CLOEXEC | MFD_ALLOW_SEALING);
    if (fd < 0) return -1;
    if (ftruncate(fd, size) < 0) {
        close(fd);
        return -1;
    }
    return fd;
}

static void frame_handle_buffer(void *data, struct zwlr_screencopy_frame_v1 *frame,
                                uint32_t format, uint32_t width, uint32_t height, uint32_t stride) {
    ScreencopyTask *task = (ScreencopyTask *)data;
    task->format = format;
    task->width = width;
    task->height = height;
    task->stride = stride;
    task->size = (size_t)stride * height;

    task->fd = create_shm_fd(task->size);
    if (task->fd < 0) {
        task->failed = true;
        return;
    }

    task->data = mmap(NULL, task->size, PROT_READ | PROT_WRITE, MAP_SHARED, task->fd, 0);
    if (task->data == MAP_FAILED) {
        task->data = NULL;
        task->failed = true;
        return;
    }

    struct wl_shm_pool *pool = wl_shm_create_pool(g_wl_shm, task->fd, task->size);
    task->buffer = wl_shm_pool_create_buffer(pool, 0, width, height, stride, format);
    wl_shm_pool_destroy(pool);

    zwlr_screencopy_frame_v1_copy(frame, task->buffer);
}

static void frame_handle_flags(void *data, struct zwlr_screencopy_frame_v1 *frame, uint32_t flags) {
    (void)data; (void)frame; (void)flags;
}

static void frame_handle_ready(void *data, struct zwlr_screencopy_frame_v1 *frame,
                               uint32_t tv_sec_hi, uint32_t tv_sec_lo, uint32_t tv_nsec) {
    (void)frame; (void)tv_sec_hi; (void)tv_sec_lo; (void)tv_nsec;
    ((ScreencopyTask *)data)->done = true;
}

static void frame_handle_failed(void *data, struct zwlr_screencopy_frame_v1 *frame) {
    (void)frame;
    ScreencopyTask *task = (ScreencopyTask *)data;
    task->failed = true;
    task->done = true;
}

static void frame_handle_damage(void *data, struct zwlr_screencopy_frame_v1 *frame,
                                uint32_t x, uint32_t y, uint32_t width, uint32_t height) {
    (void)data; (void)frame; (void)x; (void)y; (void)width; (void)height;
}

static void frame_handle_linux_dmabuf(void *data, struct zwlr_screencopy_frame_v1 *frame,
                                      uint32_t format, uint32_t width, uint32_t height) {
    (void)data; (void)frame; (void)format; (void)width; (void)height;
}

static void frame_handle_buffer_done(void *data, struct zwlr_screencopy_frame_v1 *frame) {
    (void)data; (void)frame;
}

static const struct zwlr_screencopy_frame_v1_listener frame_listener = {
    .buffer = frame_handle_buffer,
    .flags = frame_handle_flags,
    .ready = frame_handle_ready,
    .failed = frame_handle_failed,
    .damage = frame_handle_damage,
    .linux_dmabuf = frame_handle_linux_dmabuf,
    .buffer_done = frame_handle_buffer_done,
};

// ============================================================================
// 4. Регистрация глобальных Wayland-интерфейсов
// ============================================================================

static void registry_handle_global(void *data, struct wl_registry *registry,
                                   uint32_t name, const char *interface, uint32_t version) {
    (void)data;
    if (strcmp(interface, wl_shm_interface.name) == 0) {
        g_wl_shm = wl_registry_bind(registry, name, &wl_shm_interface, 1);
    } else if (strcmp(interface, wl_output_interface.name) == 0) {
        add_output(registry, name, version);
    } else if (strcmp(interface, zwlr_screencopy_manager_v1_interface.name) == 0) {
        uint32_t bind_version = (version < 3) ? version : 3;
        g_screencopy_manager = wl_registry_bind(registry, name, &zwlr_screencopy_manager_v1_interface, bind_version);
    } else if (strcmp(interface, zxdg_output_manager_v1_interface.name) == 0) {
        uint32_t bind_version = (version < 3) ? version : 3;
        g_xdg_output_manager = wl_registry_bind(registry, name, &zxdg_output_manager_v1_interface, bind_version);
    }
}

static void registry_handle_global_remove(void *data, struct wl_registry *registry, uint32_t name) {
    (void)data; (void)registry;
    remove_output(name);
}

static const struct wl_registry_listener registry_listener = {
    .global = registry_handle_global,
    .global_remove = registry_handle_global_remove,
};

// ============================================================================
// 5. Специфичный код Hyprland IPC (Получение координат)
// ============================================================================

static bool hyprland_init(void) {
    const char *xdg = getenv("XDG_RUNTIME_DIR");
    const char *sig = getenv("HYPRLAND_INSTANCE_SIGNATURE");

    if (!sig) {
        fprintf(stderr, "[Wayland/Hyprland] Ошибка: HYPRLAND_INSTANCE_SIGNATURE не задана.\n");
        return false;
    }

    if (!xdg) xdg = "/run/user/1000";

    int len = snprintf(g_hypr_socket_path, sizeof(g_hypr_socket_path), "%s/hypr/%s/.socket.sock", xdg, sig);
    if (len >= (int)sizeof(g_hypr_socket_path)) {
        fprintf(stderr, "[Wayland/Hyprland] Ошибка: Путь к сокету превышает лимит UNIX domain socket.\n");
        g_hypr_socket_path[0] = '\0';
        return false;
    }

    printf("[Wayland Backend] Инициализирован IPC сокет Hyprland: %s\n", g_hypr_socket_path);
    return true;
}

static bool hyprland_get_cursor_pos(int *x, int *y) {
    if (g_hypr_socket_path[0] == '\0') return false;

    int sock = socket(AF_UNIX, SOCK_STREAM, 0);
    if (sock == -1) return false;

    struct sockaddr_un addr;
    memset(&addr, 0, sizeof(addr));
    addr.sun_family = AF_UNIX;
    snprintf(addr.sun_path, sizeof(addr.sun_path), "%s", g_hypr_socket_path);

    if (connect(sock, (struct sockaddr*)&addr, sizeof(addr)) == -1) {
        close(sock);
        return false;
    }

    const char *cmd = "cursorpos";
    if (write(sock, cmd, strlen(cmd)) <= 0) {
        close(sock);
        return false;
    }

    char buf[64] = {0};
    ssize_t n = read(sock, buf, sizeof(buf) - 1);
    close(sock);

    if (n <= 0) return false;

    return (sscanf(buf, "%d, %d", x, y) == 2 || sscanf(buf, "%d,%d", x, y) == 2);
}

// ============================================================================
// 6. Детектор композитора
// ============================================================================

static WaylandCompositorType detect_compositor(void) {
    if (getenv("HYPRLAND_INSTANCE_SIGNATURE") != NULL) {
        return COMPOSITOR_HYPRLAND;
    }
    if (getenv("SWAYSOCK") != NULL) {
        return COMPOSITOR_SWAY;
    }

    const char *desktop = getenv("XDG_CURRENT_DESKTOP");
    if (desktop) {
        if (strcasestr(desktop, "HYPRLAND")) return COMPOSITOR_HYPRLAND;
        if (strcasestr(desktop, "SWAY"))     return COMPOSITOR_SWAY;
        if (strcasestr(desktop, "GNOME"))    return COMPOSITOR_GNOME;
        if (strcasestr(desktop, "KDE"))      return COMPOSITOR_KDE;
    }

    return COMPOSITOR_UNKNOWN;
}

static const char* compositor_type_to_string(WaylandCompositorType type) {
    switch (type) {
        case COMPOSITOR_HYPRLAND: return "Hyprland";
        case COMPOSITOR_SWAY:     return "Sway";
        case COMPOSITOR_GNOME:    return "GNOME";
        case COMPOSITOR_KDE:      return "KDE";
        default:                  return "Unknown/Generic Wayland";
    }
}

// ============================================================================
// 7. Главный API Бэкенда (Экспортируемые функции)
// ============================================================================

static bool wayland_init(void) {
    g_compositor_type = detect_compositor();
    printf("[Wayland Backend] Обнаружен композитор: %s\n", compositor_type_to_string(g_compositor_type));

    // 1. Инициализация специфичного IPC для получения координат
    if (g_compositor_type == COMPOSITOR_HYPRLAND) {
        if (!hyprland_init()) {
            fprintf(stderr, "[Wayland Backend] Предупреждение: Не удалось инициализировать Hyprland IPC.\n");
        }
    }

    // 2. Инициализация подключения к Wayland Display и Screencopy
    g_wl_display = wl_display_connect(NULL);
    if (!g_wl_display) {
        fprintf(stderr, "[Wayland Backend] Ошибка: Не удалось подключиться к Wayland display.\n");
        return false;
    }

    g_wl_registry = wl_display_get_registry(g_wl_display);
    wl_registry_add_listener(g_wl_registry, &registry_listener, NULL);

    // Раундтрип 1: Получаем глобальные объекты (вкл. wl_output и xdg_output_manager)
    wl_display_roundtrip(g_wl_display);

    // Раундтрип 2: Инициализируем xdg_output для всех найденных wl_output
    if (g_xdg_output_manager) {
        OutputNode *curr = g_output_list;
        while (curr) {
            if (!curr->xdg_output) {
                curr->xdg_output = zxdg_output_manager_v1_get_xdg_output(g_xdg_output_manager, curr->output);
                zxdg_output_v1_add_listener(curr->xdg_output, &zxdg_output_listener, curr);
            }
            curr = curr->next;
        }
    } else {
        fprintf(stderr, "[Wayland Backend] Предупреждение: Композитор не поддерживает xdg-output-unstable-v1. Координаты мониторов могут быть неверными.\n");
    }

    // Раундтрип 3: Ждем события logical_position от xdg_output_manager
    wl_display_roundtrip(g_wl_display);

    if (!g_wl_shm || !g_screencopy_manager || !g_output_list) {
        fprintf(stderr, "[Wayland Backend] Ошибка: Отсутствуют необходимые интерфейсы.\n");
        return false;
    }

    printf("[Wayland Backend] Успешно инициализирован wlr-screencopy на мультимониторной конфигурации.\n");
    return true;
}

static bool wayland_get_cursor_pos(int *x, int *y) {
    if (!x || !y) return false;
    if (g_compositor_type == COMPOSITOR_HYPRLAND) return hyprland_get_cursor_pos(x, y);
    return false;
}

static bool wayland_get_pixels(int x, int y, MESEERectOffsets offsets, MESEEPixelBuffer *out_buffer) {
    if (!out_buffer || !g_screencopy_manager || !g_wl_display) return false;

    // 1. Находим монитор, на котором расположен курсор
    OutputNode *target_output = find_output_for_point(x, y);
    if (!target_output || !target_output->output) {
        fprintf(stderr, "[Wayland Backend] Не удалось найти монитор для координат (%d, %d)\n", x, y);
        return false;
    }

    // 2. Вычисляем логический прямоугольник захвата вокруг курсора
    int global_box_x = x - offsets.left;
    int global_box_y = y - offsets.top;
    int box_w = offsets.left + offsets.right;
    int box_h = offsets.top + offsets.bottom;

    if (box_w <= 0 || box_h <= 0) return false;

    // 3. Переводим в локальные логические координаты монитора
    int local_x = global_box_x - target_output->x;
    int local_y = global_box_y - target_output->y;

    // 4. Вычисляем дробный масштаб экрана
    int lw = (target_output->logical_w > 0) ? target_output->logical_w : ((target_output->scale > 0) ? (target_output->width / target_output->scale) : target_output->width);
    int lh = (target_output->logical_h > 0) ? target_output->logical_h : ((target_output->scale > 0) ? (target_output->height / target_output->scale) : target_output->height);
    
    // float precision позволяет корректно обрабатывать 125%, 150% масштабирование без сдвига пикселей
    double scale_x = (double)target_output->width / (double)lw;
    double scale_y = (double)target_output->height / (double)lh;

    // 5. Переводим в физические пиксели буфера (через scale)
    int phys_x = (int)(local_x * scale_x);
    int phys_y = (int)(local_y * scale_y);
    int phys_w = (int)(box_w * scale_x);
    int phys_h = (int)(box_h * scale_y);

    // 6. Безопасное клиппирование (обрезаем по границам физического экрана)
    if (phys_x < 0) { phys_w += phys_x; phys_x = 0; }
    if (phys_y < 0) { phys_h += phys_y; phys_y = 0; }
    
    if (phys_x + phys_w > target_output->width) {
        phys_w = target_output->width - phys_x;
    }
    if (phys_y + phys_h > target_output->height) {
        phys_h = target_output->height - phys_y;
    }

    if (phys_w <= 0 || phys_h <= 0) {
        fprintf(stderr, "[Wayland Backend] Область захвата вышла за пределы монитора (phys: x=%d, y=%d, w=%d, h=%d)\n",
                phys_x, phys_y, phys_w, phys_h);
        return false;
    }

    ScreencopyTask task = {0};

    // 7. Запрашиваем кадр у композитора
    struct zwlr_screencopy_frame_v1 *frame = zwlr_screencopy_manager_v1_capture_output_region(
        g_screencopy_manager, 0, target_output->output, phys_x, phys_y, phys_w, phys_h);

    if (!frame) return false;

    zwlr_screencopy_frame_v1_add_listener(frame, &frame_listener, &task);

    while (!task.done && wl_display_dispatch(g_wl_display) != -1) {
        // Ожидание готовности кадра
    }

    if (task.failed || !task.data) {
        if (task.data) munmap(task.data, task.size);
        if (task.fd >= 0) close(task.fd);
        zwlr_screencopy_frame_v1_destroy(frame);
        return false;
    }

    out_buffer->data = (uint8_t *)malloc(task.size);
    if (!out_buffer->data) {
        munmap(task.data, task.size);
        close(task.fd);
        if (task.buffer) wl_buffer_destroy(task.buffer);
        zwlr_screencopy_frame_v1_destroy(frame);
        return false;
    }

    memcpy(out_buffer->data, task.data, task.size);
    out_buffer->width = task.width;
    out_buffer->height = task.height;
    out_buffer->stride = task.stride;
    out_buffer->data_size = task.size;
    out_buffer->timestamp = (uint64_t)time(NULL);
    out_buffer->internal_ptr = NULL;

    munmap(task.data, task.size);
    close(task.fd);
    if (task.buffer) wl_buffer_destroy(task.buffer);
    zwlr_screencopy_frame_v1_destroy(frame);

    return true;
}

static void wayland_free_pixels(MESEEPixelBuffer *buffer) {
    if (buffer && buffer->data) {
        free(buffer->data);
        buffer->data = NULL;
        buffer->data_size = 0;
    }
}

static void wayland_cleanup(void) {
    free_output_list();

    if (g_screencopy_manager) {
        zwlr_screencopy_manager_v1_destroy(g_screencopy_manager);
        g_screencopy_manager = NULL;
    }
    if (g_xdg_output_manager) {
        zxdg_output_manager_v1_destroy(g_xdg_output_manager);
        g_xdg_output_manager = NULL;
    }
    if (g_wl_shm) {
        wl_shm_destroy(g_wl_shm);
        g_wl_shm = NULL;
    }
    if (g_wl_registry) {
        wl_registry_destroy(g_wl_registry);
        g_wl_registry = NULL;
    }
    if (g_wl_display) {
        wl_display_disconnect(g_wl_display);
        g_wl_display = NULL;
    }
}

static MESEEBackendAPI g_wayland_api = {
    .name = "MESEE Wayland Backend v0.5 (Multi-Monitor wlr-screencopy + xdg-output)",
    .init = wayland_init,
    .get_cursor_pos = wayland_get_cursor_pos,
    .get_pixels = wayland_get_pixels,
    .free_pixels = wayland_free_pixels,
    .cleanup = wayland_cleanup
};

MESEEBackendAPI* mesee_get_backend_api(void) {
    return &g_wayland_api;
}