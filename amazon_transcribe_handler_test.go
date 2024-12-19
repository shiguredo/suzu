package suzu

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/transcribestreamingservice"
	"github.com/stretchr/testify/assert"
)

func TestBuildMessage(t *testing.T) {

	type Input struct {
		Alt       transcribestreamingservice.Alternative
		IsPartial bool
	}

	type Expect struct {
		Message string
		Ok      bool
	}

	testCases := []struct {
		Name   string
		Config Config
		Input  Input
		Expect Expect
	}{
		{
			Name: "minimumTranscribedTime is 0",
			Config: Config{
				MinimumTranscribedTime: 0,
			},
			Input: Input{
				Alt: transcribestreamingservice.Alternative{
					Items: []*transcribestreamingservice.Item{
						{
							StartTime: aws.Float64(0),
							EndTime:   aws.Float64(1),
							Content:   aws.String("test"),
						},
						{
							StartTime: aws.Float64(0),
							EndTime:   aws.Float64(1),
							Content:   aws.String("data"),
						},
					},
				},
				IsPartial: false,
			},
			Expect: Expect{
				Message: "testdata",
				Ok:      true,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			actual, ok := buildMessage(tc.Config, tc.Input.Alt, tc.Input.IsPartial)
			assert.Equal(t, tc.Expect.Ok, ok)
			assert.Equal(t, tc.Expect.Message, actual)
		})
	}

}
