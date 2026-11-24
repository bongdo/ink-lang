#define _GNU_SOURCE
#include <wayland-client.h>
#include <xdg-shell-client-protocol.h>
#include <sys/mman.h>
#include <unistd.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>

static struct wl_compositor *compositor;
static struct wl_shm *shm;
static struct xdg_wm_base *wm;
static struct wl_surface *surface;
static struct xdg_surface *xsurf;
static struct xdg_toplevel *toplevel;
static struct wl_buffer *buffer;
static struct wl_callback *frame_cb;

static int width = 800, height = 600;
static int target = 600;
static int frames = 0;
static struct timespec t0, tprev;

#define MAXF 200000
static double dt_us[MAXF];

static void wm_ping(void *d, struct xdg_wm_base *w, uint32_t serial) { (void)d; xdg_wm_base_pong(w, serial); }
static const struct xdg_wm_base_listener wm_listener = { wm_ping };

static void shm_format(void *d, struct wl_shm *s, uint32_t format) { (void)d; (void)s; (void)format; }
static const struct wl_shm_listener shm_listener = { shm_format };

static void registry_global(void *d, struct wl_registry *r, uint32_t name, const char *iface, uint32_t version) {
    (void)d; (void)version;
    if (strcmp(iface, wl_compositor_interface.name) == 0) {
        compositor = wl_registry_bind(r, name, &wl_compositor_interface, 4);
    } else if (strcmp(iface, wl_shm_interface.name) == 0) {
        shm = wl_registry_bind(r, name, &wl_shm_interface, 1);
        wl_shm_add_listener(shm, &shm_listener, NULL);
    } else if (strcmp(iface, xdg_wm_base_interface.name) == 0) {
        wm = wl_registry_bind(r, name, &xdg_wm_base_interface, 1);
        xdg_wm_base_add_listener(wm, &wm_listener, NULL);
    }
}

static void registry_remove(void *d, struct wl_registry *r, uint32_t name) { (void)d; (void)r; (void)name; }
static const struct wl_registry_listener registry_listener = { registry_global, registry_remove };

static struct wl_buffer *make_buffer(int w, int h);

static const struct wl_callback_listener frame_listener;

static void submit_frame(void) {
    frame_cb = wl_surface_frame(surface);
    wl_callback_add_listener(frame_cb, &frame_listener, NULL);
    wl_surface_attach(surface, buffer, 0, 0);
    wl_surface_damage_buffer(surface, 0, 0, width, height);
    wl_surface_commit(surface);
}

static int cmp_double(const void *a, const void *b) {
    double x = *(const double *)a, y = *(const double *)b;
    return (x > y) - (x < y);
}

static double mean(const double *xs, int n) {
    double s = 0;
    for (int i = 0; i < n; i++) s += xs[i];
    return s / n;
}

static void report(void) {
    int n = frames - 1;
    if (n < 1) n = 1;
    struct timespec t1;
    clock_gettime(CLOCK_MONOTONIC, &t1);
    double sec = (t1.tv_sec - t0.tv_sec) + (t1.tv_nsec - t0.tv_nsec) / 1e9;
    double *ys = malloc(sizeof(double) * (size_t)n);
    memcpy(ys, dt_us, sizeof(double) * (size_t)n);
    qsort(ys, (size_t)n, sizeof(double), cmp_double);
    int tail = n / 100;
    if (tail < 1) tail = 1;
    double low1 = mean(ys + n - tail, tail);
    printf("{\"frames\":%d,\"seconds\":%.3f,\"fps\":%.1f,"
           "\"frame_ms_mean\":%.3f,\"p50\":%.3f,\"p95\":%.3f,\"p99\":%.3f,\"low1\":%.3f,\"max\":%.3f,"
           "\"fps_low1\":%.1f}\n",
           frames, sec, frames / sec,
           mean(ys, n) / 1000.0, ys[n / 2] / 1000.0, ys[(n * 95) / 100] / 1000.0,
           ys[(n * 99) / 100] / 1000.0, low1 / 1000.0, ys[n - 1] / 1000.0,
           1e6 / low1);
    fflush(stdout);
}

