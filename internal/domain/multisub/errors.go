package multisub

import "errors"

var (
	ErrBindingNotFound       = errors.New("remnawave binding not found")
	ErrProvisioningFailed    = errors.New("provisioning failed")
	ErrDeprovisioningFailed  = errors.New("deprovisioning failed")
	ErrSyncFailed            = errors.New("sync failed")
	ErrBindingAlreadyActive  = errors.New("binding already active")
	ErrBindingAlreadyLimited = errors.New("binding is already limited")
	ErrBindingNotLimited     = errors.New("binding is not limited")
	ErrRemnawaveUnavailable  = errors.New("remnawave panel unavailable")
	ErrMaxBindingsExceeded   = errors.New("maximum remnawave bindings exceeded")
	ErrSagaNotFound          = errors.New("saga instance not found")
	ErrSagaAlreadyExists     = errors.New("saga instance already exists for correlation")
	ErrNoVPNPlugin           = errors.New("no VPN provider plugin configured")
)
