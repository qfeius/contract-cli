package cli

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"cn.qfei/contract-cli/internal/config"
	"cn.qfei/contract-cli/internal/credential"
	"cn.qfei/contract-cli/internal/oauth"
	"github.com/skip2/go-qrcode"
)

type deviceAuthOutput struct {
	Status                  string `json:"status"`
	CredentialScope         string `json:"credential_scope,omitempty"`
	VerificationURIComplete string `json:"verification_uri_complete,omitempty"`
	QRCodePath              string `json:"qr_code_path,omitempty"`
	QRCodeDataURI           string `json:"qr_code_data_uri,omitempty"`
	ExpiresAt               string `json:"expires_at,omitempty"`
	ExpiresAtDisplay        string `json:"expires_at_display,omitempty"`
}

const (
	authorizationQRCodeSize          = 320
	authorizationQRCodeDataURIPrefix = "data:image/png;base64,"
)

type authorizationQRCode struct {
	Path    string
	DataURI string
}

/*
runAuthDeviceInit 发起 Device 授权并保存状态，授权链接须匹配配置环境。
入参 ctx（context.Context）为上下文，args（[]string）为命令参数；返回 error。
*/
func (a *App) runAuthDeviceInit(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("auth init", flag.ContinueOnError)
	flags.SetOutput(a.stderr)
	var profileName, outputFormat string
	var restart bool
	flags.StringVar(&profileName, "profile", "", "profile name")
	flags.StringVar(&outputFormat, "output", "json", "output format: json")
	flags.BoolVar(&restart, "restart", false, "replace an existing Device authorization after explicit user confirmation")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if outputFormat != "json" {
		return errors.New("auth init only supports --output json")
	}

	profile, err := a.loadDeviceAwareProfile(profileName)
	if err != nil {
		return err
	}
	user := profile.Identities.User
	if user.DeviceAuthorizationEndpoint == "" || user.TokenEndpoint == "" || user.DeviceClientID == "" || user.DeviceScope == "" || profile.Resource == "" {
		return fmt.Errorf("device authorization is not configured for profile %q; run `contract-cli config add --env %s --name %s` first", profile.Name, profile.Environment, profile.Name)
	}
	store, err := a.deviceCredentials()
	if err != nil {
		return err
	}
	existing, loadErr := a.loadProductionDeviceCredential(profile, store)
	if loadErr != nil && !errors.Is(loadErr, credential.ErrCredentialNotFound) {
		return loadErr
	}
	release, acquired, err := a.tryDeviceAuthorizationOperation(profile.Name)
	if err != nil {
		return err
	}
	if !acquired {
		return a.writeDeviceAuthOutput(deviceAuthOutput{Status: "busy"})
	}
	defer release()

	existing, loadErr = a.loadProductionDeviceCredential(profile, store)
	if loadErr != nil && !errors.Is(loadErr, credential.ErrCredentialNotFound) {
		return loadErr
	}
	if restart && (errors.Is(loadErr, credential.ErrCredentialNotFound) || existing.Pending == nil) {
		return errors.New("no device authorization to restart; run `contract-cli auth init --output json` without --restart")
	}
	if !restart && existing.Pending == nil {
		if reused, err := a.reuseDeviceAuthorization(ctx, profile, store, existing); reused || err != nil {
			return err
		}
	}
	if !restart && existing.Pending != nil {
		return a.writeExistingDeviceAuthorization(profile.Name, store, existing)
	}

	a.logger.Info("device authorization init started", "profile", profile.Name, "environment", profile.Environment)
	response, err := oauth.StartDeviceAuthorization(ctx, a.httpClient, oauth.DeviceAuthorizationRequest{
		Endpoint: user.DeviceAuthorizationEndpoint,
		ClientID: user.DeviceClientID,
		Scope:    user.DeviceScope,
		Resource: profile.Resource,
	})
	if oauth.IsDeviceGrantRequestNotSent(err) {
		a.logger.Warn("retrying device authorization init after tcp connection failure", "profile", profile.Name, "error", err.Error())
		response, err = oauth.StartDeviceAuthorization(ctx, a.httpClient, oauth.DeviceAuthorizationRequest{
			Endpoint: user.DeviceAuthorizationEndpoint,
			ClientID: user.DeviceClientID,
			Scope:    user.DeviceScope,
			Resource: profile.Resource,
		})
	}
	if err != nil {
		a.logger.Error("device authorization init failed", "profile", profile.Name, "error", err.Error())
		return err
	}
	// 授权链接必须属于 profile 所选环境，防止跨环境登录。
	_, accountOrigin, _ := environmentOrigins(profile.Environment)
	if !isProductionOriginURL(response.VerificationURIComplete, accountOrigin, true) {
		return productionProfileError(profile.Name)
	}
	expiresAt := a.now().Add(time.Duration(response.ExpiresIn) * time.Second)
	profile.DefaultIdentity = config.IdentityUser
	profile.Identities.User.AuthMode = config.UserAuthModeDevice
	profile.Identities.User.Token = nil
	existing.Pending = &credential.PendingTransaction{
		Status: credential.PendingStatusPending, DeviceCode: response.DeviceCode,
		VerificationURIComplete: response.VerificationURIComplete, TokenEndpoint: user.TokenEndpoint,
		ClientID: user.DeviceClientID, ExpiresAt: expiresAt,
	}
	existing.DeviceProfile = snapshotDeviceProfile(profile)
	if err := store.Save(profile.Name, existing); err != nil {
		return err
	}
	qrCode, err := a.writeAuthorizationQRCode(profile.Name, response.VerificationURIComplete)
	if err != nil {
		return err
	}
	if err := a.store.SaveProfile(profile); err != nil {
		return err
	}
	a.logger.Info("device authorization init completed", "profile", profile.Name, "expires_at", expiresAt.Format(time.RFC3339))
	return a.writeDeviceAuthOutput(deviceAuthOutput{
		Status: "pending", VerificationURIComplete: response.VerificationURIComplete,
		QRCodePath: qrCode.Path, QRCodeDataURI: qrCode.DataURI, ExpiresAt: expiresAt.Format(time.RFC3339),
		ExpiresAtDisplay: formatDeviceAuthorizationExpiry(expiresAt),
	})
}

