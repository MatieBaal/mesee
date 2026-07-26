#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdbool.h>
#include <time.h>
#include <X11/Xlib.h>
#include <X11/Xutil.h>

#include "mesee_backend.h"

static Display * g_display = NULL;
static Window g_root = 0;

static int x11_error_handler
(
    Display * disp,
    XErrorEvent * err
)
{
    (void) disp;

    char msg[256];

    XGetErrorText(disp, err->error_code, msg, sizeof(msg));
    fprintf(stderr, "MESEE | BACKEND\t| X11 Error: %s\n", msg);
    
    return 0;
}

static bool x11_init
(
    void
)
{
    g_display = XOpenDisplay(NULL);
    if (!g_display)
    {
        fprintf(stderr, "MESEE | BACKEND\t| Error: Failed to open X11 display.\n");
        return false;
    }

    g_root = DefaultRootWindow(g_display);
    
    XSetErrorHandler(x11_error_handler);

    printf("MESEE | BACKEND\t| X11 Backend successfully initialized (Multi-monitor supported).\n");
    
    return true;
}

static bool x11_get_cursor_pos
(
    int * x,
    int * y
)
{
    if (!g_display || !x || !y) { return false; }

    Window root_return, child_return;
    int root_x, root_y, win_x, win_y;
    unsigned int mask_return;

    if (XQueryPointer( g_display, g_root, &root_return, &child_return, &root_x, &root_y, &win_x, &win_y, &mask_return))
    {
        *x = root_x;
        *y = root_y;
        return true;
    }

    return false;
}

static bool x11_get_pixels
(
    int x,
    int y,
    MESEERectOffsets offsets,
    MESEEPixelBuffer * out_buffer
)
{
    if (!g_display || !out_buffer) { return false; }

    int box_x = x - offsets.left;
    int box_y = y - offsets.top;
    int box_w = offsets.left + offsets.right;
    int box_h = offsets.top + offsets.bottom;

    if (box_w <= 0 || box_h <= 0) { return false; }

    int screen_w = DisplayWidth(g_display, DefaultScreen(g_display));
    int screen_h = DisplayHeight(g_display, DefaultScreen(g_display));

    if (box_x < 0)
    { 
        box_w += box_x; 
        box_x = 0; 
    }

    if (box_y < 0)
    { 
        box_h += box_y; 
        box_y = 0; 
    }

    if (box_x + box_w > screen_w)
    { 
        box_w = screen_w - box_x; 
    }

    if (box_y + box_h > screen_h)
    { 
        box_h = screen_h - box_y; 
    }

    if (box_w <= 0 || box_h <= 0)
    {
        fprintf(stderr, "MESEE | BACKEND\t| Error: Capture area is out of bounds (x=%d, y=%d, w=%d, h=%d)\n", box_x, box_y, box_w, box_h);
        return false;
    }

    XImage * img = XGetImage(g_display, g_root, box_x, box_y, box_w, box_h, AllPlanes, ZPixmap);
    if (!img) { return false; }

    size_t data_size = (size_t)img->bytes_per_line * img->height;
    
    out_buffer->data = (uint8_t *) malloc(data_size);
    if (!out_buffer->data)
    {
        XDestroyImage(img);
        return false;
    }

    memcpy(out_buffer->data, img->data, data_size);

    out_buffer->width = img->width;
    out_buffer->height = img->height;
    out_buffer->stride = img->bytes_per_line;
    out_buffer->data_size = data_size;
    out_buffer->timestamp = (uint64_t)time(NULL);
    out_buffer->internal_ptr = NULL;

    XDestroyImage(img);
    
    return true;
}

static void x11_free_pixels
(
    MESEEPixelBuffer * buffer
)
{
    if (buffer && buffer->data)
    {
        free(buffer->data);
        buffer->data = NULL;
        buffer->data_size = 0;
        buffer->width = 0;
        buffer->height = 0; // Сохраняем логику очистки полей из прошлой итерации[cite: 3]
    }
}

static void x11_cleanup
(
    void
)
{
    if (g_display)
    {
        XCloseDisplay(g_display);
        g_display = NULL;
        printf("MESEE | BACKEND\t| X11 Backend cleaned up and closed.\n");
    }
}

static MESEEBackendAPI g_x11_api = {
    .name = "MESEE X11 Backend v0.5",
    .init = x11_init,
    .get_cursor_pos = x11_get_cursor_pos,
    .get_pixels = x11_get_pixels,
    .free_pixels = x11_free_pixels,
    .cleanup = x11_cleanup
};

MESEEBackendAPI * mesee_get_backend_api
(
    void
)
{
    return &g_x11_api;
}