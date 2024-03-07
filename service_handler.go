package suzu

import (
	"context"
	"fmt"
	"io"

	"golang.org/x/exp/slices"
)

var (
	ServiceHandlers = NewServiceHandlers()

	ErrServiceNotFound = fmt.Errorf("SERVICE-NOT-FOUND")
)

type serviceHandlerInterface interface {
	Handle(context.Context, io.Reader) (*io.PipeReader, error)
	UpdateRetryCount() int
	GetRetryCount() int
	ResetRetryCount() int
	New(Config, string, string, uint32, uint16, string, any) serviceHandlerInterface
}

type serviceHandlers struct {
	Handlers map[string]serviceHandlerInterface
}

func NewServiceHandlers() serviceHandlers {
	return serviceHandlers{
		Handlers: make(map[string]serviceHandlerInterface),
	}
}

func (h *serviceHandlers) Register(name string, f serviceHandlerInterface) {
	h.Handlers[name] = f
}

func (h *serviceHandlers) Get(name string) (serviceHandlerInterface, error) {
	handler, ok := h.Handlers[name]
	if !ok {
		return nil, ErrServiceNotFound
	}
	return handler, nil
}

func (h *serviceHandlers) GetNames(exclude []string) []string {
	handlers := h.Handlers
	names := make([]string, 0, len(handlers))
	for name := range handlers {
		if slices.Contains(exclude, name) {
			continue
		}
		names = append(names, name)
	}

	return names
}
