package suzu

import (
	"context"
	"fmt"
	"io"

	"golang.org/x/exp/slices"
)

var (
	ServiceHandlers = make(serviceHandlerFuncs)

	ErrServiceNotFound = fmt.Errorf("SERVICE-NOT-FOUND")
)

type serviceHandlerInterface interface {
	Handle(context.Context, io.Reader) (*io.PipeReader, error)
}

type serviceHandlerFunc func(Config, string, string, uint32, uint16, string) serviceHandlerInterface

type serviceHandlerFuncs map[string]serviceHandlerFunc

func (sh *serviceHandlerFuncs) register(name string, f serviceHandlerFunc) {
	(*sh)[name] = f
}

func (sh *serviceHandlerFuncs) get(name string) (*serviceHandlerFunc, error) {
	h, ok := (*sh)[name]
	if !ok {
		return nil, ErrServiceNotFound
	}
	return &h, nil
}

func (sh *serviceHandlerFuncs) GetNames(exclude []string) []string {
	names := make([]string, 0, len(*sh))
	for name := range *sh {
		if slices.Contains(exclude, name) {
			continue
		}
		names = append(names, name)
	}

	return names
}
