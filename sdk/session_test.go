package sdk_test

import (
	"testing"

	sdk "github.com/lorem-dev/locksmith/sdk"
)

func TestHideSession_Short(t *testing.T) {
	result := sdk.HideSession("abc")
	want := "****"
	if result != want {
		t.Errorf("HideSession() = %q, want %q", result, want)
	}
}

func TestHideSession_Medium(t *testing.T) {
	result := sdk.HideSession("abcdefghijklmnop")
	want := "abcde****mnop"
	if result != want {
		t.Errorf("HideSession() = %q, want %q", result, want)
	}
}

func TestHideSession_Long(t *testing.T) {
	result := sdk.HideSession("abcdefghijklmnopqrstuvwxyz12345")
	want := "abcde****vwxyz12345"
	if result != want {
		t.Errorf("HideSession() = %q, want %q", result, want)
	}
}
