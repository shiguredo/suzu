package suzu

import (
	"context"
	"errors"
	"io"
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

	t.Run("default", func(t *testing.T) {
		testCases := []struct {
			Name   string
			Config Config
			Input  Input
			Expect Expect
		}{
			{
				Name: "minimumTranscribedTime == 0 && minimumConfidenceScore == 0",
				Config: Config{
					MinimumTranscribedTime: 0,
					MinimumConfidenceScore: 0,
				},
				Input: Input{
					Alt: transcribestreamingservice.Alternative{
						Transcript: aws.String("filter is disabled"),
						Items: []*transcribestreamingservice.Item{
							{
								StartTime:  aws.Float64(0),
								EndTime:    aws.Float64(1),
								Confidence: aws.Float64(1),
								Content:    aws.String("test"),
								Type:       aws.String(transcribestreamingservice.ItemTypePronunciation),
							},
							{
								StartTime:  aws.Float64(0),
								EndTime:    aws.Float64(1),
								Confidence: aws.Float64(1),
								Content:    aws.String("data"),
								Type:       aws.String(transcribestreamingservice.ItemTypePronunciation),
							},
						},
					},
					IsPartial: false,
				},
				Expect: Expect{
					Message: "filter is disabled",
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
					MinimumConfidenceScore: 0.1,
				},
				Input: Input{
					Alt: transcribestreamingservice.Alternative{
						Items: []*transcribestreamingservice.Item{
							{
								StartTime:  aws.Float64(0),
								EndTime:    aws.Float64(0),
								Confidence: aws.Float64(1),
								Content:    aws.String("test"),
								Type:       aws.String(transcribestreamingservice.ItemTypePronunciation),
							},
							{
								StartTime:  aws.Float64(0),
								EndTime:    aws.Float64(0),
								Confidence: aws.Float64(1),
								Content:    aws.String("data"),
								Type:       aws.String(transcribestreamingservice.ItemTypePronunciation),
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
								Type:      aws.String(transcribestreamingservice.ItemTypePronunciation),
							},
							{
								StartTime: aws.Float64(1.03),
								EndTime:   aws.Float64(1.04),
								Content:   aws.String("data"),
								Type:      aws.String(transcribestreamingservice.ItemTypePronunciation),
							},
							{
								StartTime: aws.Float64(1.04),
								EndTime:   aws.Float64(1.06),
								Content:   aws.String("0"),
								Type:      aws.String(transcribestreamingservice.ItemTypePronunciation),
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
								Type:      aws.String(transcribestreamingservice.ItemTypePronunciation),
							},
							{
								StartTime: aws.Float64(1.01),
								EndTime:   aws.Float64(1.02),
								Content:   aws.String("data"),
								Type:      aws.String(transcribestreamingservice.ItemTypePronunciation),
							},
							{
								StartTime: aws.Float64(1.02),
								EndTime:   aws.Float64(1.03),
								Content:   aws.String("0"),
								Type:      aws.String(transcribestreamingservice.ItemTypePronunciation),
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
								Type:      aws.String(transcribestreamingservice.ItemTypePronunciation),
							},
							{
								StartTime: aws.Float64(1.019),
								EndTime:   aws.Float64(1.038),
								Content:   aws.String("data"),
								Type:      aws.String(transcribestreamingservice.ItemTypePronunciation),
							},
							{
								StartTime: aws.Float64(1.038),
								EndTime:   aws.Float64(1.057),
								Content:   aws.String("0"),
								Type:      aws.String(transcribestreamingservice.ItemTypePronunciation),
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
								Type:      aws.String(transcribestreamingservice.ItemTypePronunciation),
							},
							{
								StartTime: aws.Float64(1.02),
								EndTime:   aws.Float64(1.02),
								Content:   aws.String("、"),
								Type:      aws.String(transcribestreamingservice.ItemTypePunctuation),
							},
							{
								StartTime: aws.Float64(1.02),
								EndTime:   aws.Float64(1.04),
								Content:   aws.String("データ"),
								Type:      aws.String(transcribestreamingservice.ItemTypePronunciation),
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
		t.Run("IsPartial == false", func(t *testing.T) {
			testCases := []struct {
				Name   string
				Config Config
				Input  Input
				Expect Expect
			}{
				{
					Name: "minimumConfidenceScore is 0",
					Config: Config{
						MinimumTranscribedTime: 0.01,
						MinimumConfidenceScore: 0,
					},
					Input: Input{
						Alt: transcribestreamingservice.Alternative{
							Items: []*transcribestreamingservice.Item{
								{
									Confidence: aws.Float64(0),
									StartTime:  aws.Float64(0),
									EndTime:    aws.Float64(1),
									Content:    aws.String("test"),
									Type:       aws.String(transcribestreamingservice.ItemTypePronunciation),
								},
								{
									Confidence: aws.Float64(0),
									StartTime:  aws.Float64(0),
									EndTime:    aws.Float64(1),
									Content:    aws.String("data"),
									Type:       aws.String(transcribestreamingservice.ItemTypePronunciation),
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
									Type:       aws.String(transcribestreamingservice.ItemTypePronunciation),
								},
								{
									Confidence: aws.Float64(0),
									StartTime:  aws.Float64(0),
									EndTime:    aws.Float64(0),
									Content:    aws.String("data"),
									Type:       aws.String(transcribestreamingservice.ItemTypePronunciation),
								},
								{
									Confidence: aws.Float64(0.11),
									StartTime:  aws.Float64(0),
									EndTime:    aws.Float64(0),
									Content:    aws.String("1"),
									Type:       aws.String(transcribestreamingservice.ItemTypePronunciation),
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
									Type:       aws.String(transcribestreamingservice.ItemTypePronunciation),
								},
								{
									Confidence: aws.Float64(0.1),
									StartTime:  aws.Float64(0),
									EndTime:    aws.Float64(0),
									Content:    aws.String("data"),
									Type:       aws.String(transcribestreamingservice.ItemTypePronunciation),
								},
								{
									Confidence: aws.Float64(0.1),
									StartTime:  aws.Float64(0),
									EndTime:    aws.Float64(0),
									Content:    aws.String("1"),
									Type:       aws.String(transcribestreamingservice.ItemTypePronunciation),
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
									Type:       aws.String(transcribestreamingservice.ItemTypePronunciation),
								},
								{
									Confidence: aws.Float64(0.09),
									StartTime:  aws.Float64(0),
									EndTime:    aws.Float64(0),
									Content:    aws.String("data"),
									Type:       aws.String(transcribestreamingservice.ItemTypePronunciation),
								},
								{
									Confidence: aws.Float64(0.09),
									StartTime:  aws.Float64(0),
									EndTime:    aws.Float64(0),
									Content:    aws.String("1"),
									Type:       aws.String(transcribestreamingservice.ItemTypePronunciation),
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
									Type:       aws.String(transcribestreamingservice.ItemTypePronunciation),
								},
								{
									// 句読点は Confidence は nil
									Confidence: nil,
									StartTime:  aws.Float64(1.02),
									EndTime:    aws.Float64(1.02),
									Content:    aws.String("、"),
									Type:       aws.String(transcribestreamingservice.ItemTypePunctuation),
								},
								{
									Confidence: aws.Float64(0.2),
									StartTime:  aws.Float64(1.02),
									EndTime:    aws.Float64(1.04),
									Content:    aws.String("データ"),
									Type:       aws.String(transcribestreamingservice.ItemTypePronunciation),
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

		t.Run("IsPartial == true", func(t *testing.T) {
			testCases := []struct {
				Name   string
				Config Config
				Input  Input
				Expect Expect
			}{
				{
					Name: "minimumConfidenceScore is 0",
					Config: Config{
						MinimumTranscribedTime: 0.01,
						MinimumConfidenceScore: 0,
					},
					Input: Input{
						Alt: transcribestreamingservice.Alternative{
							Items: []*transcribestreamingservice.Item{
								{
									Confidence: aws.Float64(0),
									StartTime:  aws.Float64(0),
									EndTime:    aws.Float64(1),
									Content:    aws.String("test"),
									Type:       aws.String(transcribestreamingservice.ItemTypePronunciation),
								},
								{
									Confidence: aws.Float64(0),
									StartTime:  aws.Float64(0),
									EndTime:    aws.Float64(1),
									Content:    aws.String("data"),
									Type:       aws.String(transcribestreamingservice.ItemTypePronunciation),
								},
							},
						},
						IsPartial: true,
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
									Type:       aws.String(transcribestreamingservice.ItemTypePronunciation),
								},
								{
									Confidence: aws.Float64(0),
									StartTime:  aws.Float64(0),
									EndTime:    aws.Float64(0),
									Content:    aws.String("data"),
									Type:       aws.String(transcribestreamingservice.ItemTypePronunciation),
								},
								{
									Confidence: aws.Float64(0.11),
									StartTime:  aws.Float64(0),
									EndTime:    aws.Float64(0),
									Content:    aws.String("1"),
									Type:       aws.String(transcribestreamingservice.ItemTypePronunciation),
								},
							},
						},
						IsPartial: true,
					},
					Expect: Expect{
						Message: "testdata1",
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
									Type:       aws.String(transcribestreamingservice.ItemTypePronunciation),
								},
								{
									Confidence: aws.Float64(0.1),
									StartTime:  aws.Float64(0),
									EndTime:    aws.Float64(0),
									Content:    aws.String("data"),
									Type:       aws.String(transcribestreamingservice.ItemTypePronunciation),
								},
								{
									Confidence: aws.Float64(0.1),
									StartTime:  aws.Float64(0),
									EndTime:    aws.Float64(0),
									Content:    aws.String("1"),
									Type:       aws.String(transcribestreamingservice.ItemTypePronunciation),
								},
							},
						},
						IsPartial: true,
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
									Type:       aws.String(transcribestreamingservice.ItemTypePronunciation),
								},
								{
									Confidence: aws.Float64(0.09),
									StartTime:  aws.Float64(0),
									EndTime:    aws.Float64(0),
									Content:    aws.String("data"),
									Type:       aws.String(transcribestreamingservice.ItemTypePronunciation),
								},
								{
									Confidence: aws.Float64(0.09),
									StartTime:  aws.Float64(0),
									EndTime:    aws.Float64(0),
									Content:    aws.String("1"),
									Type:       aws.String(transcribestreamingservice.ItemTypePronunciation),
								},
							},
						},
						IsPartial: true,
					},
					Expect: Expect{
						Message: "testdata1",
						Ok:      true,
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
									Type:       aws.String(transcribestreamingservice.ItemTypePronunciation),
								},
								{
									// 句読点は Confidence は nil
									Confidence: nil,
									StartTime:  aws.Float64(1.02),
									EndTime:    aws.Float64(1.02),
									Content:    aws.String("、"),
									Type:       aws.String(transcribestreamingservice.ItemTypePunctuation),
								},
								{
									Confidence: aws.Float64(0.2),
									StartTime:  aws.Float64(1.02),
									EndTime:    aws.Float64(1.04),
									Content:    aws.String("データ"),
									Type:       aws.String(transcribestreamingservice.ItemTypePronunciation),
								},
							},
						},
						IsPartial: true,
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
	})

	t.Run("punctuation", func(t *testing.T) {
		testCases := []struct {
			Name   string
			Config Config
			Input  Input
			Expect Expect
		}{
			{
				Name: "pronunciation with punctuation",
				Config: Config{
					MinimumConfidenceScore: 0.1,
					MinimumTranscribedTime: 0.1,
				},
				Input: Input{
					Alt: transcribestreamingservice.Alternative{
						Items: []*transcribestreamingservice.Item{
							{
								Confidence: nil,
								StartTime:  aws.Float64(0),
								EndTime:    aws.Float64(0.1),
								Content:    aws.String("テスト"),
								Type:       aws.String(transcribestreamingservice.ItemTypePronunciation),
							},
							{
								Confidence: nil,
								StartTime:  aws.Float64(0.1),
								EndTime:    aws.Float64(0.1),
								Content:    aws.String("、"),
								Type:       aws.String(transcribestreamingservice.ItemTypePunctuation),
							},
							{
								Confidence: nil,
								StartTime:  aws.Float64(0.1),
								EndTime:    aws.Float64(0.2),
								Content:    aws.String("データ"),
								Type:       aws.String(transcribestreamingservice.ItemTypePronunciation),
							},
							{
								Confidence: nil,
								StartTime:  aws.Float64(0.2),
								EndTime:    aws.Float64(0.2),
								Content:    aws.String("。"),
								Type:       aws.String(transcribestreamingservice.ItemTypePunctuation),
							},
						},
					},
					IsPartial: false,
				},
				Expect: Expect{
					Message: "テスト、データ。",
					Ok:      true,
				},
			},
			{
				Name: "no pronunciation",
				Config: Config{
					MinimumConfidenceScore: 0.1,
					MinimumTranscribedTime: 0,
				},
				Input: Input{
					Alt: transcribestreamingservice.Alternative{
						Items: []*transcribestreamingservice.Item{
							{
								Confidence: nil,
								StartTime:  aws.Float64(0),
								EndTime:    aws.Float64(0),
								Content:    aws.String("。"),
								Type:       aws.String(transcribestreamingservice.ItemTypePunctuation),
							},
							{
								Confidence: nil,
								StartTime:  aws.Float64(0),
								EndTime:    aws.Float64(0),
								Content:    aws.String("、"),
								Type:       aws.String(transcribestreamingservice.ItemTypePunctuation),
							},
							{
								Confidence: nil,
								StartTime:  aws.Float64(0),
								EndTime:    aws.Float64(0),
								Content:    aws.String("。"),
								Type:       aws.String(transcribestreamingservice.ItemTypePunctuation),
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

func TestIsRetryTargetForAmazonTranscribe(t *testing.T) {
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
		{
			Name:         "LimitExceededException",
			RetryTargets: "UNEXPECTED-ERROR",
			Error:        &transcribestreamingservice.LimitExceededException{},
			Expect:       true,
		},
		{
			Name:         "InternalFailureException",
			RetryTargets: "UNEXPECTED-ERROR",
			Error:        &transcribestreamingservice.InternalFailureException{},
			Expect:       true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			config := Config{
				RetryTargets: tc.RetryTargets,
			}

			serviceHandler, err := getServiceHandler("awsv1", config, channelID, connectionID, sampleRate, channelCount, languageCode, onResultFunc)
			assert.NoError(t, err)

			assert.Equal(t, tc.Expect, serviceHandler.IsRetryTarget(tc.Error))
		})
	}
}
