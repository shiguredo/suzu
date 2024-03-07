package suzu

import (
	"context"
	"fmt"
	"io"
	"sync"

	"golang.org/x/exp/slices"
)

var (
	ServiceHandlerMakers = NewServiceHandlerMakers()

	ErrServiceNotFound = fmt.Errorf("SERVICE-NOT-FOUND")
)

type serviceHandlerMakerInterface interface {
	New(Config, string, string, uint32, uint16, string, any) serviceHandlerInterface
}

type serviceHandlerInterface interface {
	Handle(context.Context, io.Reader) (*io.PipeReader, error)
	UpdateRetryCount() int
	GetRetryCount() int
	ResetRetryCount() int
}

type serviceHandlerMakers struct {
	Makers map[string]serviceHandlerMakerInterface
}

func NewServiceHandlerMakers() serviceHandlerMakers {
	return serviceHandlerMakers{
		Makers: make(map[string]serviceHandlerMakerInterface),
	}
}

func (h *serviceHandlerMakers) Register(name string, f serviceHandlerMakerInterface) {
	h.Makers[name] = f
}

func (h *serviceHandlerMakers) Get(name string) (serviceHandlerMakerInterface, error) {
	maker, ok := h.Makers[name]
	if !ok {
		return nil, ErrServiceNotFound
	}
	return maker, nil
}

func (h *serviceHandlerMakers) GetNames(exclude []string) []string {
	makers := h.Makers
	names := make([]string, 0, len(makers))
	for name := range makers {
		if slices.Contains(exclude, name) {
			continue
		}
		names = append(names, name)
	}

	return names
}

type RetryCounter struct {
	Count int
	mu    sync.Mutex
}

func NewRetryCounter() RetryCounter {
	return RetryCounter{
		Count: 0,
	}
}

func (r *RetryCounter) Update() int {
	defer r.mu.Unlock()
	r.mu.Lock()
	r.Count++
	return r.Count
}

func (r *RetryCounter) Get() int {
	return r.Count
}

func (r *RetryCounter) Reset() int {
	defer r.mu.Unlock()
	r.mu.Lock()
	r.Count = 0
	return r.Count
}
