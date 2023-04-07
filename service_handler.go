package suzu

import (
	"context"
	"io"
)

var (
	ServiceHandlerNames = serviceHandlers{}
)

type serviceHandlerInterface interface {
	Handle(context.Context, io.Reader) (*io.PipeReader, error)
}

type serviceHandlers []string

func (sh *serviceHandlers) register(name string) {
	*sh = append(*sh, name)
}

func (sh *serviceHandlers) GetNames() []string {
	return *sh
}
