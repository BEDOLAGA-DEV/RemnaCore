package service

import "context"

// ShopProvisioner is the narrow, primitive-typed port the IdentityAdminService
// uses to provision shops. Implemented by a bridge over reseller.ResellerService
// (in internal/app) so the identity domain does not import the reseller domain.
type ShopProvisioner interface {
	CreateTenant(ctx context.Context, name, domain string, ownerUserID *string) (tenantID, apiKey string, err error)
	SetTenantOwner(ctx context.Context, tenantID, userID string) error
	CreateResellerAccount(ctx context.Context, tenantID, userID string, commissionRate int) error
}
