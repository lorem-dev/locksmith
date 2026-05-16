// Package errors provides vault error types for locksmith SDK consumers.
package errors

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// VaultError is a vault error that carries a gRPC status code.
// Implementing GRPCStatus() causes the go-plugin gRPC server to use this code
// when serialising the error, preventing double-wrapping.
type VaultError struct {
	Code    codes.Code
	Message string
}

func (e *VaultError) Error() string              { return e.Message }
func (e *VaultError) GRPCStatus() *status.Status { return status.New(e.Code, e.Message) }

// NotFoundError returns a VaultError with code NotFound.
func NotFoundError(msg string) error { return &VaultError{Code: codes.NotFound, Message: msg} }

// PermissionDeniedError returns a VaultError with code PermissionDenied.
func PermissionDeniedError(msg string) error {
	return &VaultError{Code: codes.PermissionDenied, Message: msg}
}

// UnavailableError returns a VaultError with code Unavailable.
func UnavailableError(msg string) error { return &VaultError{Code: codes.Unavailable, Message: msg} }

// UnauthenticatedError returns a VaultError with code Unauthenticated.
func UnauthenticatedError(msg string) error {
	return &VaultError{Code: codes.Unauthenticated, Message: msg}
}

// InternalError returns a VaultError with code Internal.
func InternalError(msg string) error { return &VaultError{Code: codes.Internal, Message: msg} }

// InvalidArgumentError returns a VaultError with code InvalidArgument.
func InvalidArgumentError(msg string) error {
	return &VaultError{Code: codes.InvalidArgument, Message: msg}
}
