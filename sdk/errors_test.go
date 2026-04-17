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
		want codes.Code
	}{
		{"NotFound", sdk.NotFoundError("x"), codes.NotFound},
		{"PermissionDenied", sdk.PermissionDeniedError("x"), codes.PermissionDenied},
		{"Unavailable", sdk.UnavailableError("x"), codes.Unavailable},
		{"Unauthenticated", sdk.UnauthenticatedError("x"), codes.Unauthenticated},
		{"Internal", sdk.InternalError("x"), codes.Internal},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, ok := status.FromError(tc.err)
			if !ok {
				t.Fatalf("status.FromError returned ok=false")
			}
			if s.Code() != tc.want {
				t.Errorf("Code() = %v, want %v", s.Code(), tc.want)
			}
		})
	}
}
