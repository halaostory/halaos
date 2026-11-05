// Package devices tracks company hardware and drives MDM actions (lock, wipe,
// return) from the employee lifecycle, like Warp's device management: when
// someone is terminated their devices are locked automatically and a return
// is requested; a wipe always needs an explicit approval.
package devices

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/halaostory/halaos/internal/integration/crypto"
	"github.com/halaostory/halaos/internal/store"
)

// Service manages devices and MDM actions.
type Service struct {
	q      *store.Queries
	logger *slog.Logger
	enc    *crypto.CredentialEncryptor
	http   *http.Client
}

// NewService builds the device service. enc may be nil (no MDM secrets).
func NewService(q *store.Queries, logger *slog.Logger, enc *crypto.CredentialEncryptor) *Service {
	return &Service{q: q, logger: logger, enc: enc, http: &http.Client{Timeout: 30 * time.Second}}
}

// NewServiceFromKey builds the service from a base64 encryption key.
func NewServiceFromKey(q *store.Queries, logger *slog.Logger, key string) *Service {
	var enc *crypto.CredentialEncryptor
	if key != "" {
		enc, _ = crypto.NewCredentialEncryptor(key)
	}
	return NewService(q, logger, enc)
}

// ErrInvalid is a user-facing problem.
var ErrInvalid = errors.New("invalid")

// RequestAction queues an MDM action. Wipe requires approval unless approvedBy is set.
func (s *Service) RequestAction(ctx context.Context, companyID, deviceID int64, action, trigger string, requestedBy *int64, preApproved bool) (*store.DeviceAction, error) {
	dev, err := s.q.GetDevice(ctx, store.GetDeviceParams{ID: deviceID, CompanyID: companyID})
	if err != nil {
		return nil, fmt.Errorf("%w: device not found", ErrInvalid)
	}
	switch action {
	case "lock", "unlock", "wipe", "return_request":
	default:
		return nil, fmt.Errorf("%w: unknown action", ErrInvalid)
	}
	status := "pending"
	if action == "wipe" && !preApproved {
		status = "requires_approval"
	}
	if action == "return_request" {
		status = "manual" // a person collects the device
	}
	a, err := s.q.CreateDeviceAction(ctx, store.CreateDeviceActionParams{
		CompanyID: companyID, DeviceID: deviceID, EmployeeID: dev.AssignedEmployeeID, Action: action,
		Status: status, Trigger: trigger, RequestedBy: requestedBy,
	})
	if err != nil {
		return nil, err
	}
	if action == "return_request" {
		_ = s.q.SetDeviceStatus(ctx, store.SetDeviceStatusParams{ID: deviceID, Status: "return_pending"})
	}
	return &a, nil
}

// OnTermination locks every device assigned to the employee (if the company
// enabled it) and requests their return. Returns the number of devices.
func (s *Service) OnTermination(ctx context.Context, companyID, employeeID int64, actor *int64) (int, error) {
	devs, err := s.q.ListAssignedDevicesForEmployee(ctx, store.ListAssignedDevicesForEmployeeParams{AssignedEmployeeID: &employeeID, CompanyID: companyID})
	if err != nil {
		return 0, err
	}
	autoLock := true
	if conn, err := s.q.GetMDMConnection(ctx, companyID); err == nil {
		autoLock = conn.AutoLockOnTermination
	}
	for _, d := range devs {
		if autoLock && d.Status != "locked" {
			if _, err := s.RequestAction(ctx, companyID, d.ID, "lock", "termination", actor, false); err != nil {
				return 0, err
			}
		}
		if _, err := s.RequestAction(ctx, companyID, d.ID, "return_request", "termination", actor, false); err != nil {
			return 0, err
		}
	}
	return len(devs), nil
}

// MDMRequest is posted to the company's MDM gateway.
type MDMRequest struct {
	ActionID    int64  `json:"action_id"`
	Action      string `json:"action"`
	AssetTag    string `json:"asset_tag"`
	SerialNo    string `json:"serial_no"`
	MDMDeviceID string `json:"mdm_device_id"`
	OS          string `json:"os"`
	Reason      string `json:"reason"`
}

