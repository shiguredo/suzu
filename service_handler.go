package suzu

import (
	"context"
	"fmt"
	"io"
)

var (
	ServiceHandlers = NewServiceHandlers()
)

type serviceHandler func(ctx context.Context, conn io.Reader, args HandlerArgs) (*io.PipeReader, error)

type serviceHandlers struct {
	Handlers map[string]serviceHandler
}

func NewServiceHandlers() serviceHandlers {
	return serviceHandlers{
		Handlers: make(map[string]serviceHandler),
	}
}

func (sh *serviceHandlers) registerHandler(name string, handler serviceHandler) {
	sh.Handlers[name] = handler
}

func (sh *serviceHandlers) getServiceHandler(name string) (serviceHandler, error) {
	h, ok := sh.Handlers[name]
	if !ok {
		return nil, fmt.Errorf("UNREGISTERED-SERVICE: %s", name)
	}

	return h, nil
}

func (sh *serviceHandlers) GetNames() []string {
	var names []string
	for name := range sh.Handlers {
		names = append(names, name)
	}

	return names
}
