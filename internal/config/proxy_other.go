//go:build !windows

package config

// InitSystemProxy is a no-op on non-Windows platforms as Go reads env vars natively.
func InitSystemProxy() {
}
