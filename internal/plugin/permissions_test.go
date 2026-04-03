package plugin

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHasPermission_DirectMatch(t *testing.T) {
	pc := &PermissionChecker{}
	p := &Plugin{
		Permissions: []PermissionScope{PermBillingRead, PermUsersRead},
	}

	assert.True(t, pc.HasPermission(p, PermBillingRead))
	assert.True(t, pc.HasPermission(p, PermUsersRead))
}

func TestHasPermission_WriteImpliesRead(t *testing.T) {
	pc := &PermissionChecker{}
	p := &Plugin{
		Permissions: []PermissionScope{PermBillingWrite, PermUsersWrite, PermVPNWrite},
	}

	// Write implies read for same resource.
	assert.True(t, pc.HasPermission(p, PermBillingRead))
	assert.True(t, pc.HasPermission(p, PermUsersRead))
	assert.True(t, pc.HasPermission(p, PermVPNRead))
}

func TestHasPermission_ReadWriteImpliesRead(t *testing.T) {
	pc := &PermissionChecker{}
	p := &Plugin{
		Permissions: []PermissionScope{PermStorageWrite}, // storage:readwrite
	}

	assert.True(t, pc.HasPermission(p, PermStorageRead))
}

func TestHasPermission_Denied(t *testing.T) {
	pc := &PermissionChecker{}
	p := &Plugin{
		Permissions: []PermissionScope{PermBillingRead},
	}

	assert.False(t, pc.HasPermission(p, PermBillingWrite))
	assert.False(t, pc.HasPermission(p, PermUsersRead))
	assert.False(t, pc.HasPermission(p, PermPaymentWrite))
	assert.False(t, pc.HasPermission(p, PermVPNWrite))
}

func TestHasPermission_EmptyPermissions(t *testing.T) {
	pc := &PermissionChecker{}
	p := &Plugin{
		Permissions: nil,
	}

	assert.False(t, pc.HasPermission(p, PermBillingRead))
}

func TestValidateHTTPRequest_ExactMatch(t *testing.T) {
	pc := &PermissionChecker{}
	p := &Plugin{
		Manifest: &Manifest{
			Permissions: ManifestPermissions{
				HTTP: []string{"https://api.example.com/v1/webhook"},
			},
		},
	}

	assert.True(t, pc.ValidateHTTPRequest(p, "https://api.example.com/v1/webhook"))
}

func TestValidateHTTPRequest_WildcardMatch(t *testing.T) {
	pc := &PermissionChecker{}
	p := &Plugin{
		Manifest: &Manifest{
			Permissions: ManifestPermissions{
				HTTP: []string{"https://api.stripe.com/*"},
			},
		},
	}

	assert.True(t, pc.ValidateHTTPRequest(p, "https://api.stripe.com/v1/charges"))
	assert.True(t, pc.ValidateHTTPRequest(p, "https://api.stripe.com/v2/events"))
	// The host prefix itself should also match.
	assert.True(t, pc.ValidateHTTPRequest(p, "https://api.stripe.com"))
}

func TestValidateHTTPRequest_Denied(t *testing.T) {
	pc := &PermissionChecker{}
	p := &Plugin{
		Manifest: &Manifest{
			Permissions: ManifestPermissions{
				HTTP: []string{"https://api.stripe.com/*"},
			},
		},
	}

	assert.False(t, pc.ValidateHTTPRequest(p, "https://evil.com/callback"))
	assert.False(t, pc.ValidateHTTPRequest(p, "https://api.stripe.com.evil.com/phish"))
}

func TestValidateHTTPRequest_NilManifest(t *testing.T) {
	pc := &PermissionChecker{}
	p := &Plugin{
		Manifest: nil,
	}

	assert.False(t, pc.ValidateHTTPRequest(p, "https://any-url.com"))
}

func TestValidateHTTPRequest_EmptyAllowlist(t *testing.T) {
	pc := &PermissionChecker{}
	p := &Plugin{
		Manifest: &Manifest{
			Permissions: ManifestPermissions{
				HTTP: nil,
			},
		},
	}

	assert.False(t, pc.ValidateHTTPRequest(p, "https://any-url.com"))
}

func TestImpliesPermission_CrossResource(t *testing.T) {
	// billing:write should NOT imply users:read.
	assert.False(t, impliesPermission(PermBillingWrite, PermUsersRead))
}
