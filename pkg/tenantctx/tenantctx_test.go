package tenantctx_test

import (
	"context"
	"testing"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/tenantctx"
)

func TestPlatformScopeSentinelValue(t *testing.T) {
	if tenantctx.PlatformScopeSentinel != "*" {
		t.Fatalf("PlatformScopeSentinel = %q, want %q", tenantctx.PlatformScopeSentinel, "*")
	}
}

func TestWithPlatformScopeSetsSentinel(t *testing.T) {
	ctx := tenantctx.WithPlatformScope(context.Background())
	if got := tenantctx.TenantIDFromContext(ctx); got != tenantctx.PlatformScopeSentinel {
		t.Fatalf("TenantIDFromContext after WithPlatformScope = %q, want %q", got, tenantctx.PlatformScopeSentinel)
	}
}

func TestWithoutPlatformScopeIsEmpty(t *testing.T) {
	if got := tenantctx.TenantIDFromContext(context.Background()); got != "" {
		t.Fatalf("TenantIDFromContext on bare context = %q, want empty", got)
	}
}
