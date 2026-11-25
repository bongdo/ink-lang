package guard

import "sync"

type Lock struct {
	mu sync.Mutex
}

func New() *Lock {
	return &Lock{}
}

func (l *Lock) Lock() {
	l.mu.Lock()
}

func (l *Lock) Unlock() {
	l.mu.Unlock()
}
