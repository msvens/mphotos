package server

import (
	"database/sql"
	"fmt"
	"google.golang.org/api/googleapi"
	"net/http"
)

type ApiError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *ApiError) Error() string {
	return fmt.Sprintf("code: %d message: %s", e.Code, e.Message)
}

func newError(code int, message string) *ApiError {
	return &ApiError{Code: code, Message: message}
}

func UnauthorizedError(message string) *ApiError {
	return newError(http.StatusUnauthorized, message)
}

func NotFoundError(message string) *ApiError {
	return newError(http.StatusNotFound, message)
}

func BadRequestError(message string) *ApiError {
	return newError(http.StatusBadRequest, message)
}

func InternalError(message string) *ApiError {
	return newError(http.StatusInternalServerError, message)
}

func ResolveError(err error) *ApiError {
	//check for api error
	e, ok := err.(*ApiError)
	if ok {
		return e
	}
	//check for google error
	e1, ok := err.(*googleapi.Error)
	if ok {
		return &ApiError{e1.Code, e1.Message}
	}
	//check for pg errors
	/*e2, ok := err.(*pq.Error)
	if ok {
		if e2.Code == ""
		return &ApiError{http.StatusInternalServerError, string(e2.Code)}
	}*/
	if err == sql.ErrNoRows {
		return NotFoundError("No such data")
	}
	//check for db error
	return InternalError(err.Error())
}
