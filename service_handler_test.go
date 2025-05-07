package suzu

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsRetryTarget(t *testing.T) {
	channelID := "test-channel-id"
	connectionID := "test-connection-id"
	sampleRate := uint32(48000)
	channelCount := uint16(2)
	languageCode := "ja-JP"
	onResultFunc := func(context.Context, io.WriteCloser, string, string, string, any) error { return nil }

	testCases := []struct {
		Name         string
		RetryTargets string
		Error        any
		Expect       bool
	}{
		{
			Name:         "retry target is empty",
			RetryTargets: "",
			Error:        errors.New(""),
			Expect:       false,
		},
		{
			Name:         "match",
			RetryTargets: "UNEXPECTED-ERROR,BAD-REQUEST",
			Error:        errors.New("UNEXPECTED-ERROR"),
			Expect:       true,
		},
		{
			Name:         "match",
			RetryTargets: "UNEXPECTED-ERROR,BAD-REQUEST",
			Error:        errors.New("BAD-REQUEST"),
			Expect:       true,
		},
		{
			Name:         "match",
			RetryTargets: "UNEXPECTED ERROR,BAD REQUEST",
			Error:        errors.New("BAD REQUEST"),
			Expect:       true,
		},
		{
			Name:         "partial match",
			RetryTargets: "UNEXPECTED-ERROR,BAD-REQUEST",
			Error:        errors.New("UUNEXPECTED-ERRORR"),
			Expect:       true,
		},
		{
			Name:         "mismatch",
			RetryTargets: "UNEXPECTED-ERROR",
			Error:        errors.New("ERROR"),
			Expect:       false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			config := Config{
				RetryTargets: tc.RetryTargets,
			}

			serviceHandler, err := getServiceHandler("aws", config, channelID, connectionID, sampleRate, channelCount, languageCode, onResultFunc)
			assert.NoError(t, err)

			assert.Equal(t, tc.Expect, serviceHandler.IsRetryTarget(tc.Error))
		})
	}
}
