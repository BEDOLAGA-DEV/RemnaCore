package tariff

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestTariffRoutes_AdminOnlyHaveRequiredPermission asserts that every
// AdminOnly route produced by tariffRoutes() carries a non-empty
// RequiredPermission. This is a compile-time-equivalent guard: if a new
// AdminOnly route is added without updating the permission switch, this test
// will catch it.
func TestTariffRoutes_AdminOnlyHaveRequiredPermission(t *testing.T) {
	routes := tariffRoutes()
	for _, r := range routes {
		if !r.AdminOnly {
			continue
		}
		t.Run(fmt.Sprintf("%s %s", r.Method, r.Path), func(t *testing.T) {
			assert.NotEmpty(t, r.RequiredPermission,
				"AdminOnly route must have a RequiredPermission")
		})
	}
}

// TestTariffRoutes_PublicRoutesHaveNoPermission asserts that public routes
// (no auth required) do not carry a RequiredPermission, keeping the contract
// clean for the route registrar.
func TestTariffRoutes_PublicRoutesHaveNoPermission(t *testing.T) {
	routes := tariffRoutes()
	for _, r := range routes {
		if !r.Public {
			continue
		}
		t.Run(fmt.Sprintf("%s %s", r.Method, r.Path), func(t *testing.T) {
			assert.Empty(t, r.RequiredPermission,
				"Public route must not have a RequiredPermission")
		})
	}
}
