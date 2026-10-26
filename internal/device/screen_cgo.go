//go:build darwin && cgo

package device

/*
#cgo LDFLAGS: -framework CoreGraphics
#include <CoreGraphics/CoreGraphics.h>
*/
import "C"

// ScreenCaptureAllowed reports whether the terminal that runs uji has the
// Screen Recording permission. known is false when uji cannot tell.
func ScreenCaptureAllowed() (allowed, known bool) {
	return bool(C.CGPreflightScreenCaptureAccess()), true
}
