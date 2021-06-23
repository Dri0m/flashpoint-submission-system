package service

import "github.com/Dri0m/flashpoint-submission-system/constants"

func dberr(err error) error {
	return constants.DatabaseError{Err: err}
}

func perr(msg string, status int) error {
	return constants.PublicError{Msg: msg, Status: status}
}
