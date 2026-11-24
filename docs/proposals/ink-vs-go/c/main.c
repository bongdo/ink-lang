#define _GNU_SOURCE
#include <pthread.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>

#define W 2560
#define H 1440
#define ROWLEN (W * 4)
#define FRAMES 240
#define MAXT 8

static uint8_t *px;

typedef struct {
    int lo, hi;
    pthread_barrier_t *start, *end;
} Worker;

static inline void shade_rows(int lo, int hi, int t) {
    for (int y = lo; y < hi; y++) {
        uint8_t *row = px + (size_t)y * ROWLEN;
        for (int x = 0; x < W; x++) {
            uint8_t v = (uint8_t)((unsigned)(x * 31 + y * 17 + t * 13) & 255u);
            int i = x * 4;
            row[i] = v;
            row[i + 1] = v >> 1;
            row[i + 2] = (uint8_t)(255 - v);
            row[i + 3] = 255;
        }
    }
}

static void *worker(void *arg) {
    Worker *w = arg;
    for (int f = 0; f < FRAMES; f++) {
        pthread_barrier_wait(w->start);
        shade_rows(w->lo, w->hi, f);
        pthread_barrier_wait(w->end);
    }
    return NULL;
}

static double now_ms(void) {
    struct timespec ts;
    clock_gettime(CLOCK_MONOTONIC, &ts);
    return (double)ts.tv_sec * 1000.0 + (double)ts.tv_nsec / 1e6;
}

static int cmp_double(const void *a, const void *b) {
    double x = *(const double *)a, y = *(const double *)b;
    return (x > y) - (x < y);
}

static void flush_cache(int *buf, size_t n) {
    for (size_t i = 0; i < n; i += 16) buf[i] = (int)i;
}

int main(int argc, char **argv) {
    int threads = argc > 1 ? atoi(argv[1]) : 4;
    int cold = argc > 2 && strcmp(argv[2], "cold") == 0;
    if (threads < 1) threads = 1;
    if (threads > MAXT) threads = MAXT;
    if (posix_memalign((void **)&px, 64, (size_t)ROWLEN * H) != 0) return 1;
    memset(px, 0, (size_t)ROWLEN * H);

    int *flush = NULL;
    size_t flushn = 0;
    if (cold) {
        flushn = 32u << 20; /* 128 MB of ints */
        flush = malloc(flushn * sizeof(int));
        if (!flush) return 1;
    }

    double times[FRAMES];
    int per = (H + threads - 1) / threads;

    if (threads == 1) {
        for (int f = 0; f < FRAMES; f++) {
            if (cold) flush_cache(flush, flushn);
            double t0 = now_ms();
            shade_rows(0, H, f);
            times[f] = now_ms() - t0;
        }
    } else {
        pthread_t th[MAXT];
        Worker ws[MAXT];
        pthread_barrier_t start, end;
        pthread_barrier_init(&start, NULL, threads + 1);
        pthread_barrier_init(&end, NULL, threads + 1);
        for (int i = 0; i < threads; i++) {
            ws[i].lo = i * per;
            ws[i].hi = ws[i].lo + per;
            if (ws[i].hi > H) ws[i].hi = H;
            ws[i].start = &start;
            ws[i].end = &end;
            pthread_create(&th[i], NULL, worker, &ws[i]);
        }
        for (int f = 0; f < FRAMES; f++) {
            if (cold) flush_cache(flush, flushn);
            double t0 = now_ms();
            pthread_barrier_wait(&start);
            pthread_barrier_wait(&end);
            times[f] = now_ms() - t0;
        }
        for (int i = 0; i < threads; i++) pthread_join(th[i], NULL);
    }

    uint64_t sum = 0;
    for (size_t i = 0; i < (size_t)ROWLEN * H; i++) sum += px[i];
    qsort(times, FRAMES, sizeof(double), cmp_double);
    double avg = 0;
    for (int i = 0; i < FRAMES; i++) avg += times[i];
    printf("threads %d %s avg %.3f  p50 %.3f  p99 %.3f  max %.3f ms  sum %llu\n",
           threads, cold ? "cold" : "hot", avg / FRAMES, times[FRAMES / 2],
           times[FRAMES * 99 / 100], times[FRAMES - 1], (unsigned long long)sum);
    return 0;
}
