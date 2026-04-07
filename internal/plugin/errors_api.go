package plugin

import (
	"errors"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/apierror"
)

// MapToAPIError maps plugin errors to API error codes.
// Returns nil if the error is not a plugin error.
func MapToAPIError(err error) *apierror.Error {
	switch {
	case errors.Is(err, ErrPluginNotFound):
		return apierror.PluginNotFound
	case errors.Is(err, ErrPluginAlreadyExists):
		return apierror.PluginAlreadyExists
	case errors.Is(err, ErrInvalidManifest):
		return apierror.PluginInvalidManifest
	case errors.Is(err, ErrInvalidPluginSlug):
		return apierror.PluginInvalidSlug
	case errors.Is(err, ErrPluginNotEnabled):
		return apierror.PluginNotEnabled
	case errors.Is(err, ErrPluginAlreadyEnabled):
		return apierror.PluginAlreadyEnabled
	case errors.Is(err, ErrHookTimeout):
		return apierror.PluginHookTimeout
	case errors.Is(err, ErrHookHalted):
		return apierror.PluginHookHalted
	case errors.Is(err, ErrPermissionDenied):
		return apierror.PluginPermissionDenied
	case errors.Is(err, ErrStorageQuotaExceeded):
		return apierror.PluginStorageQuota
	case errors.Is(err, ErrHTTPRateLimitExceeded):
		return apierror.PluginHTTPRateLimit
	case errors.Is(err, ErrInternalNetworkAccess):
		return apierror.PluginInternalNetwork
	case errors.Is(err, ErrWASMCompilationFailed):
		return apierror.PluginWASMCompileFail
	case errors.Is(err, ErrNoHandlerForHook):
		return apierror.PluginNoHandler
	case errors.Is(err, ErrSlugMismatch):
		return apierror.PluginSlugMismatch
	case errors.Is(err, ErrPluginNotRunning):
		return apierror.PluginNotRunning
	case errors.Is(err, ErrIncompatibleSDK):
		return apierror.PluginIncompatibleSDK
	case errors.Is(err, ErrPluginDraining):
		return apierror.PluginDraining
	case errors.Is(err, ErrWASMNotFound):
		return apierror.PluginWASMNotFound
	case errors.Is(err, ErrMissingConfig):
		return apierror.PluginMissingConfig
	default:
		return nil
	}
}
