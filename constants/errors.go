package constants

type PublicError struct {
	Msg    string `json:"message"`
	Status int    `json:"status"`
}

func (e PublicError) Error() string {
	return e.Msg
}

type DatabaseError struct {
	Err error
}

func (e DatabaseError) Error() string {
	return e.Err.Error()
}

func (e DatabaseError) Unwrap() error {
	return e.Err
}
