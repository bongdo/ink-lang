package main

import (
	"fmt"
	"runtime"
	"sort"
	"sync"
	"time"
)

const (
	W      = 2560
	H      = 1440
	RowLen = W * 4
	Frames = 240
)

type Row [RowLen]byte

func shadeRow(dst *Row, y, t int) {
	for x := 0; x < W; x++ {
		v := byte((x*31 + y*17 + t*13) & 255)
		i := x * 4
		dst[i] = v
		dst[i+1] = v >> 1
		dst[i+2] = 255 - v
		dst[i+3] = 255
	}
}

func p50(xs []float64) float64 { return xs[len(xs)/2] }
func p99(xs []float64) float64 { return xs[len(xs)*99/100] }

func main() {
	px := make([]byte, RowLen*H)
	rows := make([]Row, H)
	times := make([]float64, Frames)
	workers := runtime.GOMAXPROCS(0)
	per := (H + workers - 1) / workers

	for f := 0; f < Frames; f++ {
		start := time.Now()
		var wg sync.WaitGroup
		for w := 0; w < workers; w++ {
			lo := w * per
			hi := lo + per
			if hi > H {
				hi = H
			}
			wg.Add(1)
			go func(lo, hi int) {
				defer wg.Done()
				for y := lo; y < hi; y++ {
					shadeRow(&rows[y], y, f)
				}
			}(lo, hi)
		}
		wg.Wait()
		for y := 0; y < H; y++ {
			copy(px[y*RowLen:], rows[y][:])
		}
		times[f] = float64(time.Since(start).Microseconds()) / 1000
	}

	var sum uint64
	for _, b := range px {
		sum += uint64(b)
	}
	sort.Float64s(times)
	fmt.Println("frames", Frames)
	fmt.Println("p50 ms", p50(times), "p99 ms", p99(times), "max ms", times[Frames-1])
	fmt.Println("sum", sum)
}
