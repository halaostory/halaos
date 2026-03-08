package ai

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/tonypk/aigonhr/internal/store"
)

func (r *ToolRegistry) toolListTrainings(ctx context.Context, companyID, _ int64, _ map[string]any) (string, error) {
	trainings, err := r.queries.ListTrainings(ctx, store.ListTrainingsParams{
		CompanyID: companyID,
		Limit:     30,
		Offset:    0,
	})
	if err != nil {
		return "", fmt.Errorf("list trainings: %w", err)
	}

	type trainingResult struct {
		ID               int64  `json:"id"`
		Title            string `json:"title"`
		TrainingType     string `json:"training_type"`
		Trainer          string `json:"trainer,omitempty"`
		StartDate        string `json:"start_date"`
		EndDate          string `json:"end_date"`
		Status           string `json:"status"`
		ParticipantCount int64  `json:"participant_count"`
		MaxParticipants  *int32 `json:"max_participants,omitempty"`
	}
	results := make([]trainingResult, len(trainings))
	for i, t := range trainings {
		tr := trainingResult{
			ID:               t.ID,
			Title:            t.Title,
			TrainingType:     t.TrainingType,
			StartDate:        t.StartDate.Format("2006-01-02"),
			Status:           t.Status,
			ParticipantCount: t.ParticipantCount,
			MaxParticipants:  t.MaxParticipants,
		}
		if t.EndDate.Valid {
			tr.EndDate = t.EndDate.Time.Format("2006-01-02")
		}
		if t.Trainer != nil {
			tr.Trainer = *t.Trainer
		}
		results[i] = tr
	}
	return toJSON(map[string]any{"total": len(results), "trainings": results})
}

func (r *ToolRegistry) toolListMyCertifications(ctx context.Context, companyID, userID int64, _ map[string]any) (string, error) {
	emp, err := r.queries.GetEmployeeByUserID(ctx, store.GetEmployeeByUserIDParams{
		UserID:    &userID,
		CompanyID: companyID,
	})
	if err != nil {
		return "", fmt.Errorf("employee not found: %w", err)
	}

	certs, err := r.queries.ListMyCertifications(ctx, emp.ID)
	if err != nil {
		return "", fmt.Errorf("list certifications: %w", err)
	}

	type certResult struct {
		ID           int64  `json:"id"`
		Name         string `json:"name"`
		IssuingBody  string `json:"issuing_body"`
		IssueDate    string `json:"issue_date"`
		ExpiryDate   string `json:"expiry_date,omitempty"`
		Status       string `json:"status"`
		CredentialID string `json:"credential_id,omitempty"`
	}
	results := make([]certResult, len(certs))
	for i, c := range certs {
		cr := certResult{
			ID:        c.ID,
			Name:      c.Name,
			IssueDate: c.IssueDate.Format("2006-01-02"),
			Status:    c.Status,
		}
		if c.IssuingBody != nil {
			cr.IssuingBody = *c.IssuingBody
		}
		if c.ExpiryDate.Valid {
			cr.ExpiryDate = c.ExpiryDate.Time.Format("2006-01-02")
		}
		if c.CredentialID != nil {
			cr.CredentialID = *c.CredentialID
		}
		results[i] = cr
	}

	// Also check expiring certs
	expiring, _ := r.queries.ListExpiringCertifications(ctx, companyID)
	var expiringCount int
	for _, e := range expiring {
		if e.EmployeeID == emp.ID {
			expiringCount++
		}
	}

	return toJSON(map[string]any{
		"total":          len(results),
		"certifications": results,
		"expiring_soon":  expiringCount,
	})
}

func (r *ToolRegistry) toolEnrollInTraining(ctx context.Context, companyID, userID int64, input map[string]any) (string, error) {
	emp, err := r.queries.GetEmployeeByUserID(ctx, store.GetEmployeeByUserIDParams{
		UserID:    &userID,
		CompanyID: companyID,
	})
	if err != nil {
		return "", fmt.Errorf("employee not found: %w", err)
	}

	trainingID, ok := input["training_id"].(float64)
	if !ok || trainingID <= 0 {
		return "", fmt.Errorf("training_id is required")
	}

	// Verify training exists and is open
	training, err := r.queries.GetTraining(ctx, store.GetTrainingParams{
		ID:        int64(trainingID),
		CompanyID: companyID,
	})
	if err != nil {
		return "", fmt.Errorf("training not found: %w", err)
	}
	if training.Status != "scheduled" && training.Status != "in_progress" {
		return toJSON(map[string]any{
			"success": false,
			"message": fmt.Sprintf("Training is '%s' and not accepting enrollments.", training.Status),
		})
	}

	participant, err := r.queries.AddTrainingParticipant(ctx, store.AddTrainingParticipantParams{
		TrainingID: int64(trainingID),
		EmployeeID: emp.ID,
	})
	if err != nil {
		return "", fmt.Errorf("enroll in training: %w", err)
	}

	return toJSON(map[string]any{
		"success":        true,
		"participant_id": participant.ID,
		"training_title": training.Title,
		"message":        fmt.Sprintf("Successfully enrolled in '%s'.", training.Title),
	})
}

func (r *ToolRegistry) toolMarkTrainingComplete(ctx context.Context, companyID, userID int64, input map[string]any) (string, error) {
	if err := r.requireRole(ctx, userID, companyID, "admin"); err != nil {
		return "", err
	}

	participantID, ok := input["participant_id"].(float64)
	if !ok || participantID <= 0 {
		return "", fmt.Errorf("participant_id is required")
	}

	trainingID, ok := input["training_id"].(float64)
	if !ok || trainingID <= 0 {
		return "", fmt.Errorf("training_id is required")
	}

	var score pgtype.Numeric
	if s, ok := input["score"].(float64); ok && s >= 0 {
		_ = score.Scan(fmt.Sprintf("%.0f", s))
	}

	var feedback *string
	if f, ok := input["feedback"].(string); ok && f != "" {
		feedback = &f
	}

	participant, err := r.queries.UpdateParticipantStatus(ctx, store.UpdateParticipantStatusParams{
		ID:         int64(participantID),
		TrainingID: int64(trainingID),
		Status:     "completed",
		Score:      score,
		Feedback:   feedback,
	})
	if err != nil {
		return "", fmt.Errorf("mark training complete: %w", err)
	}

	return toJSON(map[string]any{
		"success":        true,
		"participant_id": participant.ID,
		"status":         participant.Status,
		"message":        "Training marked as completed.",
	})
}
