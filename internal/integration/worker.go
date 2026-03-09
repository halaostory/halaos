package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/tonypk/aigonhr/internal/integration/connector"
	"github.com/tonypk/aigonhr/internal/store"
)

// ProvisioningWorker polls and executes provisioning jobs.
type ProvisioningWorker struct {
	queries *store.Queries
	svc     *Service
	logger  *slog.Logger
}

// NewProvisioningWorker creates a provisioning worker.
func NewProvisioningWorker(queries *store.Queries, svc *Service, logger *slog.Logger) *ProvisioningWorker {
	return &ProvisioningWorker{
		queries: queries,
		svc:     svc,
		logger:  logger,
	}
}

// Run starts the provisioning worker loop. It polls every interval and executes pending jobs.
func (w *ProvisioningWorker) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	w.logger.Info("provisioning worker started", "interval", interval)

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("provisioning worker stopped")
			return
		case <-ticker.C:
			w.processJobs(ctx)
		}
	}
}

func (w *ProvisioningWorker) processJobs(ctx context.Context) {
	jobs, err := w.queries.ClaimPendingProvisioningJobs(ctx, 10)
	if err != nil {
		w.logger.Error("failed to claim provisioning jobs", "error", err)
		return
	}

	for _, job := range jobs {
		if err := w.executeJob(ctx, job); err != nil {
			w.logger.Error("provisioning job failed",
				"job_id", job.ID,
				"provider", job.Provider,
				"action", job.ActionType,
				"error", err,
			)
			errMsg := err.Error()
			w.queries.FailProvisioningJob(ctx, store.FailProvisioningJobParams{
				ID:           job.ID,
				ErrorMessage: &errMsg,
			})

			w.logAudit(ctx, job, false, nil, err.Error())
		}
	}
}

func (w *ProvisioningWorker) executeJob(ctx context.Context, job store.ProvisioningJob) error {
	// Get connector
	conn, err := w.svc.GetConnector(ctx, job.CompanyID, job.ConnectionID)
	if err != nil {
		return fmt.Errorf("get connector: %w", err)
	}

	// Parse resolved params
	var params map[string]string
	if job.ResolvedParams != nil {
		var rawParams map[string]any
		json.Unmarshal(job.ResolvedParams, &rawParams)
		params = make(map[string]string)
		for k, v := range rawParams {
			params[k] = fmt.Sprintf("%v", v)
		}
	}

	switch job.ActionType {
	case "provision":
		return w.executeProvision(ctx, conn, job, params)
	case "deprovision":
		return w.executeDeprovision(ctx, conn, job, params)
	default:
		return fmt.Errorf("unknown action type: %s", job.ActionType)
	}
}

func (w *ProvisioningWorker) executeProvision(ctx context.Context, conn connector.Connector, job store.ProvisioningJob, params map[string]string) error {
	result, err := conn.Provision(ctx, connector.ProvisionRequest{
		EmployeeID:  job.EmployeeID,
		Email:       params["email"],
		FirstName:   params["first_name"],
		LastName:    params["last_name"],
		DisplayName: params["first_name"] + " " + params["last_name"],
		Department:  params["department"],
		Position:    params["position"],
		Params:      params,
	})
	if err != nil {
		return err
	}

	// Upsert identity
	w.queries.UpsertIntegrationIdentity(ctx, store.UpsertIntegrationIdentityParams{
		CompanyID:        job.CompanyID,
		EmployeeID:       job.EmployeeID,
		ConnectionID:     job.ConnectionID,
		Provider:         job.Provider,
		ExternalID:       result.ExternalID,
		ExternalEmail:    &result.ExternalEmail,
		ExternalUsername: &result.ExternalUsername,
		AccountStatus:    "active",
		Metadata:         json.RawMessage("{}"),
	})

	// Mark job completed
	resultJSON, _ := json.Marshal(result)
	w.queries.CompleteProvisioningJob(ctx, store.CompleteProvisioningJobParams{
		ID:     job.ID,
		Result: resultJSON,
	})

	w.queries.UpdateConnectionLastUsed(ctx, job.ConnectionID)
	w.logAudit(ctx, job, true, result, "")

	w.logger.Info("provisioning completed",
		"job_id", job.ID,
		"provider", job.Provider,
		"external_id", result.ExternalID,
	)

	return nil
}

func (w *ProvisioningWorker) executeDeprovision(ctx context.Context, conn connector.Connector, job store.ProvisioningJob, params map[string]string) error {
	deprovisionMode := params["deprovision_mode"]
	if deprovisionMode == "" {
		deprovisionMode = "disable"
	}

	result, err := conn.Deprovision(ctx, connector.DeprovisionRequest{
		ExternalID:      params["external_id"],
		DeprovisionMode: deprovisionMode,
		Params:          params,
	})
	if err != nil {
		return err
	}

	// Update identity status
	// Find identity by employee + provider and mark deprovisioned
	identities, _ := w.queries.ListEmployeeIntegrations(ctx, store.ListEmployeeIntegrationsParams{
		EmployeeID: job.EmployeeID,
		CompanyID:  job.CompanyID,
	})
	for _, ident := range identities {
		if ident.Provider == job.Provider {
			w.queries.MarkIdentityDeprovisioned(ctx, ident.ID)
		}
	}

	// Mark job completed
	resultJSON, _ := json.Marshal(result)
	w.queries.CompleteProvisioningJob(ctx, store.CompleteProvisioningJobParams{
		ID:     job.ID,
		Result: resultJSON,
	})

	w.queries.UpdateConnectionLastUsed(ctx, job.ConnectionID)
	w.logAudit(ctx, job, true, result, "")

	w.logger.Info("deprovisioning completed",
		"job_id", job.ID,
		"provider", job.Provider,
		"external_id", params["external_id"],
	)

	return nil
}

func (w *ProvisioningWorker) logAudit(ctx context.Context, job store.ProvisioningJob, success bool, result any, errMsg string) {
	var reqSummary, respSummary json.RawMessage
	reqJSON, _ := json.Marshal(map[string]any{
		"action":      job.ActionType,
		"employee_id": job.EmployeeID,
	})
	reqSummary = reqJSON

	if result != nil {
		respJSON, _ := json.Marshal(result)
		respSummary = respJSON
	}

	var errCode, errMessage *string
	if errMsg != "" {
		errCode = strPtr("PROVISIONING_ERROR")
		errMessage = &errMsg
	}

	w.queries.InsertIntegrationAuditLog(ctx, store.InsertIntegrationAuditLogParams{
		CompanyID:       job.CompanyID,
		EmployeeID:      &job.EmployeeID,
		ConnectionID:    pgtype.UUID{Bytes: job.ConnectionID, Valid: true},
		JobID:           pgtype.UUID{Bytes: job.ID, Valid: true},
		Provider:        job.Provider,
		Action:          job.ActionType,
		Success:         success,
		RequestSummary:  reqSummary,
		ResponseSummary: respSummary,
		ErrorCode:       errCode,
		ErrorMessage:    errMessage,
	})
}

func strPtr(s string) *string { return &s }
