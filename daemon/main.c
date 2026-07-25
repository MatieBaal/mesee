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

#pragma pack(push, 1)

typedef struct {
    uint8_t status; 
    int32_t x;      
    int32_t y;      
} ResponseCursor;   

typedef struct {
    uint8_t status;
    int32_t width;
    int32_t height;
    uint32_t stride;
    uint64_t timestamp;
    uint32_t data_size;
} ResponsePixelHeader;

#pragma pack(pop)

static int g_server_fd = -1;
static void *g_dl_handle = NULL;
static MESEEBackendAPI *g_api = NULL;

void cleanup_and_exit
(
    int code
)
{
    printf("MESEE | DAEMON\t| Shutting down...\n");
    
    if (g_api && g_api->cleanup)
    {
        g_api->cleanup();
    }
    
    if (g_dl_handle)
    {
        dlclose(g_dl_handle);
    }
    
    if (g_server_fd != -1)
    {
        close(g_server_fd);
    }

    unlink(SOCKET_PATH);
    
    exit(code);
}

void signal_handler
(
    int sig
)
{
    (void) sig;

    cleanup_and_exit(0);
}

const char * detect_backend_library
(
    void
)
{
    const char * session = getenv("XDG_SESSION_TYPE");
    const char * wayland_display = getenv("WAYLAND_DISPLAY");

    if ((session && strcmp(session, "wayland") == 0) || wayland_display != NULL)
    {
        return "./libmesee_backend_wayland.so";
    }

    return "./libmesee_backend_x11.so";
}

static bool read_exact
(
    int fd,
    void * buf,
    size_t count
)
{
    size_t total = 0;
    uint8_t * p = (uint8_t *) buf;
    
    while (total < count)
    {
        ssize_t n = read(fd, p + total, count - total);
        if (n <= 0) { return false; }
        total += n;
    }

    return true;
}

static bool write_all
(
    int fd,
    const void * buf,
    size_t count
)
{
    size_t total = 0;
    const uint8_t *p = (const uint8_t*)buf;
    
    while (total < count)
    {
        ssize_t n = write(fd, p + total, count - total);
        if (n <= 0) { return false; }
        total += n;
    }

    return true;
}

void handle_client
(
    int client_fd
)
{
    while (1)
    {
        uint8_t cmd = 0;
        
        if (!read_exact(client_fd, &cmd, 1))
        {
            break;
        }

        if (cmd == 1) // GetCursorPos
        {
            ResponseCursor res = { .status = 1, .x = 0, .y = 0 };
            
            if (g_api->get_cursor_pos(&res.x, &res.y))
            {
                res.status = 0;
            }
            
            if (!write_all(client_fd, &res, sizeof(res))) { break; }

        }
        else if (cmd == 2) // GetPixels
        { 
            MESEERectOffsets offsets;
            int x = 0, y = 0;

            if (!read_exact(client_fd, &x, sizeof(x)))
            {
                break;
            }

            if (!read_exact(client_fd, &y, sizeof(y)))
            {
                break;
            }
            
            if (!read_exact(client_fd, &offsets, sizeof(offsets)))
            {
                break;
            }

            ResponsePixelHeader res = {0};

            MESEEPixelBuffer pb = {0};

            if (g_api->get_pixels(x, y, offsets, &pb))
            {
                res.status = 0;
                res.width = pb.width;
                res.height = pb.height;
                res.stride = pb.stride;
                res.timestamp = pb.timestamp;
                res.data_size = (uint32_t)pb.data_size;

                if (write_all(client_fd, &res, sizeof(res)))
                {
                    if (pb.data && pb.data_size > 0)
                    {
                        write_all(client_fd, pb.data, pb.data_size);
                    }
                }

                g_api->free_pixels(&pb);
            }
            else
            {
                res.status = 1;
                write_all(client_fd, &res, sizeof(res));
            }
        }
    }
}

int main
(
    void
)
{
    signal(SIGINT, signal_handler);  
    signal(SIGTERM, signal_handler); 
    signal(SIGPIPE, SIG_IGN);        

    const char * lib_path = detect_backend_library();
    printf("MESEE | DAEMON\t| Session found, loading: %s\n", lib_path);

    g_dl_handle = dlopen(lib_path, RTLD_LAZY);
    if (!g_dl_handle)
    {
        fprintf(stderr, "MESEE | DAEMON\t| Error: dlopen: %s\n", dlerror());
        return 1;
    }

    BackendEntryFunc get_api = (BackendEntryFunc)dlsym(g_dl_handle, BACKEND_ENTRY_POINT);
    if (!get_api)
    {
        fprintf(stderr, "MESEE | DAEMON\t| Error: dlsym: %s\n", dlerror());
        cleanup_and_exit(1);
    }

    g_api = get_api();
    printf("MESEE | DAEMON\t| Connected backend: '%s'\n", g_api->name);

    if (!g_api->init())
    {
        fprintf(stderr, "MESEE | DAEMON\t| Error: Failed to initialize backend.\n");
        cleanup_and_exit(1);
    }

    g_server_fd = socket(AF_UNIX, SOCK_STREAM, 0);
    if (g_server_fd == -1)
    {
        fprintf(stderr, "MESEE | DAEMON\t| Error: socket: %s\n", strerror(errno));
        cleanup_and_exit(1);
    }

    struct sockaddr_un addr;
    memset(&addr, 0, sizeof(addr));
    addr.sun_family = AF_UNIX;
    strncpy(addr.sun_path, SOCKET_PATH, sizeof(addr.sun_path) - 1);

    unlink(SOCKET_PATH); 

    if (bind(g_server_fd, (struct sockaddr*)&addr, sizeof(addr)) == -1)
    {
        fprintf(stderr, "MESEE | DAEMON\t| Error: bind: %s\n", strerror(errno));
        cleanup_and_exit(1);
    }

    if (listen(g_server_fd, 10) == -1)
    {
        fprintf(stderr, "MESEE | DAEMON\t| Error: listen: %s\n", strerror(errno));
        cleanup_and_exit(1);
    }

    printf("MESEE | DAEMON\t| Ready to accept connections through %s\n", SOCKET_PATH);

    while (1)
    {
        int client_fd = accept(g_server_fd, NULL, NULL);
        if (client_fd == -1)
        {
            if (errno == EINTR) continue; 
            fprintf(stderr, "MESEE | DAEMON\t| Error: accept: %s\n", strerror(errno));
            break;
        }

        handle_client(client_fd);
        close(client_fd);
    }

    cleanup_and_exit(0);
    
    return 0;
}