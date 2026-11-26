package inkrt

import (
	"fmt"
	"io/fs"
	"strconv"
	"strings"
	"sync"
)

var (
	assetsMu sync.RWMutex
	assets   fs.FS
)

func SetAssets(fsys fs.FS) {
	assetsMu.Lock()
	assets = fsys
	assetsMu.Unlock()
}

func Asset(name string) ([]byte, bool) {
	assetsMu.RLock()
	fsys := assets
	assetsMu.RUnlock()
	if fsys == nil {
		return nil, false
	}
	data, err := fs.ReadFile(fsys, strings.TrimPrefix(name, "/"))
	if err != nil {
		return nil, false
	}
	return data, true
}

func Sprint(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case int:
		return strconv.Itoa(x)
	case int8:
		return strconv.FormatInt(int64(x), 10)
	case int16:
		return strconv.FormatInt(int64(x), 10)
	case int32:
		return strconv.FormatInt(int64(x), 10)
	case int64:
		return strconv.FormatInt(x, 10)
	case uint:
		return strconv.FormatUint(uint64(x), 10)
	case uint8:
		return strconv.FormatUint(uint64(x), 10)
	case uint16:
		return strconv.FormatUint(uint64(x), 10)
	case uint32:
		return strconv.FormatUint(uint64(x), 10)
	case uint64:
		return strconv.FormatUint(x, 10)
	case float32:
		return strconv.FormatFloat(float64(x), 'g', -1, 32)
	case float64:
		return strconv.FormatFloat(x, 'g', -1, 64)
	case bool:
		return strconv.FormatBool(x)
	case error:
		return x.Error()
	default:
		return fmt.Sprint(x)
	}
}

type DeferStack struct {
	fns []func()
}

func (d *DeferStack) Mark() int {
	return len(d.fns)
}

func (d *DeferStack) Push(fn func()) {
	d.fns = append(d.fns, fn)
}

func (d *DeferStack) RunTo(mark int) {
	if mark < 0 {
		mark = 0
	}
	if mark > len(d.fns) {
		mark = len(d.fns)
	}
	pending := d.fns[mark:]
	d.fns = d.fns[:mark]
	for i := len(pending) - 1; i >= 0; i-- {
		pending[i]()
	}
}

func Assert(ok bool, pos string) {
	if !ok {
		panic("assert failed at " + pos)
	}
}

func AssertEq[T comparable](got, want T, pos string) {
	if got != want {
		panic(fmt.Sprintf("assert_eq failed at %s: got %v, want %v", pos, got, want))
	}
}

func Has[K comparable, V any](m map[K]V, k K) bool {
	_, ok := m[k]
	return ok
}

type ParError struct {
	Index int
	Err   error
}

type ParErrors struct {
	mu   sync.Mutex
	errs []ParError
}

func (p *ParErrors) Add(index int, err error) {
	if err == nil {
		return
	}
	p.mu.Lock()
	p.errs = append(p.errs, ParError{Index: index, Err: err})
	p.mu.Unlock()
}

func (p *ParErrors) First() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.errs) == 0 {
		return nil
	}
	best := -1
	for i, e := range p.errs {
		if e.Index < 0 {
			continue
		}
		if best < 0 || e.Index < p.errs[best].Index {
			best = i
		}
	}
	if best >= 0 {
		return p.errs[best].Err
	}
	return p.errs[0].Err
}

type Batch[T any] struct {
	buf []T
	off int
}

func (b *Batch[T]) Slice(n, cap int) []T {
	s, fresh := b.alloc(n, cap)
	if !fresh {
		clear(s[:cap])
	}
	return s
}

func (b *Batch[T]) SliceUninit(n, cap int) []T {
	s, _ := b.alloc(n, cap)
	return s
}

func (b *Batch[T]) alloc(n, cap int) ([]T, bool) {
	if cap < n {
		cap = n
	}
	fresh := false
	if b.off+cap > len(b.buf) {
		size := 2 * len(b.buf)
		if size < b.off+cap {
			size = b.off + cap
		}
		next := make([]T, size)
		copy(next, b.buf[:b.off])
		b.buf = next
		fresh = true
	}
	start := b.off
	b.off = start + cap
	return b.buf[start : start+n : start+cap], fresh
}

func (b *Batch[T]) New() *T {
	s := b.Slice(1, 1)
	return &s[0]
}

func (b *Batch[T]) reset() {
	b.off = 0
}

type Scratch[T any] struct {
	mu   sync.Mutex
	free []*Batch[T]
}

func (s *Scratch[T]) Begin() *Batch[T] {
	s.mu.Lock()
	var b *Batch[T]
	if n := len(s.free); n > 0 {
		b = s.free[n-1]
		s.free = s.free[:n-1]
	}
	s.mu.Unlock()
	if b == nil {
		b = &Batch[T]{}
	}
	return b
}

func (s *Scratch[T]) End(b *Batch[T]) {
	b.reset()
	s.mu.Lock()
	s.free = append(s.free, b)
	s.mu.Unlock()
}

func (s *Scratch[T]) Release() {
	s.mu.Lock()
	s.free = nil
	s.mu.Unlock()
}
