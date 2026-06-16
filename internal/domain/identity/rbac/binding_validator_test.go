package rbac_test

import (
	"testing"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity/rbac"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateBinding(t *testing.T) {
	shopA := "11111111-1111-1111-1111-111111111111"
	shopB := "22222222-2222-2222-2222-222222222222"

	t.Run("global role with NULL tenant is valid", func(t *testing.T) {
		role := rbac.Role{ScopeKind: rbac.ScopeGlobal}
		require.NoError(t, rbac.ValidateBinding(role, nil))
	})
	t.Run("global role with non-NULL tenant is rejected", func(t *testing.T) {
		role := rbac.Role{ScopeKind: rbac.ScopeGlobal}
		assert.ErrorIs(t, rbac.ValidateBinding(role, &shopA), rbac.ErrInvalidBindingScope)
	})
	t.Run("shop role with non-NULL tenant is valid", func(t *testing.T) {
		role := rbac.Role{ScopeKind: rbac.ScopeShop}
		require.NoError(t, rbac.ValidateBinding(role, &shopA))
	})
	t.Run("shop role with NULL tenant is rejected", func(t *testing.T) {
		role := rbac.Role{ScopeKind: rbac.ScopeShop}
		assert.ErrorIs(t, rbac.ValidateBinding(role, nil), rbac.ErrInvalidBindingScope)
	})
	t.Run("shop-local custom role tenant must equal binding tenant", func(t *testing.T) {
		role := rbac.Role{ScopeKind: rbac.ScopeShop, TenantID: &shopA} // custom role pinned to shop A
		assert.ErrorIs(t, rbac.ValidateBinding(role, &shopB), rbac.ErrInvalidBindingScope)
		require.NoError(t, rbac.ValidateBinding(role, &shopA))
	})
}
