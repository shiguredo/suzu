package suzu

import "github.com/aws/aws-sdk-go/service/transcribestreamingservice"

type SuzuError struct {
	Code    int
	Message string
}

func (e *SuzuError) Error() string {
	return e.Message
}

var awsTranscribeErrors = map[string]int{
	transcribestreamingservice.ErrCodeLimitExceededException:      429,
	transcribestreamingservice.ErrCodeConflictException:           409,
	transcribestreamingservice.ErrCodeBadRequestException:         400,
	transcribestreamingservice.ErrCodeInternalFailureException:    500,
	transcribestreamingservice.ErrCodeServiceUnavailableException: 503,
}
