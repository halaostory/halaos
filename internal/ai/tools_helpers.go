package ai

import (
	"context"
	"fmt"
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
