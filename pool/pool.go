package pool

import (
	"time"
	"sync"
	"sync/atomic"
	"unsafe"
)

type Poolable interface {
	Close()
}

type Object struct {
	expiresAt *time.Time
	handle Poolable
	config ObjectConfig
}

type ObjectConfig struct {
	AliveFor time.Duration
}

type Pool struct {
	objects sync.Map
}

func NewPool() *Pool {
	return &Pool {
		objects: sync.Map {},
	}
}

func (p *Pool) Add(key string, value Poolable, cfg ObjectConfig) {
	newExpTime := time.Now().Add(cfg.AliveFor)

	obj := &Object {
		expiresAt: &newExpTime,
		handle: value,
		config: cfg,
	}

	p.objects.Store(key, obj)
}

func (p *Pool) getObject(key string) (*Object, bool) {
	rawVal, ok := p.objects.Load(key)
	if !ok {
		return nil, false
	}

	return rawVal.(*Object), true
}

func (p *Pool) Get(key string) (Poolable, bool) {
	obj, ok := p.getObject(key)
	if !ok {
		return nil, false
	}

	return obj.handle, true
}

func (p *Pool) GetAndRenew(key string) (Poolable, bool) {
	obj, ok := p.getObject(key)
	if !ok {
		return nil, false
	}

	newExpTime := new(time.Time)
	*newExpTime = time.Now().Add(obj.config.AliveFor)

	atomic.SwapPointer(
		(*unsafe.Pointer)(unsafe.Pointer(&obj.expiresAt)),
		unsafe.Pointer(newExpTime),
	)

	return obj.handle, true
}

// TODO: atomic
func (p *Pool) Remove(key string) {
	p.objects.Delete(key)
}

func (p *Pool) Gc() {
	expired := make([]string, 0)
	curTime := time.Now()

	p.objects.Range(func (key interface{}, rawVal interface{}) bool {
		obj := rawVal.(*Object)
		if obj.expiresAt.Before(curTime) {
			expired = append(expired, key.(string))
		}
		return true
	})

	for _, k := range expired {
		obj, ok := p.Get(k)
		if ok {
			p.objects.Delete(k)
			obj.Close()
		}
	}
}
