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

	t.Run("minimumTranscribedTime", func(t *testing.T) {
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
								EndTime:   aws.Float64(0),
								Content:   aws.String("test"),
							},
							{
								StartTime: aws.Float64(0),
								EndTime:   aws.Float64(0),
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
			{
				Name: "transcribedTime > MinimumTranscribedTime",
				Config: Config{
					MinimumTranscribedTime: 0.02,
					MinimumConfidenceScore: 0,
				},
				Input: Input{
					Alt: transcribestreamingservice.Alternative{
						Items: []*transcribestreamingservice.Item{
							{
								StartTime: aws.Float64(1.00),
								EndTime:   aws.Float64(1.03),
								Content:   aws.String("test"),
							},
							{
								StartTime: aws.Float64(1.03),
								EndTime:   aws.Float64(1.04),
								Content:   aws.String("data"),
							},
							{
								StartTime: aws.Float64(1.04),
								EndTime:   aws.Float64(1.06),
								Content:   aws.String("0"),
							},
						},
					},
					IsPartial: false,
				},
				Expect: Expect{
					Message: "test0",
					Ok:      true,
				},
			},
			{
				Name: "transcribedTime == MinimumTranscribedTime",
				Config: Config{
					MinimumTranscribedTime: 0.01,
					MinimumConfidenceScore: 0,
				},
				Input: Input{
					Alt: transcribestreamingservice.Alternative{
						Items: []*transcribestreamingservice.Item{
							{
								StartTime: aws.Float64(1.00),
								EndTime:   aws.Float64(1.01),
								Content:   aws.String("test"),
							},
							{
								StartTime: aws.Float64(1.01),
								EndTime:   aws.Float64(1.02),
								Content:   aws.String("data"),
							},
							{
								StartTime: aws.Float64(1.02),
								EndTime:   aws.Float64(1.03),
								Content:   aws.String("0"),
							},
						},
					},
					IsPartial: false,
				},
				Expect: Expect{
					Message: "testdata0",
					Ok:      true,
				},
			},
			{
				Name: "transcribedTime < MinimumTranscribedTime",
				Config: Config{
					MinimumTranscribedTime: 0.02,
					MinimumConfidenceScore: 0,
				},
				Input: Input{
					Alt: transcribestreamingservice.Alternative{
						Items: []*transcribestreamingservice.Item{
							{
								StartTime: aws.Float64(1.00),
								EndTime:   aws.Float64(1.019),
								Content:   aws.String("test"),
							},
							{
								StartTime: aws.Float64(1.019),
								EndTime:   aws.Float64(1.038),
								Content:   aws.String("data"),
							},
							{
								StartTime: aws.Float64(1.038),
								EndTime:   aws.Float64(1.057),
								Content:   aws.String("0"),
							},
						},
					},
					IsPartial: false,
				},
				Expect: Expect{
					Message: "",
					Ok:      false,
				},
			},
			{
				Name: "punctuation",
				Config: Config{
					MinimumTranscribedTime: 0.01,
					MinimumConfidenceScore: 0,
				},
				Input: Input{
					Alt: transcribestreamingservice.Alternative{
						Items: []*transcribestreamingservice.Item{
							{
								StartTime: aws.Float64(1.00),
								EndTime:   aws.Float64(1.02),
								Content:   aws.String("テスト"),
							},
							{
								// 句読点は StartTime と EndTime が同じ
								StartTime: aws.Float64(1.02),
								EndTime:   aws.Float64(1.02),
								Content:   aws.String("、"),
							},
							{
								StartTime: aws.Float64(1.02),
								EndTime:   aws.Float64(1.04),
								Content:   aws.String("データ"),
							},
						},
					},
					IsPartial: false,
				},
				Expect: Expect{
					Message: "テスト、データ",
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

	})

	t.Run("minimumConfidence", func(t *testing.T) {
		testCases := []struct {
			Name   string
			Config Config
			Input  Input
			Expect Expect
		}{
			{
				Name: "minimumConfidenceScore is 0",
				Config: Config{
					MinimumConfidenceScore: 0,
				},
				Input: Input{
					Alt: transcribestreamingservice.Alternative{
						Items: []*transcribestreamingservice.Item{
							{
								Confidence: aws.Float64(0),
								StartTime:  aws.Float64(0),
								EndTime:    aws.Float64(0),
								Content:    aws.String("test"),
							},
							{
								Confidence: aws.Float64(0),
								StartTime:  aws.Float64(0),
								EndTime:    aws.Float64(0),
								Content:    aws.String("data"),
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
			{
				Name: "confidence > minimumConfidenceScore ",
				Config: Config{
					MinimumConfidenceScore: 0.1,
					MinimumTranscribedTime: 0,
				},
				Input: Input{
					Alt: transcribestreamingservice.Alternative{
						Items: []*transcribestreamingservice.Item{
							{
								Confidence: aws.Float64(0.11),
								StartTime:  aws.Float64(0),
								EndTime:    aws.Float64(0),
								Content:    aws.String("test"),
							},
							{
								Confidence: aws.Float64(0),
								StartTime:  aws.Float64(0),
								EndTime:    aws.Float64(0),
								Content:    aws.String("data"),
							},
							{
								Confidence: aws.Float64(0.11),
								StartTime:  aws.Float64(0),
								EndTime:    aws.Float64(0),
								Content:    aws.String("1"),
							},
						},
					},
					IsPartial: false,
				},
				Expect: Expect{
					Message: "test1",
					Ok:      true,
				},
			},
			{
				Name: "confidence == minimumConfidenceScore ",
				Config: Config{
					MinimumConfidenceScore: 0.1,
					MinimumTranscribedTime: 0,
				},
				Input: Input{
					Alt: transcribestreamingservice.Alternative{
						Items: []*transcribestreamingservice.Item{
							{
								Confidence: aws.Float64(0.1),
								StartTime:  aws.Float64(0),
								EndTime:    aws.Float64(0),
								Content:    aws.String("test"),
							},
							{
								Confidence: aws.Float64(0.1),
								StartTime:  aws.Float64(0),
								EndTime:    aws.Float64(0),
								Content:    aws.String("data"),
							},
							{
								Confidence: aws.Float64(0.1),
								StartTime:  aws.Float64(0),
								EndTime:    aws.Float64(0),
								Content:    aws.String("1"),
							},
						},
					},
					IsPartial: false,
				},
				Expect: Expect{
					Message: "testdata1",
					Ok:      true,
				},
			},
			{
				Name: "confidence < minimumConfidenceScore ",
				Config: Config{
					MinimumConfidenceScore: 0.1,
					MinimumTranscribedTime: 0,
				},
				Input: Input{
					Alt: transcribestreamingservice.Alternative{
						Items: []*transcribestreamingservice.Item{
							{
								Confidence: aws.Float64(0),
								StartTime:  aws.Float64(0),
								EndTime:    aws.Float64(0),
								Content:    aws.String("test"),
							},
							{
								Confidence: aws.Float64(0.09),
								StartTime:  aws.Float64(0),
								EndTime:    aws.Float64(0),
								Content:    aws.String("data"),
							},
							{
								Confidence: aws.Float64(0.09),
								StartTime:  aws.Float64(0),
								EndTime:    aws.Float64(0),
								Content:    aws.String("1"),
							},
						},
					},
					IsPartial: false,
				},
				Expect: Expect{
					Message: "",
					Ok:      false,
				},
			},
			{
				Name: "punctuation",
				Config: Config{
					MinimumConfidenceScore: 0.1,
					MinimumTranscribedTime: 0,
				},
				Input: Input{
					Alt: transcribestreamingservice.Alternative{
						Items: []*transcribestreamingservice.Item{
							{
								Confidence: aws.Float64(0.2),
								StartTime:  aws.Float64(1.0),
								EndTime:    aws.Float64(1.02),
								Content:    aws.String("テスト"),
							},
							{
								// 句読点は Confidence は nil
								Confidence: nil,
								StartTime:  aws.Float64(1.02),
								EndTime:    aws.Float64(1.02),
								Content:    aws.String("、"),
							},
							{
								Confidence: aws.Float64(0.2),
								StartTime:  aws.Float64(1.02),
								EndTime:    aws.Float64(1.04),
								Content:    aws.String("データ"),
							},
						},
					},
					IsPartial: false,
				},
				Expect: Expect{
					Message: "テスト、データ",
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
	})
}
