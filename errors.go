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
