package multisub

import "errors"

var (
	ErrBindingNotFound      = errors.New("remnawave binding not found")
	ErrProvisioningFailed   = errors.New("provisioning failed")
	ErrDeprovisioningFailed = errors.New("deprovisioning failed")
	ErrSyncFailed           = errors.New("sync failed")
	ErrBindingAlreadyActive = errors.New("binding already active")
	ErrRemnawaveUnavailable = errors.New("remnawave panel unavailable")
	ErrMaxBindingsExceeded  = errors.New("maximum remnawave bindings exceeded")
)