func (a *App) runAuthDeviceComplete(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("auth complete", flag.ContinueOnError)
	flags.SetOutput(a.stderr)
	var profileName, outputFormat string
	flags.StringVar(&profileName, "profile", "", "profile name")
	flags.StringVar(&outputFormat, "output", "json", "output format: json")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if outputFormat != "json" {
		return errors.New("auth complete only supports --output json")
	}
	profile, err := a.loadDeviceAwareProfile(profileName)
	if err != nil {
		return err
	}
	store, err := a.deviceCredentials()
	if err != nil {
		return err
	}
	if _, err := a.loadProductionDeviceCredential(profile, store); err != nil {
		return err
	}
	release, acquired, err := a.tryDeviceAuthorizationOperation(profile.Name)
	if err != nil {
		return err
	}
	if !acquired {
		return a.writeDeviceAuthOutput(deviceAuthOutput{Status: "busy"})
	}
	defer release()

	stored, err := a.loadProductionDeviceCredential(profile, store)
	if err != nil {
		return err
	}
	if stored.Pending == nil {
		return errors.New("no pending device authorization; run `contract-cli auth init --output json` first")
	}
	switch stored.Pending.EffectiveStatus() {
	case credential.PendingStatusChecking:
		stored.Pending.Status = credential.PendingStatusUncertain
		if err := store.Save(profile.Name, stored); err != nil {
			return fmt.Errorf("mark interrupted device authorization as uncertain: %w", err)
		}
		return a.writeDeviceAuthOutput(deviceAuthOutput{Status: "uncertain"})
	case credential.PendingStatusUncertain:
		return a.writeDeviceAuthOutput(deviceAuthOutput{Status: "uncertain"})
	case credential.PendingStatusDenied:
		return a.writeDeviceAuthOutput(deviceAuthOutput{Status: "denied"})
	case credential.PendingStatusExpired:
		return a.writeDeviceAuthOutput(deviceAuthOutput{Status: "expired"})
	case credential.PendingStatusInvalidGrant:
		return a.writeDeviceAuthOutput(deviceAuthOutput{Status: "restart_required"})
	case credential.PendingStatusPending:
		// Only a known pending state may reach the Token Endpoint.
	default:
		return fmt.Errorf("unsupported pending device authorization status %q", stored.Pending.Status)
	}
	if !a.now().Before(stored.Pending.ExpiresAt) {
		stored.Pending.Status = credential.PendingStatusExpired
		if err := store.Save(profile.Name, stored); err != nil {
			return err
		}
		return a.writeDeviceAuthOutput(deviceAuthOutput{Status: "expired"})
	}

	stored.Pending.Status = credential.PendingStatusChecking
	if err := store.Save(profile.Name, stored); err != nil {
		return fmt.Errorf("mark device authorization check in progress: %w", err)
	}
	a.logger.Info("device authorization complete check started", "profile", profile.Name)
	token, err := oauth.CompleteDeviceAuthorization(ctx, a.httpClient, oauth.DeviceTokenRequest{
		Endpoint: stored.Pending.TokenEndpoint, ClientID: stored.Pending.ClientID, DeviceCode: stored.Pending.DeviceCode,
	})
	if err != nil {
		switch {
		case oauth.IsDeviceGrantError(err, "authorization_pending"):
			stored.Pending.Status = credential.PendingStatusPending
			if saveErr := store.Save(profile.Name, stored); saveErr != nil {
				return fmt.Errorf("restore pending device authorization: %w", saveErr)
			}
			return a.writeDeviceAuthOutput(deviceAuthOutput{Status: "pending"})
		case oauth.IsDeviceGrantError(err, "slow_down"):
			stored.Pending.Status = credential.PendingStatusPending
			if saveErr := store.Save(profile.Name, stored); saveErr != nil {
				return fmt.Errorf("restore slowed down device authorization: %w", saveErr)
			}
			return a.writeDeviceAuthOutput(deviceAuthOutput{Status: "pending"})
		case oauth.IsDeviceGrantError(err, "access_denied"):
			stored.Pending.Status = credential.PendingStatusDenied
			if saveErr := store.Save(profile.Name, stored); saveErr != nil {
				return fmt.Errorf("save denied device authorization: %w", saveErr)
			}
			return a.writeDeviceAuthOutput(deviceAuthOutput{Status: "denied"})
		case oauth.IsDeviceGrantError(err, "expired_token"):
			stored.Pending.Status = credential.PendingStatusExpired
			if saveErr := store.Save(profile.Name, stored); saveErr != nil {
				return fmt.Errorf("save expired device authorization: %w", saveErr)
			}
			return a.writeDeviceAuthOutput(deviceAuthOutput{Status: "expired"})
		case oauth.IsDeviceGrantError(err, "invalid_grant"):
			stored.Pending.Status = credential.PendingStatusInvalidGrant
			if saveErr := store.Save(profile.Name, stored); saveErr != nil {
				return fmt.Errorf("save rejected device authorization: %w", saveErr)
			}
			return a.writeDeviceAuthOutput(deviceAuthOutput{Status: "restart_required"})
		default:
			a.logger.Error("device authorization complete check failed", "profile", profile.Name, "error", err.Error())
			stored.Pending.Status = credential.PendingStatusUncertain
			if saveErr := store.Save(profile.Name, stored); saveErr != nil {
				return fmt.Errorf("save uncertain device authorization after failed Token Endpoint request: %w", saveErr)
			}
			return a.writeDeviceAuthOutput(deviceAuthOutput{Status: "uncertain"})
		}
	}
	stored.Pending = nil
	stored.Token = token
	if err := store.Save(profile.Name, stored); err != nil {
		a.logger.Error("save completed device authorization failed", "profile", profile.Name, "error", err.Error())
		return fmt.Errorf(
			"save completed device credential for profile %q: authorization result is uncertain; do not retry auth complete; fix secure credential storage and restart authorization only after user confirmation: %w",
			profile.Name,
			err,
		)
	}
	a.logger.Info("device authorization complete succeeded", "profile", profile.Name, "expires_at", token.Expiry.Format(time.RFC3339))
	return a.writeDeviceAuthOutput(deviceAuthOutput{
		Status: "succeeded", ExpiresAt: token.Expiry.Format(time.RFC3339),
		ExpiresAtDisplay: formatDeviceAuthorizationExpiry(token.Expiry),
	})
}

