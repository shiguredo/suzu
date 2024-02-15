package suzu

type SuzuError struct {
	Code    int
	Message string
}

func (e *SuzuError) Error() string {
	return e.Message
}
