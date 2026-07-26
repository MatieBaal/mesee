#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdbool.h>
#include <time.h>
#include <X11/Xlib.h>
#include <X11/Xutil.h>

#include "mesee_backend.h"

// Глобальные переменные сессии X11
static Display *g_display = NULL;
static Window g_root = 0;

// Обработчик ошибок X11 (чтобы демон не падал при BadMatch и других протокольных ошибках)
static int x11_error_handler(Display *disp, XErrorEvent *err) {
    (void)disp;
    char msg[256];
    XGetErrorText(disp, err->error_code, msg, sizeof(msg));
    fprintf(stderr, "MESEE | BACKEND\t| X11 Error: %s\n", msg);
    return 0;
}

// 1. Инициализация X11-бэкенда
static bool x11_init(void) {
    g_display = XOpenDisplay(NULL);
    if (!g_display) {
        fprintf(stderr, "MESEE | BACKEND\t| Error: Failed to open X11 display.\n");
        return false;
    }

    // DefaultRootWindow объединяет все мониторы в единое виртуальное пространство
    g_root = DefaultRootWindow(g_display);
    
    // Устанавливаем безопасный кастомный обработчик ошибок
    XSetErrorHandler(x11_error_handler);

    printf("MESEE | BACKEND\t| X11 Backend successfully initialized (Multi-monitor supported).\n");
    return true;
}

// 2. Получение текущих координат курсора
static bool x11_get_cursor_pos(int *x, int *y) {
    if (!g_display || !x || !y) return false;

    Window root_return, child_return;
    int root_x, root_y, win_x, win_y;
    unsigned int mask_return;

    // Запрашиваем глобальные координаты мыши относительно Root Window
    if (XQueryPointer(g_display, g_root, &root_return, &child_return, 
                      &root_x, &root_y, &win_x, &win_y, &mask_return)) {
        *x = root_x;
        *y = root_y;
        return true;
    }

    return false;
}

// 3. Формирование буфера пикселей (по запрошенным координатам, а не по текущему курсору)
static bool x11_get_pixels(int x, int y, MESEERectOffsets offsets, MESEEPixelBuffer *out_buffer) {
    if (!g_display || !out_buffer) return false;

    // Вычисляем итоговый размер области на основе запрошенных (x,y) и оффсетов[cite: 3]
    int box_x = x - offsets.left;
    int box_y = y - offsets.top;
    int box_w = offsets.left + offsets.right;
    int box_h = offsets.top + offsets.bottom;

    if (box_w <= 0 || box_h <= 0) return false;

    // Получаем размеры всего виртуального пространства X11 (всех мониторов вместе)
    int screen_w = DisplayWidth(g_display, DefaultScreen(g_display));
    int screen_h = DisplayHeight(g_display, DefaultScreen(g_display));

    // Клиппирование области захвата
    // Если XGetImage попытается выйти за пределы экрана, он выбросит BadMatch. Защищаемся от этого[cite: 2].
    if (box_x < 0) { 
        box_w += box_x; 
        box_x = 0; 
    }
    if (box_y < 0) { 
        box_h += box_y; 
        box_y = 0; 
    }
    if (box_x + box_w > screen_w) { 
        box_w = screen_w - box_x; 
    }
    if (box_y + box_h > screen_h) { 
        box_h = screen_h - box_y; 
    }

    if (box_w <= 0 || box_h <= 0) {
        fprintf(stderr, "MESEE | BACKEND\t| Error: Capture area is out of bounds (x=%d, y=%d, w=%d, h=%d)\n", 
                box_x, box_y, box_w, box_h);
        return false;
    }

    // Захват пикселей
    // Используем стандартный XGetImage (он не захватывает курсор поверх экрана, что нам и нужно)
    XImage *img = XGetImage(g_display, g_root, box_x, box_y, box_w, box_h, AllPlanes, ZPixmap);
    if (!img) return false;

    size_t data_size = (size_t)img->bytes_per_line * img->height;
    
    // Копируем в независимый буфер
    out_buffer->data = (uint8_t *)malloc(data_size);
    if (!out_buffer->data) {
        XDestroyImage(img);
        return false;
    }

    memcpy(out_buffer->data, img->data, data_size);

    // Заполняем структуру ответа (без изменений в заголовке)[cite: 1]
    out_buffer->width = img->width;
    out_buffer->height = img->height;
    out_buffer->stride = img->bytes_per_line;
    out_buffer->data_size = data_size;
    out_buffer->timestamp = (uint64_t)time(NULL);
    out_buffer->internal_ptr = NULL;

    // Очищаем оригинальный XImage
    XDestroyImage(img);
    
    return true;
}

// 4. Освобождение памяти буфера
static void x11_free_pixels(MESEEPixelBuffer *buffer) {
    if (buffer && buffer->data) {
        free(buffer->data);
        buffer->data = NULL;
        buffer->data_size = 0;
        buffer->width = 0;
        buffer->height = 0; // Сохраняем логику очистки полей из прошлой итерации[cite: 3]
    }
}

// 5. Очистка ресурсов X11
static void x11_cleanup(void) {
    if (g_display) {
        XCloseDisplay(g_display);
        g_display = NULL;
        printf("MESEE | BACKEND\t| X11 Backend cleaned up and closed.\n");
    }
}

// Экспортируемая структура API
static MESEEBackendAPI g_x11_api = {
    .name = "MESEE X11 Backend v0.5",
    .init = x11_init,
    .get_cursor_pos = x11_get_cursor_pos,
    .get_pixels = x11_get_pixels,
    .free_pixels = x11_free_pixels,
    .cleanup = x11_cleanup
};

// Главная точка входа для dlsym()
MESEEBackendAPI* mesee_get_backend_api(void) {
    return &g_x11_api;
}