func (a *App) writeExistingDeviceAuthorization(profileName string, store credential.Store, existing credential.DeviceCredential) error {
	pending := existing.Pending
	if pending == nil {
		return errors.New("device authorization state is missing")
	}
	switch pending.EffectiveStatus() {
	case credential.PendingStatusPending:
		if !a.now().Before(pending.ExpiresAt) {
			pending.Status = credential.PendingStatusExpired
			if err := store.Save(profileName, existing); err != nil {
				return fmt.Errorf("save expired device authorization: %w", err)
			}
			return a.writeDeviceAuthOutput(deviceAuthOutput{Status: "expired"})
		}
		if strings.TrimSpace(pending.VerificationURIComplete) == "" {
			return a.writeDeviceAuthOutput(deviceAuthOutput{Status: "restart_required"})
		}
		qrCode, err := a.writeAuthorizationQRCode(profileName, pending.VerificationURIComplete)
		if err != nil {
			return err
		}
		return a.writeDeviceAuthOutput(deviceAuthOutput{
			Status: "pending", VerificationURIComplete: pending.VerificationURIComplete,
			QRCodePath: qrCode.Path, QRCodeDataURI: qrCode.DataURI, ExpiresAt: pending.ExpiresAt.Format(time.RFC3339),
			ExpiresAtDisplay: formatDeviceAuthorizationExpiry(pending.ExpiresAt),
		})
	case credential.PendingStatusChecking, credential.PendingStatusUncertain:
		return a.writeDeviceAuthOutput(deviceAuthOutput{Status: "uncertain"})
	case credential.PendingStatusDenied:
		return a.writeDeviceAuthOutput(deviceAuthOutput{Status: "denied"})
	case credential.PendingStatusExpired:
		return a.writeDeviceAuthOutput(deviceAuthOutput{Status: "expired"})
	case credential.PendingStatusInvalidGrant:
		return a.writeDeviceAuthOutput(deviceAuthOutput{Status: "restart_required"})
	default:
		return fmt.Errorf("unsupported pending device authorization status %q", pending.Status)
	}
}

