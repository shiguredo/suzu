package suzu

type SuzuError struct {
	Code    int
	Message string
	Retry   bool
}

func (e *SuzuError) Error() string {
	return e.Message
}

func (e *SuzuError) IsRetry() bool {
	return e.Retry
}

type SuzuConfError struct {
	Message string
}

func NewSuzuConfError(err error) *SuzuConfError {
	return &SuzuConfError{
		Message: err.Error(),
	}
}

func (e *SuzuConfError) Error() string {
	return e.Message
}
