package suzu

import (
	"context"
	"fmt"
	"io"
	"strings"

	"golang.org/x/exp/slices"
)

var (
	NewServiceHandlerFuncs = make(newServiceHandlerFuncs)

	ErrServiceNotFound = fmt.Errorf("SERVICE-NOT-FOUND")
)

type serviceHandlerInterface interface {
	Handle(context.Context, chan any, soraHeader) (*io.PipeReader, error)
	UpdateRetryCount() int
	GetRetryCount() int
	ResetRetryCount() int
	IsRetryTarget(any) bool
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

// message が retry_targets に含まれているかどうかを判定する
func isRetryTargetByConfig(config Config, message string) bool {
	// retry_targets が設定されていない場合は固定のエラー判定処理へ
	retryTargets := config.RetryTargets

	// retry_targets が設定されている場合は、リトライ対象のエラーかどうかを判定する
	if len(retryTargets) > 0 {
		for _, target := range retryTargets {
			if strings.Contains(message, target) {
				return true
			}
		}
	}

	return false
}
