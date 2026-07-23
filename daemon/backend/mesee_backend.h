#ifndef MESEE_BACKEND_H
#define MESEE_BACKEND_H

#include <stdint.h>
#include <stdbool.h>
#include <stddef.h>

typedef struct {
    int width;
    int height;
    uint32_t stride;
    uint8_t *data;
    size_t data_size;
    uint64_t timestamp;
    void *internal_ptr;
} MESEEPixelBuffer;

typedef struct {
    int top;
    int bottom;
    int left;
    int right;
} MESEERectOffsets;

typedef struct {
    const char *name;
    bool (*init)(void);
    bool (*get_cursor_pos)(int *x, int *y); 
    bool (*get_pixels)(int x, int y, MESEERectOffsets offsets, MESEEPixelBuffer *out_buffer);
    void (*free_pixels)(MESEEPixelBuffer *buffer);
    void (*cleanup)(void);
} MESEEBackendAPI;

#define BACKEND_ENTRY_POINT "mesee_get_backend_api"
typedef MESEEBackendAPI* (*BackendEntryFunc)(void);

#endif // MESEE_BACKEND_H