static void frame_done(void *d, struct wl_callback *cb, uint32_t t) {
    (void)d; (void)t;
    wl_callback_destroy(cb);
    struct timespec now;
    clock_gettime(CLOCK_MONOTONIC, &now);
    double us = (now.tv_sec - tprev.tv_sec) * 1e6 + (now.tv_nsec - tprev.tv_nsec) / 1e3;
    if (frames > 0 && frames < MAXF) dt_us[frames - 1] = us;
    tprev = now;
    frames++;
    if (frames >= target) {
        report();
        exit(0);
    }
    submit_frame();
}
static const struct wl_callback_listener frame_listener = { frame_done };

static void xsurf_configure(void *d, struct xdg_surface *s, uint32_t serial) {
    (void)d;
    xdg_surface_ack_configure(s, serial);
    if (!buffer) {
        buffer = make_buffer(width, height);
        clock_gettime(CLOCK_MONOTONIC, &t0);
        tprev = t0;
        submit_frame();
    }
}
static const struct xdg_surface_listener xsurf_listener = { xsurf_configure };

static void toplevel_configure(void *d, struct xdg_toplevel *t, int32_t w, int32_t h, struct wl_array *states) {
    (void)d; (void)t; (void)states;
    if (w > 0) width = w;
    if (h > 0) height = h;
}
static void toplevel_close(void *d, struct xdg_toplevel *t) { (void)d; (void)t; exit(0); }
static const struct xdg_toplevel_listener toplevel_listener = { toplevel_configure, toplevel_close };

static struct wl_buffer *make_buffer(int w, int h) {
    int stride = w * 4, size = stride * h;
    int fd = memfd_create("fps-shm", MFD_CLOEXEC);
    if (fd < 0 || ftruncate(fd, size) < 0) { perror("shm"); exit(1); }
    uint32_t *px = mmap(NULL, size, PROT_READ | PROT_WRITE, MAP_SHARED, fd, 0);
    if (px == MAP_FAILED) { perror("mmap"); exit(1); }
    for (int i = 0; i < w * h; i++) px[i] = 0xff3060a0;
    struct wl_shm_pool *pool = wl_shm_create_pool(shm, fd, size);
    struct wl_buffer *b = wl_shm_pool_create_buffer(pool, 0, w, h, stride, WL_SHM_FORMAT_XRGB8888);
    wl_shm_pool_destroy(pool);
    munmap(px, (size_t)size);
    close(fd);
    return b;
}

int main(int argc, char **argv) {
    for (int i = 1; i < argc; i++) {
        if (!strcmp(argv[i], "-n") && i + 1 < argc) target = atoi(argv[++i]);
        else if (!strcmp(argv[i], "-w") && i + 1 < argc) width = atoi(argv[++i]);
        else if (!strcmp(argv[i], "-h") && i + 1 < argc) height = atoi(argv[++i]);
        else if (argv[i][0] != '-') target = atoi(argv[i]);
    }
    if (target > MAXF) target = MAXF;

    struct wl_display *display = wl_display_connect(NULL);
    if (!display) { fprintf(stderr, "no display\n"); return 1; }
    struct wl_registry *reg = wl_display_get_registry(display);
    wl_registry_add_listener(reg, &registry_listener, NULL);
    wl_display_roundtrip(display);
    if (!compositor || !shm || !wm) { fprintf(stderr, "missing globals\n"); return 1; }

    surface = wl_compositor_create_surface(compositor);
    xsurf = xdg_wm_base_get_xdg_surface(wm, surface);
    xdg_surface_add_listener(xsurf, &xsurf_listener, NULL);
    toplevel = xdg_surface_get_toplevel(xsurf);
    xdg_toplevel_add_listener(toplevel, &toplevel_listener, NULL);
    xdg_toplevel_set_title(toplevel, "fps-client");
    wl_surface_commit(surface);
    wl_display_roundtrip(display);

    while (wl_display_dispatch(display) != -1) {}
    return 0;
}
