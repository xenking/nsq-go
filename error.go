package nsq

import (
	"bufio"
	"io"

	"github.com/pkg/errors"
)

type Error string

const (
	ErrInvalid      Error = "E_INVALID"
	ErrBadBody      Error = "E_BAD_BODY"
	ErrBadTopic     Error = "E_BAD_TOPIC"
	ErrBadChannel   Error = "E_BAD_CHANNEL"
	ErrBadMessage   Error = "E_BAD_MESSAGE"
	ErrPubFailed    Error = "E_PUB_FAILED"
	ErrMPubFailed   Error = "E_MPUB_FAILED"
	ErrFinFailed    Error = "E_FIN_FAILED"
	ErrReqFailed    Error = "E_REQ_FAILED"
	ErrTouchFailed  Error = "E_TOUCH_FAILED"
	ErrAuthFailed   Error = "E_AUTH_FAILED"
	ErrUnauthorized Error = "E_UNAUTHORIZED"
)

func (e Error) FrameType() FrameType {
	return FrameTypeError
}

func (e Error) Error() string {
	return string(e)
}

func (e Error) String() string {
	return string(e)
}

func (e Error) Write(w *bufio.Writer) (err error) {
	if err = writeFrameHeader(w, FrameTypeError, len(e)); err != nil {
		err = errors.WithMessage(err, "writing error message")
		return
	}

	if _, err = w.WriteString(string(e)); err != nil {
		err = errors.Wrap(err, "writing error message")
		return
	}

	return
}

func readError(n int, r *bufio.Reader) (e Error, err error) {
	data := make([]byte, n)

	if _, err = io.ReadFull(r, data); err != nil {
		err = errors.Wrap(err, "reading error message")
		return
	}

	e = Error(data)
	return
}
