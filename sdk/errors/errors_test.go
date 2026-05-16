package errors_test

import (
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	sdkerrors "github.com/lorem-dev/locksmith/sdk/errors"
)

func TestVaultError_Error(t *testing.T) {
	err := sdkerrors.NotFoundError("item not found")
	if err.Error() != "item not found" {
		t.Errorf("Error() = %q, want %q", err.Error(), "item not found")
	}
}

func TestVaultError_GRPCStatus(t *testing.T) {
	err := sdkerrors.NotFoundError("item not found")
	s, ok := status.FromError(err)
	if !ok {
		t.Fatal("status.FromError() returned ok=false for VaultError")
	}
	if s.Code() != codes.NotFound {
		t.Errorf("Code() = %v, want NotFound", s.Code())
	}
	if s.Message() != "item not found" {
		t.Errorf("Message() = %q, want %q", s.Message(), "item not found")
	}
}

func TestVaultError_Constructors(t *testing.T) {
	cases := []struct {
		name string
		err  error
		code codes.Code
		msg  string
	}{
		{"NotFound", sdkerrors.NotFoundError("item missing"), codes.NotFound, "item missing"},
		{"PermissionDenied", sdkerrors.PermissionDeniedError("access denied"), codes.PermissionDenied, "access denied"},
		{"Unavailable", sdkerrors.UnavailableError("plugin down"), codes.Unavailable, "plugin down"},
		{"Unauthenticated", sdkerrors.UnauthenticatedError("no passphrase"), codes.Unauthenticated, "no passphrase"},
		{"Internal", sdkerrors.InternalError("unexpected"), codes.Internal, "unexpected"},
		{"InvalidArgument", sdkerrors.InvalidArgumentError("bad input"), codes.InvalidArgument, "bad input"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, ok := status.FromError(tc.err)
			if !ok {
				t.Fatalf("status.FromError returned ok=false")
			}
			if s.Code() != tc.code {
				t.Errorf("Code() = %v, want %v", s.Code(), tc.code)
			}
			if s.Message() != tc.msg {
				t.Errorf("Message() = %q, want %q", s.Message(), tc.msg)
			}
		})
	}
}
