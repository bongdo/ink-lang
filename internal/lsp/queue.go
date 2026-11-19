package lsp

import "sync"

type job struct {
	version int
	run     func() any
}

type queue struct {
	mu      sync.Mutex
	cond    *sync.Cond
	hook    func(version int)
	publish func(version int, result any)
	job     *job
	latest  int
	done    int
	closed  bool
	quit    chan struct{}
}

func newQueue(hook func(version int), publish func(version int, result any)) *queue {
	q := &queue{hook: hook, publish: publish, quit: make(chan struct{})}
	q.cond = sync.NewCond(&q.mu)
	go q.loop()
	return q
}

func (q *queue) submit(version int, run func() any) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed || version <= q.latest {
		return false
	}
	q.latest = version
	q.job = &job{version: version, run: run}
	q.cond.Signal()
	return true
}

func (q *queue) close() {
	q.mu.Lock()
	q.closed = true
	q.cond.Broadcast()
	q.mu.Unlock()
	<-q.quit
}

func (q *queue) loop() {
	defer close(q.quit)
	for {
		j := q.next()
		if j == nil {
			return
		}
		if q.hook != nil {
			q.hook(j.version)
		}
		result := j.run()
		if !q.current(j.version) {
			continue
		}
		q.publish(j.version, result)
	}
}

func (q *queue) next() *job {
	q.mu.Lock()
	defer q.mu.Unlock()
	for q.job == nil && !q.closed {
		q.cond.Wait()
	}
	if q.closed {
		return nil
	}
	j := q.job
	q.job = nil
	return j
}

func (q *queue) current(version int) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed || version != q.latest || version <= q.done {
		return false
	}
	q.done = version
	return true
}
