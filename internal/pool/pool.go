// Package pool provides a generic, type-safe wrapper around sync.Pool.
// Objects are automatically reset before being returned to the pool.
package pool

import "sync"

// Resetter is satisfied by any type that can reset its own state.
type Resetter interface {
	Reset()
}

// Pool is a type-safe wrapper around sync.Pool.
// T must implement Resetter; Reset() is called on every Put so that
// the next caller always receives a clean object.
type Pool[T Resetter] struct {
	p sync.Pool
}

func New[T Resetter](factory func() T) *Pool[T] {
	return &Pool[T]{
		p: sync.Pool{
			New: func() any { return factory() },
		},
	}
}

func (p *Pool[T]) Get() T {
	return p.p.Get().(T)
}

func (p *Pool[T]) Put(v T) {
	v.Reset()
	p.p.Put(v)
}
