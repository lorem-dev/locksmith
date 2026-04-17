package sdk_test

import (
	"testing"

	sdk "github.com/lorem-dev/locksmith/sdk"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestVaultError_Error(t *testing.T) {
	err := sdk.NotFoundError("item not found")
	if err.Error() != "item not found" {
		t.Errorf("Error() = %q, want %q", err.Error(), "item not found")
	}
}

func TestVaultError_GRPCStatus(t *testing.T) {
	err := sdk.NotFoundError("item not found")
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
		{"NotFound", sdk.NotFoundError("item missing"), codes.NotFound, "item missing"},
		{"PermissionDenied", sdk.PermissionDeniedError("access denied"), codes.PermissionDenied, "access denied"},
		{"Unavailable", sdk.UnavailableError("plugin down"), codes.Unavailable, "plugin down"},
		{"Unauthenticated", sdk.UnauthenticatedError("no passphrase"), codes.Unauthenticated, "no passphrase"},
		{"Internal", sdk.InternalError("unexpected"), codes.Internal, "unexpected"},
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
