package control

import (
	"errors"
	"fmt"
	"os"

	"llamarig/config"
	"llamarig/core/configstore"
	"llamarig/core/modelpresets"
	"llamarig/core/runtime"
)

type ErrorKind string

const (
	ErrorInvalidInput ErrorKind = "invalid_input"
	ErrorNotFound     ErrorKind = "not_found"
	ErrorConflict     ErrorKind = "conflict"
	ErrorRuntime      ErrorKind = "runtime_error"
	ErrorTimeout      ErrorKind = "timeout"
	ErrorPermission   ErrorKind = "permission"
	ErrorInternal     ErrorKind = "internal"
)

type Error struct {
	Kind    ErrorKind `json:"kind"`
	Message string    `json:"message"`
	Err     error     `json:"-"`
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func CoreError(kind ErrorKind, message string, err error) *Error {
	return &Error{Kind: kind, Message: message, Err: err}
}

func Errorf(kind ErrorKind, format string, args ...any) *Error {
	return &Error{Kind: kind, Message: fmt.Sprintf(format, args...)}
}

func Kind(err error) ErrorKind {
	if err == nil {
		return ""
	}
	if coreErr, ok := errors.AsType[*Error](err); ok {
		return coreErr.Kind
	}
	return ErrorInternal
}

func Message(err error) string {
	if err == nil {
		return ""
	}
	if coreErr, ok := errors.AsType[*Error](err); ok {
		return coreErr.Message
	}
	return err.Error()
}

func mapRuntimeError(err error, fallback string) error {
	switch runtime.Kind(err) {
	case runtime.ErrorInvalidInput:
		return CoreError(ErrorInvalidInput, MessageOr(fallback, err), err)
	case runtime.ErrorTimeout:
		return CoreError(ErrorTimeout, MessageOr(fallback, err), err)
	case runtime.ErrorRuntime:
		return CoreError(ErrorRuntime, MessageOr(fallback, err), err)
	default:
		return err
	}
}

// SentinelKind pairs a sentinel error with the ErrorKind it maps to.
type SentinelKind struct {
	Target error
	Kind   ErrorKind
}

// MapSentinel returns a CoreError for the first table entry whose Target
// matches err (via errors.Is), preserving err.Error() as the message. If no
// entry matches, err is returned unchanged.
func MapSentinel(err error, table []SentinelKind) error {
	if err == nil {
		return nil
	}
	for _, e := range table {
		if errors.Is(err, e.Target) {
			return CoreError(e.Kind, err.Error(), err)
		}
	}
	return err
}

var serverConfigErrorTable = []SentinelKind{
	{modelpresets.ErrInvalid, ErrorInvalidInput},
	{modelpresets.ErrNotFound, ErrorNotFound},
	{modelpresets.ErrExists, ErrorConflict},
	{os.ErrNotExist, ErrorNotFound},
	{errors.ErrUnsupported, ErrorInvalidInput},
}

func mapServerConfigError(err error) error {
	return MapSentinel(err, serverConfigErrorTable)
}

var configStoreErrorTable = []SentinelKind{
	{config.ErrAutostartCapExceeded, ErrorConflict},
	{configstore.ErrEmpty, ErrorInvalidInput},
	{configstore.ErrTooLarge, ErrorInvalidInput},
	{configstore.ErrMalformed, ErrorInvalidInput},
	{errors.ErrUnsupported, ErrorInvalidInput},
}

func mapConfigStoreError(err error) error {
	if err == nil {
		return nil
	}
	// os.ErrNotExist uses a custom message, so handle it before the table.
	// It must be checked after the autostart/store sentinels to preserve
	// the original precedence.
	if errors.Is(err, config.ErrAutostartCapExceeded) {
		return CoreError(ErrorConflict, err.Error(), err)
	}
	if errors.Is(err, configstore.ErrEmpty) || errors.Is(err, configstore.ErrTooLarge) || errors.Is(err, configstore.ErrMalformed) {
		return CoreError(ErrorInvalidInput, err.Error(), err)
	}
	if errors.Is(err, os.ErrNotExist) {
		return CoreError(ErrorNotFound, "config.yaml entry not found", err)
	}
	return MapSentinel(err, configStoreErrorTable)
}

func MessageOr(fallback string, err error) string {
	if err == nil || err.Error() == "" {
		return fallback
	}
	return err.Error()
}
