package cli

import (
	"context"

	"cn.qfei/contract-cli/internal/skillsync"
)

// Called only after InstallContext succeeds, so Installed describes committed
// changes rather than the proposed entries returned with an installation error.
func skillInstallationCancellation(ctx context.Context, result skillsync.InstallResult) error {
	if err := ctx.Err(); err != nil {
		if len(result.Installed) > 0 || result.Confirmed {
			return &skillsync.CommittedCancellationError{Cause: err, Result: result}
		}
		return err
	}
	return nil
}
