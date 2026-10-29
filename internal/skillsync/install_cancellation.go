package skillsync

import "fmt"

// CommittedCancellationError reports a cancellation observed after the atomic
// installation commit. Its files and registration must not be rolled back.
type CommittedCancellationError struct {
	Cause  error
	Result InstallResult
}

func (err *CommittedCancellationError) Error() string {
	return fmt.Sprintf("Skill installation was committed before cancellation; installed files and registration were retained: %v", err.Cause)
}

func (err *CommittedCancellationError) Unwrap() error { return err.Cause }
