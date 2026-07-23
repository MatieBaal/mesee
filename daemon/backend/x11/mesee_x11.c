#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>

#include "../mesee_backend.h"

// 1. Инициализация X11-бэкенда
static bool x11_init(void) {
    printf("[X11 Backend] Модуль успешно инициализирован.\n");
    return true;
}

// 2. Получение текущих координат курсора (заглушка)
static bool x11_get_cursor_pos(int *x, int *y) {
    if (!x || !y) return false;

    // Возвращаем тестовые координаты
    *x = 960;
    *y = 540;
    return true;
}

// 3. Формирование буфера пикселей (заглушка)
static bool x11_get_pixels(int x, int y, MESEERectOffsets offsets, MESEEPixelBuffer *out_buffer) {
    (void)x; // Временно не используем базовые координаты
    (void)y;

    if (!out_buffer) return false;

    // Вычисляем итоговый размер области
    int width = offsets.left + offsets.right;
    int height = offsets.top + offsets.bottom;

    if (width <= 0 || height <= 0) {
        fprintf(stderr, "[X11 Backend] Ошибка: Некорректные размеры области (%dx%d)\n", width, height);
        return false;
    }

    // В формате RGBA 1 пиксель = 4 байта
    uint32_t stride = (uint32_t)width * 4;
    size_t data_size = (size_t)height * stride;

    // Выделяем память под пиксели
    uint8_t *pixels = (uint8_t *)malloc(data_size);
    if (!pixels) {
        perror("[X11 Backend] malloc");
        return false;
    }

    // Заполняем синим тестовым цветом (RGBA: 0, 150, 255, 255)
    for (size_t i = 0; i < data_size; i += 4) {
        pixels[i + 0] = 0;   // R
        pixels[i + 1] = 150; // G
        pixels[i + 2] = 255; // B
        pixels[i + 3] = 255; // A
    }

    // Заполняем структуру ответа
    out_buffer->width = width;
    out_buffer->height = height;
    out_buffer->stride = stride;
    out_buffer->data = pixels;
    out_buffer->data_size = data_size;
    out_buffer->timestamp = (uint64_t)time(NULL);
    out_buffer->internal_ptr = NULL;

    return true;
}

// 4. Освобождение памяти буфера
static void x11_free_pixels(MESEEPixelBuffer *buffer) {
    if (buffer && buffer->data) {
        free(buffer->data);
        buffer->data = NULL;
        buffer->data_size = 0;
        buffer->width = 0;
        buffer->height = 0;
    }
}

// 5. Очистка ресурсов X11
static void x11_cleanup(void) {
    printf("[X11 Backend] Завершение работы и очистка ресурсов.\n");
}

// Экспортируемая структура API
static MESEEBackendAPI g_x11_api = {
    .name = "MESEE X11 Backend v0.1",
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