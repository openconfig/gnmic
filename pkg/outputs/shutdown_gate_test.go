package outputs

import (
	"testing"
	"time"
)

func TestShutdownGate_SignalOnce(t *testing.T) {
	g := NewShutdownGate()
	g.Signal()
	g.Signal()

	select {
	case <-g.C():
	case <-time.After(time.Second):
		t.Fatal("expected gate to be closed")
	}
}
