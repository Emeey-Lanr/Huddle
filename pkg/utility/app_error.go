package utility

type AppError struct {
	StatusCode int
	Message    string
	Err        error
}

// Error lets an AppError travel through code that returns plain errors, such
// as a gorm transaction, and be recovered with errors.As.
func (e *AppError) Error() string {
	if e.Err != nil {
		return e.Message + ": " + e.Err.Error()
	}
	return e.Message
}

func (e *AppError) Unwrap() error { return e.Err }
