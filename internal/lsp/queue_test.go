package lsp

import (
	"sync"
	"testing"
	"time"
)

func TestQueueRunsLatestOnly(t *testing.T) {
	release := make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	started := make(chan int, 8)
	published := make(chan int, 8)
	var mu sync.Mutex
	var ran []int
	record := func(version int) func() any {
		return func() any {
			mu.Lock()
			ran = append(ran, version)
			mu.Unlock()
			return version
		}
	}
	q := newQueue(func(version int) {
		started <- version
		if version == 1 {
			<-release
		}
	}, func(version int, _ any) {
		published <- version
	})
	defer q.close()
	defer unblock()

	if !q.submit(1, record(1)) {
		t.Fatal("submit 1 rejected")
	}
	if version := <-started; version != 1 {
		t.Fatalf("started %d, want 1", version)
	}
	for _, version := range []int{2, 3, 4} {
		if !q.submit(version, record(version)) {
			t.Fatalf("submit %d rejected", version)
		}
	}
	unblock()
	select {
	case version := <-published:
		if version != 4 {
			t.Fatalf("published %d, want 4", version)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no publication")
	}
	select {
	case version := <-published:
		t.Fatalf("extra publication %d", version)
	default:
	}
	mu.Lock()
	defer mu.Unlock()
	if len(ran) != 2 || ran[0] != 1 || ran[1] != 4 {
		t.Fatalf("ran %v, want [1 4]", ran)
	}
}

func TestQueueDropsKnownVersion(t *testing.T) {
	published := make(chan int, 4)
	q := newQueue(nil, func(version int, _ any) { published <- version })
	defer q.close()

	var mu sync.Mutex
	runs := 0
	run := func() any {
		mu.Lock()
		runs++
		mu.Unlock()
		return nil
	}
	if !q.submit(5, run) {
		t.Fatal("submit 5 rejected")
	}
	select {
	case version := <-published:
		if version != 5 {
			t.Fatalf("published %d, want 5", version)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no publication")
	}
	if q.submit(3, run) {
		t.Fatal("stale submit accepted")
	}
	mu.Lock()
	defer mu.Unlock()
	if runs != 1 {
		t.Fatalf("runs = %d, want 1", runs)
	}
}

func TestQueueCloseStopsWorker(t *testing.T) {
	release := make(chan struct{})
	started := make(chan struct{})
	published := make(chan int, 1)
	q := newQueue(func(int) {
		close(started)
		<-release
	}, func(version int, _ any) { published <- version })
	if !q.submit(1, func() any { return nil }) {
		t.Fatal("submit 1 rejected")
	}
	<-started
	done := make(chan struct{})
	go func() {
		q.close()
		close(done)
	}()
	q.mu.Lock()
	for !q.closed {
		q.cond.Wait()
	}
	q.mu.Unlock()
	close(release)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("worker did not stop")
	}
	select {
	case version := <-published:
		t.Fatalf("published %d after close", version)
	default:
	}
}
