package winagent

// Platform abstracts OS-specific operations for workstation control.
// This allows testing on non-Windows platforms with mock implementations.
type Platform interface {
	// LockWorkstation locks the Windows workstation
	LockWorkstation() error

	// ShowWarningNotification displays a toast notification to the user
	ShowWarningNotification(title, message string) error
}