// ProcessQueue sends pending actions to the MDM gateway. Without a gateway
// the action becomes a manual IT task.
func (s *Service) ProcessQueue(ctx context.Context) int {
	if err := s.q.RecoverStuckDeviceActions(ctx); err != nil {
		s.logger.Warn("devices: recover stuck", "error", err)
	}
	acts, err := s.q.ClaimPendingDeviceActions(ctx, 20)
	if err != nil {
		s.logger.Error("devices: claim", "error", err)
		return 0
	}
	for _, a := range acts {
		status, result, errMsg := s.execute(ctx, a)
		var r, e *string
		if result != "" {
			r = &result
		}
		if errMsg != "" {
			e = &errMsg
		}
		if err := s.q.FinishDeviceAction(ctx, store.FinishDeviceActionParams{ID: a.ID, Status: status, Result: r, ErrorMessage: e}); err != nil {
			s.logger.Error("devices: finish action", "action_id", a.ID, "error", err)
		}
		if status == "completed" {
			switch a.Action {
			case "lock":
				_ = s.q.SetDeviceStatus(ctx, store.SetDeviceStatusParams{ID: a.DeviceID, Status: "locked"})
			case "wipe":
				_ = s.q.SetDeviceStatus(ctx, store.SetDeviceStatusParams{ID: a.DeviceID, Status: "wiped"})
			}
		}
	}
	return len(acts)
}

func (s *Service) execute(ctx context.Context, a store.DeviceAction) (status, result, errMsg string) {
	dev, err := s.q.GetDevice(ctx, store.GetDeviceParams{ID: a.DeviceID, CompanyID: a.CompanyID})
	if err != nil {
		return "failed", "", "device not found"
	}
	conn, err := s.q.GetMDMConnection(ctx, a.CompanyID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "manual", "", "no MDM connected — perform this action in your device management console and mark it done"
	}
	if err != nil {
		return "failed", "", err.Error()
	}
	secret := ""
	if conn.SecretEnc != "" && s.enc != nil {
		if ct, err := hex.DecodeString(conn.SecretEnc); err == nil {
			if pt, err := s.enc.Decrypt(ct); err == nil {
				secret = string(pt)
			}
		}
	}
	body, _ := json.Marshal(MDMRequest{ActionID: a.ID, Action: a.Action, AssetTag: dev.AssetTag, SerialNo: dev.SerialNo,
		MDMDeviceID: deref(dev.MdmDeviceID), OS: dev.Os, Reason: a.Trigger})
	if !strings.HasPrefix(conn.Url, "https://") && !strings.HasPrefix(conn.Url, "http://127.0.0.1") && !strings.HasPrefix(conn.Url, "http://localhost") {
		return "failed", "", "MDM gateway url must use https"
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, conn.Url, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", fmt.Sprintf("halaos-device-action-%d", a.ID))
	if secret != "" {
		m := hmac.New(sha256.New, []byte(secret))
		m.Write(body)
		req.Header.Set("X-HalaOS-Signature", "sha256="+hex.EncodeToString(m.Sum(nil)))
	}
	resp, err := s.http.Do(req)
	if err != nil {
		return "failed", "", err.Error()
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode >= 300 {
		return "failed", "", fmt.Sprintf("MDM gateway returned %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	return "completed", strings.TrimSpace(string(raw)), ""
}

// SaveConnection stores the MDM gateway (secret encrypted).
func (s *Service) SaveConnection(ctx context.Context, companyID int64, provider, url, secret string, autoLock bool) (*store.MdmConnection, error) {
	if !strings.HasPrefix(url, "https://") && !strings.HasPrefix(url, "http://127.0.0.1") && !strings.HasPrefix(url, "http://localhost") {
		return nil, fmt.Errorf("%w: url must use https", ErrInvalid)
	}
	enc := ""
	if secret != "" {
		if s.enc == nil {
			return nil, fmt.Errorf("%w: INTEGRATION_ENCRYPTION_KEY is not configured", ErrInvalid)
		}
		ct, err := s.enc.Encrypt([]byte(secret))
		if err != nil {
			return nil, err
		}
		enc = hex.EncodeToString(ct)
	}
	if provider == "" {
		provider = "webhook"
	}
	c, err := s.q.UpsertMDMConnection(ctx, store.UpsertMDMConnectionParams{CompanyID: companyID, Provider: provider, Url: url, SecretEnc: enc, AutoLockOnTermination: autoLock})
	if err != nil {
		return nil, err
	}
	c.SecretEnc = ""
	return &c, nil
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
