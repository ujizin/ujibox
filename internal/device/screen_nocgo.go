//go:build !(darwin && cgo)

package device

// ScreenCaptureAllowed reports whether the terminal that runs uji has the
// Screen Recording permission. Without cgo uji cannot tell, so known is false.
func ScreenCaptureAllowed() (allowed, known bool) { return false, false }
