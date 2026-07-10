package outputs

import "sync"

// ShutdownGate signals producers that an output has closed so late Write/WriteEvent
// calls can drop without sending on a closed channel or blocking forever.
type ShutdownGate struct {
	ch   chan struct{}
	once sync.Once
}

func NewShutdownGate() *ShutdownGate {
	return &ShutdownGate{ch: make(chan struct{})}
}

func (g *ShutdownGate) Signal() {
	if g == nil {
		return
	}
	g.once.Do(func() { close(g.ch) })
}

func (g *ShutdownGate) C() <-chan struct{} {
	if g == nil {
		return nil
	}
	return g.ch
}
