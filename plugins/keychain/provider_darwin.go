//go:build darwin

package main

/*
#cgo LDFLAGS: -framework Security -framework CoreFoundation
#cgo CFLAGS: -Wno-deprecated-declarations
#include <Security/Security.h>
#include <CoreFoundation/CoreFoundation.h>
#include <stdlib.h>
#include <string.h>

typedef struct {
    char* data;
    int   length;
    int   error_code;
    char* error_msg;
} KeychainResult;

static KeychainResult keychainGetPassword(const char* service, const char* account) {
    KeychainResult result = {NULL, 0, 0, NULL};

    CFStringRef svcRef  = CFStringCreateWithCString(NULL, service,  kCFStringEncodingUTF8);
    CFStringRef accRef  = CFStringCreateWithCString(NULL, account,  kCFStringEncodingUTF8);
    CFStringRef prompt  = CFStringCreateWithCString(NULL,
        "Locksmith wants to access a secret", kCFStringEncodingUTF8);

    CFMutableDictionaryRef q = CFDictionaryCreateMutable(NULL, 0,
        &kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
    CFDictionarySetValue(q, kSecClass,              kSecClassGenericPassword);
    CFDictionarySetValue(q, kSecAttrService,        svcRef);
    CFDictionarySetValue(q, kSecAttrAccount,        accRef);
    CFDictionarySetValue(q, kSecReturnData,         kCFBooleanTrue);
    CFDictionarySetValue(q, kSecMatchLimit,         kSecMatchLimitOne);
    CFDictionarySetValue(q, kSecUseOperationPrompt, prompt);

    CFTypeRef dataRef = NULL;
    OSStatus status = SecItemCopyMatching(q, &dataRef);

    if (status == errSecSuccess && dataRef != NULL) {
        CFDataRef data    = (CFDataRef)dataRef;
        result.length     = (int)CFDataGetLength(data);
        result.data       = (char*)malloc(result.length);
        memcpy(result.data, CFDataGetBytePtr(data), result.length);
        CFRelease(dataRef);
    } else {
        result.error_code = (int)status;
        // Use SecCopyErrorMessageString for a human-readable description.
        CFStringRef cfMsg = SecCopyErrorMessageString(status, NULL);
        if (cfMsg != NULL) {
            CFIndex sz = CFStringGetMaximumSizeForEncoding(
                CFStringGetLength(cfMsg), kCFStringEncodingUTF8) + 1;
            result.error_msg = (char*)malloc(sz);
            CFStringGetCString(cfMsg, result.error_msg, sz, kCFStringEncodingUTF8);
            CFRelease(cfMsg);
        } else {
            result.error_msg = (char*)malloc(32);
            snprintf(result.error_msg, 32, "OSStatus %d", (int)status);
        }
    }

    CFRelease(q); CFRelease(svcRef); CFRelease(accRef); CFRelease(prompt);
    return result;
}
*/
import "C"

import (
	"context"
	"fmt"
	"strings"
	"unsafe"

	sdk "github.com/lorem-dev/locksmith/sdk"
	vaultv1 "github.com/lorem-dev/locksmith/gen/proto/vault/v1"
)

// KeychainProvider retrieves secrets from the macOS Keychain using the Security framework.
type KeychainProvider struct{}

// keychainGetPasswordFunc is injectable for tests.
var keychainGetPasswordFunc = func(service, account string) ([]byte, error) {
	cService := C.CString(service)
	defer C.free(unsafe.Pointer(cService))
	cAccount := C.CString(account)
	defer C.free(unsafe.Pointer(cAccount))

	result := C.keychainGetPassword(cService, cAccount)
	if result.error_code != 0 {
		code := int(result.error_code)
		msg := C.GoString(result.error_msg)
		C.free(unsafe.Pointer(result.error_msg))
		return nil, keychainError(code, msg)
	}

	secret := C.GoBytes(unsafe.Pointer(result.data), result.length)
	C.memset(unsafe.Pointer(result.data), 0, C.size_t(result.length))
	C.free(unsafe.Pointer(result.data))
	return secret, nil
}

// keychainError maps an OSStatus code to a typed sdk.VaultError.
func keychainError(code int, msg string) error {
	full := fmt.Sprintf("keychain: %s", msg)
	switch code {
	case -25300: // errSecItemNotFound
		return sdk.NotFoundError(full)
	case -25293, -25308: // errSecAuthFailed, errSecInteractionNotAllowed
		return sdk.PermissionDeniedError(full)
	default:
		return sdk.InternalError(full)
	}
}

// parseKeychainPath resolves the service and account from a path and vault-level service.
// Path "service/account" overrides vaultService. Plain "account" uses vaultService,
// falling back to "locksmith" for backward compatibility.
func parseKeychainPath(path, vaultService string) (service, account string) {
	if i := strings.Index(path, "/"); i >= 0 {
		return path[:i], path[i+1:]
	}
	if vaultService != "" {
		return vaultService, path
	}
	return "locksmith", path
}

// GetSecret retrieves a secret from the macOS Keychain.
func (p *KeychainProvider) GetSecret(_ context.Context, req *vaultv1.GetSecretRequest) (*vaultv1.GetSecretResponse, error) {
	vaultService := req.Opts["service"]
	service, account := parseKeychainPath(req.Path, vaultService)

	secret, err := keychainGetPasswordFunc(service, account)
	if err != nil {
		return nil, err
	}
	return &vaultv1.GetSecretResponse{Secret: secret, ContentType: "text/plain"}, nil
}

// HealthCheck confirms the macOS Keychain is accessible.
func (p *KeychainProvider) HealthCheck(_ context.Context, _ *vaultv1.HealthCheckRequest) (*vaultv1.HealthCheckResponse, error) {
	return &vaultv1.HealthCheckResponse{Available: true, Message: "macOS Keychain available"}, nil
}

// Info returns plugin metadata.
func (p *KeychainProvider) Info(_ context.Context, _ *vaultv1.InfoRequest) (*vaultv1.InfoResponse, error) {
	return &vaultv1.InfoResponse{Name: "keychain", Version: "0.1.0", Platforms: []string{"darwin"}}, nil
}
