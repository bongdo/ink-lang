package inkrt

import (
	"sync"
	"testing"
)

func TestScratchClearsOnReuse(t *testing.T) {
	var s Scratch[int32]
	b := s.Begin()
	xs := b.Slice(4, 4)
	for i := range xs {
		xs[i] = int32(i + 1)
	}
	s.End(b)

	b = s.Begin()
	ys := b.Slice(4, 4)
	for i, v := range ys {
		if v != 0 {
			t.Fatalf("stale value %d at %d", v, i)
		}
	}
	s.End(b)
}

func TestScratchKeepsLengthAndCapacity(t *testing.T) {
	var s Scratch[int32]
	b := s.Begin()
	xs := b.Slice(2, 4)
	if len(xs) != 2 || cap(xs) != 4 {
		t.Fatalf("len=%d cap=%d", len(xs), cap(xs))
	}
	s.End(b)
}

func TestScratchNestedBatches(t *testing.T) {
	var s Scratch[int32]
	outer := s.Begin()
	a := outer.Slice(2, 2)
	inner := s.Begin()
	b := inner.Slice(2, 2)
	if &a[0] == &b[0] {
		t.Fatal("nested batch reused the same memory")
	}
	s.End(inner)
	s.End(outer)
}

func TestScratchNewIsDistinct(t *testing.T) {
	var s Scratch[[2]int32]
	b := s.Begin()
	p := b.New()
	q := b.New()
	if p == q {
		t.Fatal("two new calls returned the same pointer")
	}
	p[0] = 7
	s.End(b)

	b = s.Begin()
	r := b.New()
	if r[0] != 0 {
		t.Fatal("new did not clear reused memory")
	}
	s.End(b)
}

func TestScratchConcurrent(t *testing.T) {
	var s Scratch[int32]
	const n = 8
	ptrs := make([][]int32, n)
	var ready, done sync.WaitGroup
	ready.Add(n)
	done.Add(n)
	release := make(chan struct{})
	for i := 0; i < n; i++ {
		go func(i int) {
			defer done.Done()
			b := s.Begin()
			xs := b.Slice(2, 2)
			xs[0] = int32(i + 1)
			ptrs[i] = xs
			ready.Done()
			<-release
			s.End(b)
		}(i)
	}
	ready.Wait()
	for i, xs := range ptrs {
		if xs[0] != int32(i+1) {
			t.Fatalf("batch %d saw %d", i, xs[0])
		}
	}
	close(release)
	done.Wait()
}

func TestScratchReleaseDropsBuffers(t *testing.T) {
	var s Scratch[int32]
	b := s.Begin()
	b.Slice(8, 8)
	s.End(b)

	s.Release()

	b = s.Begin()
	xs := b.Slice(4, 4)
	if cap(xs) != 4 {
		t.Fatalf("released buffer reused: cap=%d", cap(xs))
	}
	s.End(b)
}
