package ai

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
)

// requireRole checks that the user has one of the specified roles.
// super_admin is always allowed.
func (r *ToolRegistry) requireRole(ctx context.Context, userID, companyID int64, roles ...string) error {
	user, err := r.queries.GetUserByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}
	if user.CompanyID != companyID {
		return fmt.Errorf("access denied")
	}
	for _, role := range roles {
		if user.Role == role || user.Role == "super_admin" {
			return nil
		}
	}
	return fmt.Errorf("only %v can perform this action", roles)
}

func toJSON(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func jsonSchema(schema map[string]any) map[string]any {
	return schema
}

func numericToFloat(n pgtype.Numeric) float64 {
	if !n.Valid {
		return 0
	}
	f, err := n.Float64Value()
	if err != nil || !f.Valid {
		return 0
	}
	return f.Float64
}

func numericToString(n pgtype.Numeric) string {
	if !n.Valid {
		return "0"
	}
	f, err := n.Float64Value()
	if err != nil || !f.Valid {
		return "0"
	}
	return fmt.Sprintf("%.1f", f.Float64)
}

// coalesceStrPtr returns the input value if present, otherwise the existing value.
func coalesceStrPtr(input map[string]any, key string, existing *string) *string {
	if v, ok := input[key].(string); ok && v != "" {
		return &v
	}
	return existing
}
