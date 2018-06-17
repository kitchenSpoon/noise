package pool

import (
	"testing"
	"time"
)

type Foo struct {
	closed bool
}

func (f *Foo) Close() {
	f.closed = true
}

func TestSingleThreadedPool(t *testing.T) {
	foo := &Foo {}

	p := NewPool()
	p.Add("foo", foo, ObjectConfig { AliveFor: time.Duration(100 * time.Millisecond) })
	f, ok := p.Get("foo")
	if !ok || f.(*Foo) != foo {
		panic("Invalid value (1)")
	}

	p.Gc()
	f, ok = p.Get("foo")
	if !ok || f.(*Foo) != foo {
		panic("Invalid value (2)")
	}

	time.Sleep(100 * time.Millisecond)
	p.Gc()
	_, ok = p.Get("foo")
	if ok {
		panic("Invalid value (3)")
	}

	p.Add("foo", foo, ObjectConfig { AliveFor: time.Duration(100 * time.Millisecond) })
	time.Sleep(50 * time.Millisecond)
	p.Gc()
	p.GetAndRenew("foo")
	time.Sleep(50 * time.Millisecond)
	p.Gc()
	f, ok = p.Get("foo")
	if !ok || f.(*Foo) != foo {
		panic("Invalid value (4)")
	}
}
