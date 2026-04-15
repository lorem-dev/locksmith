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

// keychainGetPassword retrieves a generic password from the macOS Keychain.
// Passing kSecUseOperationPrompt causes the OS to show a Touch ID / password dialog.
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
		result.error_msg  = (char*)malloc(64);
		snprintf(result.error_msg, 64, "SecItemCopyMatching failed: %d", (int)status);
	}

	CFRelease(q); CFRelease(svcRef); CFRelease(accRef); CFRelease(prompt);
	return result;
}
*/
import "C"

import (
	"context"
	"fmt"
	"unsafe"

	vaultv1 "github.com/lorem-dev/locksmith/gen/proto/vault/v1"
)

// KeychainProvider retrieves secrets from the macOS Keychain using the Security framework.
// Authorization (Touch ID or password) is triggered by the OS when SecItemCopyMatching is called.
type KeychainProvider struct{}

// GetSecret retrieves a secret from the macOS Keychain. The service defaults to "locksmith"
// and can be overridden via opts["service"]. The path is used as the account name.
//
// Note: GetSecret is not covered by unit tests because the CGo Security framework call
// requires an actual Keychain item and may prompt for Touch ID / password. It is exercised
// by integration tests (Task 15).
func (p *KeychainProvider) GetSecret(_ context.Context, req *vaultv1.GetSecretRequest) (*vaultv1.GetSecretResponse, error) {
	service := "locksmith"
	if svc, ok := req.Opts["service"]; ok && svc != "" {
		service = svc
	}

	cService := C.CString(service)
	defer C.free(unsafe.Pointer(cService))
	cAccount := C.CString(req.Path)
	defer C.free(unsafe.Pointer(cAccount))

	result := C.keychainGetPassword(cService, cAccount)
	if result.error_code != 0 {
		msg := C.GoString(result.error_msg)
		C.free(unsafe.Pointer(result.error_msg))
		return nil, fmt.Errorf("keychain: %s", msg)
	}

	secret := C.GoBytes(unsafe.Pointer(result.data), result.length)
	C.memset(unsafe.Pointer(result.data), 0, C.size_t(result.length))
	C.free(unsafe.Pointer(result.data))

	return &vaultv1.GetSecretResponse{Secret: secret, ContentType: "text/plain"}, nil
}

// HealthCheck confirms the macOS Keychain is accessible.
func (p *KeychainProvider) HealthCheck(_ context.Context, _ *vaultv1.HealthCheckRequest) (*vaultv1.HealthCheckResponse, error) {
	return &vaultv1.HealthCheckResponse{Available: true, Message: "macOS Keychain available"}, nil
}

// Info returns plugin metadata.
func (p *KeychainProvider) Info(_ context.Context, _ *vaultv1.InfoRequest) (*vaultv1.PluginInfo, error) {
	return &vaultv1.PluginInfo{Name: "keychain", Version: "0.1.0", Platforms: []string{"darwin"}}, nil
}
