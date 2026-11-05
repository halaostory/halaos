package lifecycle

import (
	"github.com/gin-gonic/gin"

	"github.com/halaostory/halaos/internal/access"
	"github.com/halaostory/halaos/internal/auth"
)

// RegisterRoutes registers lifecycle, inbox and custom field routes.
func (h *Handler) RegisterRoutes(protected *gin.RouterGroup) {
	protected.POST("/lifecycle/hire", auth.AdminOnly(), h.Hire)
	protected.GET("/lifecycle/employees/:id", auth.ManagerOrAbove(), h.GetOverview)
	protected.POST("/lifecycle/employees/:id/terminate", auth.AdminOnly(), h.Terminate)
	protected.DELETE("/lifecycle/employees/:id/terminate", auth.AdminOnly(), h.CancelTermination)

	protected.GET("/inbox", auth.AdminOnly(), h.GetInbox)
	protected.GET("/inbox/brief", auth.AdminOnly(), h.GetInboxBrief)

	protected.GET("/custom-fields", auth.ManagerOrAbove(), h.ListCustomFields)
	protected.POST("/custom-fields", auth.AdminOnly(), h.CreateCustomField)
	protected.PUT("/custom-fields/:id", auth.AdminOnly(), h.UpdateCustomField)
	protected.GET("/employees/:id/custom-fields", auth.ManagerOrAbove(), access.EmployeeInCompany(h.queries, "id"), h.GetEmployeeCustomFields)
	protected.PUT("/employees/:id/custom-fields", auth.AdminOnly(), access.EmployeeInCompany(h.queries, "id"), h.UpdateEmployeeCustomFields)
}
