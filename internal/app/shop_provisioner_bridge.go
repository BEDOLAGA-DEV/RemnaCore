package app

import (
	"context"

	identityservice "github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity/service"
	resellerservice "github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/reseller/service"
)

// shopProvisionerBridge adapts the reseller service to identity's primitive port.
type shopProvisionerBridge struct{ rs *resellerservice.ResellerService }

func newShopProvisioner(rs *resellerservice.ResellerService) identityservice.ShopProvisioner {
	return &shopProvisionerBridge{rs: rs}
}

func (b *shopProvisionerBridge) CreateTenant(ctx context.Context, name, domain string, ownerUserID *string) (string, string, error) {
	t, apiKey, err := b.rs.CreateTenant(ctx, name, domain, ownerUserID)
	if err != nil {
		return "", "", err
	}
	return t.ID, apiKey, nil
}

func (b *shopProvisionerBridge) SetTenantOwner(ctx context.Context, tenantID, userID string) error {
	return b.rs.SetTenantOwner(ctx, tenantID, userID)
}

func (b *shopProvisionerBridge) CreateResellerAccount(ctx context.Context, tenantID, userID string, commissionRate int) error {
	_, err := b.rs.CreateResellerAccount(ctx, tenantID, userID, commissionRate)
	return err
}
