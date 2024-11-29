package suzu

import (
	"context"
	"fmt"
	"io"

	"golang.org/x/exp/slices"
)

var (
	NewServiceHandlerFuncs = make(newServiceHandlerFuncs)

	ErrServiceNotFound = fmt.Errorf("SERVICE-NOT-FOUND")
)

type serviceHandlerInterface interface {
	Handle(context.Context, chan []byte) (*io.PipeReader, error)
	UpdateRetryCount() int
	GetRetryCount() int
	ResetRetryCount() int
}

type newServiceHandlerFunc func(Config, string, string, uint32, uint16, string, any) serviceHandlerInterface

type newServiceHandlerFuncs map[string]newServiceHandlerFunc

func (sh *newServiceHandlerFuncs) register(name string, f newServiceHandlerFunc) {
	(*sh)[name] = f
}

func (sh *newServiceHandlerFuncs) get(name string) (*newServiceHandlerFunc, error) {
	h, ok := (*sh)[name]
	if !ok {
		return nil, ErrServiceNotFound
	}
	return &h, nil
}

func (sh *newServiceHandlerFuncs) GetNames(exclude []string) []string {
	names := make([]string, 0, len(*sh))
	for name := range *sh {
		if slices.Contains(exclude, name) {
			continue
		}
		names = append(names, name)
	}

	return names
}
