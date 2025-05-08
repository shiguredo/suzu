package suzu

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
)

func TestIsRetryTargetForSpeechToText(t *testing.T) {
	channelID := "test-channel-id"
	connectionID := "test-connection-id"
	sampleRate := uint32(48000)
	channelCount := uint16(2)
	languageCode := "ja-JP"
	onResultFunc := func(context.Context, io.WriteCloser, string, string, string, any) error { return nil }

	testCases := []struct {
		Name         string
		RetryTargets []string
		Error        any
		Expect       bool
	}{
		{
			Name:         "match",
			RetryTargets: []string{"UNEXPECTED-ERROR", "BAD-REQUEST"},
			Error:        errors.New("UNEXPECTED-ERROR"),
			Expect:       true,
		},
		{
			Name:         "mismatch",
			RetryTargets: []string{"UNEXPECTED-ERROR"},
			Error:        errors.New("ERROR"),
			Expect:       false,
		},
		{
			Name:         "code = OutOfRange",
			RetryTargets: []string{"UNEXPECTED-ERROR"},
			Error:        errors.New("code = OutOfRange"),
			Expect:       true,
		},
		{
			Name:         "code = InvalidArgument",
			RetryTargets: []string{"UNEXPECTED-ERROR"},
			Error:        errors.New("code = InvalidArgument"),
			Expect:       true,
		},
		{
			Name:         "code = ResourceExhausted",
			RetryTargets: []string{"UNEXPECTED-ERROR"},
			Error:        errors.New("code = ResourceExhausted"),
			Expect:       true,
		},
		{
			Name:         "codes.OutOfRange",
			RetryTargets: []string{"UNEXPECTED-ERROR"},
			Error:        codes.OutOfRange,
			Expect:       true,
		},
		{
			Name:         "codes.InvalidArgument",
			RetryTargets: []string{"UNEXPECTED-ERROR"},
			Error:        codes.InvalidArgument,
			Expect:       true,
		},
		{
			Name:         "codes.ResourceExhausted",
			RetryTargets: []string{"UNEXPECTED-ERROR"},
			Error:        codes.ResourceExhausted,
			Expect:       true,
		},
		{
			Name:         "Internal",
			RetryTargets: []string{"UNEXPECTED-ERROR", "Internal"},
			Error:        codes.Internal,
			Expect:       true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			config := Config{
				RetryTargets: tc.RetryTargets,
			}

			serviceHandler, err := getServiceHandler("gcp", config, channelID, connectionID, sampleRate, channelCount, languageCode, onResultFunc)
			assert.NoError(t, err)

			assert.Equal(t, tc.Expect, serviceHandler.IsRetryTarget(tc.Error))
		})
	}
}
