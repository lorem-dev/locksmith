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

static KeychainResult keychainSetPassword(const char* service, const char* account,
                                          const void* secret, int secret_len) {
    KeychainResult result = {NULL, 0, 0, NULL};
    CFErrorRef cferr = NULL;

    SecAccessControlRef ac = SecAccessControlCreateWithFlags(
        NULL,
        kSecAttrAccessibleWhenUnlockedThisDeviceOnly,
        kSecAccessControlUserPresence,
        &cferr);
    if (ac == NULL) {
        result.error_code = -1;
        result.error_msg = strdup("SecAccessControlCreateWithFlags failed");
        if (cferr) CFRelease(cferr);
        return result;
    }

    CFStringRef svcRef = CFStringCreateWithCString(NULL, service, kCFStringEncodingUTF8);
    CFStringRef accRef = CFStringCreateWithCString(NULL, account, kCFStringEncodingUTF8);
    CFDataRef   secRef = CFDataCreate(NULL, (const UInt8*)secret, secret_len);

    // Delete any existing item; ignore errSecItemNotFound. SecItemUpdate
    // cannot change access control, so delete-then-add is the only path.
    CFMutableDictionaryRef del = CFDictionaryCreateMutable(NULL, 0,
        &kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
    CFDictionarySetValue(del, kSecClass,       kSecClassGenericPassword);
    CFDictionarySetValue(del, kSecAttrService, svcRef);
    CFDictionarySetValue(del, kSecAttrAccount, accRef);
    OSStatus delStatus = SecItemDelete(del);
    CFRelease(del);
    if (delStatus != errSecSuccess && delStatus != errSecItemNotFound) {
        // Delete failed for a real reason (auth, missing entitlements, ...);
        // surface it rather than silently triggering an errSecDuplicateItem
        // on the subsequent SecItemAdd.
        result.error_code = (int)delStatus;
        CFStringRef cfMsg = SecCopyErrorMessageString(delStatus, NULL);
        if (cfMsg != NULL) {
            CFIndex sz = CFStringGetMaximumSizeForEncoding(
                CFStringGetLength(cfMsg), kCFStringEncodingUTF8) + 1;
            result.error_msg = (char*)malloc(sz);
            CFStringGetCString(cfMsg, result.error_msg, sz, kCFStringEncodingUTF8);
            CFRelease(cfMsg);
        } else {
            result.error_msg = (char*)malloc(32);
            snprintf(result.error_msg, 32, "OSStatus %d", (int)delStatus);
        }
        CFRelease(svcRef);
        CFRelease(accRef);
        CFRelease(secRef);
        CFRelease(ac);
        return result;
    }

    CFMutableDictionaryRef add = CFDictionaryCreateMutable(NULL, 0,
        &kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
    CFDictionarySetValue(add, kSecClass,             kSecClassGenericPassword);
    CFDictionarySetValue(add, kSecAttrService,       svcRef);
    CFDictionarySetValue(add, kSecAttrAccount,       accRef);
    CFDictionarySetValue(add, kSecValueData,         secRef);
    CFDictionarySetValue(add, kSecAttrAccessControl, ac);

    OSStatus status = SecItemAdd(add, NULL);
    if (status != errSecSuccess) {
        result.error_code = (int)status;
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

    CFRelease(add);
    CFRelease(svcRef);
    CFRelease(accRef);
    CFRelease(secRef);
    CFRelease(ac);
    return result;
}

// keychainExists fills result.length with 1 if a generic-password item
// with the given service+account is present, 0 if not. On error
// (anything other than errSecSuccess / errSecItemNotFound), error_code
// is set and error_msg is allocated.
static KeychainResult keychainExists(const char* service, const char* account) {
    KeychainResult result = {NULL, 0, 0, NULL};

    CFStringRef svcRef = CFStringCreateWithCString(NULL, service, kCFStringEncodingUTF8);
    CFStringRef accRef = CFStringCreateWithCString(NULL, account, kCFStringEncodingUTF8);

    CFMutableDictionaryRef q = CFDictionaryCreateMutable(NULL, 0,
        &kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
    CFDictionarySetValue(q, kSecClass,        kSecClassGenericPassword);
    CFDictionarySetValue(q, kSecAttrService,  svcRef);
    CFDictionarySetValue(q, kSecAttrAccount,  accRef);
    CFDictionarySetValue(q, kSecReturnData,   kCFBooleanFalse);
    CFDictionarySetValue(q, kSecMatchLimit,   kSecMatchLimitOne);

    OSStatus status = SecItemCopyMatching(q, NULL);
    CFRelease(q); CFRelease(svcRef); CFRelease(accRef);

    if (status == errSecSuccess) {
        result.length = 1;
        return result;
    }
    if (status == errSecItemNotFound) {
        result.length = 0;
        return result;
    }
    result.error_code = (int)status;
    result.error_msg = (char*)malloc(32);
    snprintf(result.error_msg, 32, "OSStatus %d", (int)status);
    return result;
}
*/
import "C"

import (
	"context"
	"fmt"
	"strings"
	"unsafe"

	vaultv1 "github.com/lorem-dev/locksmith/gen/proto/vault/v1"
	sdkerrors "github.com/lorem-dev/locksmith/sdk/errors"
	"github.com/lorem-dev/locksmith/sdk/platform"
	sdkversion "github.com/lorem-dev/locksmith/sdk/version"
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

// keychainSetPasswordFunc is injectable for tests.
var keychainSetPasswordFunc = func(service, account string, secret []byte) error {
	cService := C.CString(service)
	defer C.free(unsafe.Pointer(cService))
	cAccount := C.CString(account)
	defer C.free(unsafe.Pointer(cAccount))
	cSecret := unsafe.Pointer(&secret[0])

	result := C.keychainSetPassword(cService, cAccount, cSecret, C.int(len(secret)))
	if result.error_code != 0 {
		code := int(result.error_code)
		msg := C.GoString(result.error_msg)
		C.free(unsafe.Pointer(result.error_msg))
		return keychainError(code, msg)
	}
	return nil
}

// keychainExistsFunc is injectable for tests.
var keychainExistsFunc = func(service, account string) (bool, error) {
	cService := C.CString(service)
	defer C.free(unsafe.Pointer(cService))
	cAccount := C.CString(account)
	defer C.free(unsafe.Pointer(cAccount))

	result := C.keychainExists(cService, cAccount)
	if result.error_code != 0 {
		msg := C.GoString(result.error_msg)
		C.free(unsafe.Pointer(result.error_msg))
		return false, keychainError(int(result.error_code), msg)
	}
	return result.length == 1, nil
}

// keychainError maps an OSStatus code to a typed sdkerrors.VaultError.
func keychainError(code int, msg string) error {
	full := fmt.Sprintf("keychain: %s", msg)
	switch code {
	case -25300: // errSecItemNotFound
		return sdkerrors.NotFoundError(full)
	case -25293, -25308: // errSecAuthFailed, errSecInteractionNotAllowed
		return sdkerrors.PermissionDeniedError(full)
	default:
		return sdkerrors.InternalError(full)
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
func (p *KeychainProvider) GetSecret(
	_ context.Context,
	req *vaultv1.GetSecretRequest,
) (*vaultv1.GetSecretResponse, error) {
	vaultService := req.Opts["service"]
	service, account := parseKeychainPath(req.Path, vaultService)

	secret, err := keychainGetPasswordFunc(service, account)
	if err != nil {
		return nil, err
	}
	return &vaultv1.GetSecretResponse{Secret: secret, ContentType: "text/plain"}, nil
}

// SetSecret stores a secret in the macOS Keychain. The item is created
// with kSecAccessControlUserPresence access control so subsequent reads
// prompt for Touch ID (or passcode if biometry is unavailable). Existing
// items at the same service+account are deleted first because
// SecItemUpdate cannot change access control.
func (p *KeychainProvider) SetSecret(
	_ context.Context, req *vaultv1.SetSecretRequest,
) (*vaultv1.SetSecretResponse, error) {
	if len(req.Secret) == 0 {
		return nil, sdkerrors.InvalidArgumentError("keychain: secret must not be empty")
	}
	vaultService := req.Opts["service"]
	service, account := parseKeychainPath(req.Path, vaultService)
	if err := keychainSetPasswordFunc(service, account, req.Secret); err != nil {
		return nil, err
	}
	return &vaultv1.SetSecretResponse{}, nil
}

// KeyExists probes the Keychain for the given item without retrieving
// the secret data. The probe uses kSecReturnData=false so it never
// triggers a Touch ID or password prompt.
func (p *KeychainProvider) KeyExists(
	_ context.Context, req *vaultv1.KeyExistsRequest,
) (*vaultv1.KeyExistsResponse, error) {
	vaultService := req.Opts["service"]
	service, account := parseKeychainPath(req.Path, vaultService)
	exists, err := keychainExistsFunc(service, account)
	if err != nil {
		return nil, err
	}
	return &vaultv1.KeyExistsResponse{Exists: exists}, nil
}

// HealthCheck confirms the macOS Keychain is accessible.
func (p *KeychainProvider) HealthCheck(
	_ context.Context,
	_ *vaultv1.HealthCheckRequest,
) (*vaultv1.HealthCheckResponse, error) {
	return &vaultv1.HealthCheckResponse{Available: true, Message: "macOS Keychain available"}, nil
}

// Info returns plugin metadata.
func (p *KeychainProvider) Info(_ context.Context, _ *vaultv1.InfoRequest) (*vaultv1.InfoResponse, error) {
	return &vaultv1.InfoResponse{
		Name:                "keychain",
		Version:             "0.2.0",
		Platforms:           []string{platform.Darwin},
		MinLocksmithVersion: "0.4.0",
		MaxLocksmithVersion: sdkversion.Current,
	}, nil
}
