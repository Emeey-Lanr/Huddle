package utility




type AppError struct {
	Status int
	Message string
	Err error
}


func (err *AppError) Error ()AppError{
  return *err
}