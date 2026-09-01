package operations

import (
	"context"
	"log"
	"sync"
	"time"
)

type Reconciler interface {
	Kind() string
	Reconcile(context.Context) error
}
type Coordinator struct {
	interval time.Duration
	mu       sync.RWMutex
	items    map[string]Reconciler
}

func New(interval time.Duration) *Coordinator {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	return &Coordinator{interval: interval, items: map[string]Reconciler{}}
}
func (c *Coordinator) Register(r Reconciler) {
	if r == nil || r.Kind() == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[r.Kind()] = r
}
func (c *Coordinator) ReconcileOnce(ctx context.Context) {
	c.mu.RLock()
	xs := make([]Reconciler, 0, len(c.items))
	for _, r := range c.items {
		xs = append(xs, r)
	}
	c.mu.RUnlock()
	for _, r := range xs {
		_ = r.Reconcile(ctx)
	}
}
func (c *Coordinator) Run(ctx context.Context) {
	run := func() {
		c.mu.RLock()
		xs := make([]Reconciler, 0, len(c.items))
		for _, r := range c.items {
			xs = append(xs, r)
		}
		c.mu.RUnlock()
		for _, r := range xs {
			if e := r.Reconcile(ctx); e != nil && ctx.Err() == nil {
				log.Printf("operation reconcile %s: %v", r.Kind(), e)
			}
		}
	}
	run()
	go func() {
		t := time.NewTicker(c.interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				run()
			}
		}
	}()
}