func formatDeviceAuthorizationExpiry(expiresAt time.Time) string {
	beijingTime := time.FixedZone("Beijing Time", 8*60*60)
	return expiresAt.In(beijingTime).Format("2006-01-02 15:04:05")
}

func (a *App) tryDeviceAuthorizationOperation(profileName string) (func(), bool, error) {
	lock, err := a.deviceAuthorizationLock(profileName)
	if err != nil {
		return nil, false, err
	}
	locked, err := lock.TryLock()
	if err != nil {
		return nil, false, fmt.Errorf("acquire device authorization lock: %w", err)
	}
	if !locked {
		return nil, false, nil
	}
	return func() {
		if unlockErr := lock.Unlock(); unlockErr != nil {
			a.logger.Error("release device authorization lock failed", "profile", profileName, "error", unlockErr.Error())
		}
	}, true, nil
}

func (a *App) deviceCredentials() (credential.Store, error) {
	if a.credentialStore != nil {
		return a.credentialStore, nil
	}
	runtimeContext, err := a.resolveDeviceRuntime()
	if err != nil {
		return nil, err
	}
	store, err := credential.NewStore(credential.Options{LookupEnv: a.lookupEnv, Keyring: a.credentialKeyring, Runtime: &runtimeContext})
	if err != nil {
		return nil, err
	}
	return store, nil
}

func (a *App) writeAuthorizationQRCode(profileName, verificationURI string) (authorizationQRCode, error) {
	runtimeContext, err := a.resolveDeviceRuntime()
	if err != nil {
		return authorizationQRCode{}, err
	}
	baseDir := ""
	artifactNamespace := profileName
	filePrefix := "device-auth-"
	switch runtimeContext.Kind {
	case credential.DeviceRuntimeDoubaoCloud:
		baseDir = filepath.Join(runtimeContext.Workspace, ".contract-cli", "artifacts")
	case credential.DeviceRuntimeDoubaoWorkTask:
		baseDir = runtimeContext.Workspace
		artifactNamespace = runtimeContext.SessionNamespace + "\x00" + profileName
		filePrefix = "contract-cli-device-auth-"
	case credential.DeviceRuntimeWorkBuddy, credential.DeviceRuntimeLocalUser:
		cacheDir, err := os.UserCacheDir()
		if err != nil {
			return authorizationQRCode{}, fmt.Errorf("resolve qr cache directory: %w", err)
		}
		baseDir = filepath.Join(cacheDir, "contract-cli", "artifacts")
		// Keep the existing WorkBuddy namespace stable for backward compatibility.
		artifactNamespace += "\x00" + runtimeContext.SessionID
	default:
		return authorizationQRCode{}, fmt.Errorf("unsupported Device runtime %q", runtimeContext.Kind)
	}
	if err := os.MkdirAll(baseDir, 0o700); err != nil {
		return authorizationQRCode{}, fmt.Errorf("create qr directory: %w", err)
	}
	digest := sha256.Sum256([]byte(artifactNamespace))
	path := filepath.Join(baseDir, filePrefix+hex.EncodeToString(digest[:8])+".png")
	pngBytes, err := qrcode.Encode(verificationURI, qrcode.Medium, authorizationQRCodeSize)
	if err != nil {
		return authorizationQRCode{}, fmt.Errorf("encode authorization qr code: %w", err)
	}
	if err := os.WriteFile(path, pngBytes, 0o600); err != nil {
		return authorizationQRCode{}, fmt.Errorf("write authorization qr code: %w", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return authorizationQRCode{}, fmt.Errorf("set qr code permissions: %w", err)
	}
	return authorizationQRCode{
		Path:    path,
		DataURI: authorizationQRCodeDataURIPrefix + base64.StdEncoding.EncodeToString(pngBytes),
	}, nil
}
