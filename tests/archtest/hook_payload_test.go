package archtest

import (
	"reflect"
	"testing"

	billingservice "github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/service"
	multisubservice "github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub/service"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/sdk"
	"github.com/stretchr/testify/assert"
)

// TestHookPayloadFieldsMatchSDK verifies that domain-local hook payload types
// (in billing/service and multisub/service) have JSON tags that are a subset
// of the corresponding SDK types. If the two drift apart, plugins will receive
// unexpected field names or miss fields they depend on.
//
// Direction: for every field in the domain payload, a matching JSON tag must
// exist in the SDK type. The SDK may have additional fields (e.g., enrichment
// fields added for plugin convenience) — that is acceptable. The invariant is
// that the domain never sends a JSON key that the SDK does not declare.
func TestHookPayloadFieldsMatchSDK(t *testing.T) {
	testCases := []struct {
		name       string
		domainType reflect.Type
		sdkType    reflect.Type
	}{
		// =====================================================================
		// Billing — subscription sync hook payloads
		// =====================================================================
		{
			name:       "CancellingPayload",
			domainType: reflect.TypeOf(billingservice.CancellingPayload{}),
			sdkType:    reflect.TypeOf(sdk.SubCancellingRequest{}),
		},
		{
			name:       "CancellingResponse",
			domainType: reflect.TypeOf(billingservice.CancellingResponse{}),
			sdkType:    reflect.TypeOf(sdk.SubCancellingResponse{}),
		},
		{
			name:       "RenewingPayload",
			domainType: reflect.TypeOf(billingservice.RenewingPayload{}),
			sdkType:    reflect.TypeOf(sdk.SubRenewingRequest{}),
		},
		{
			name:       "RenewingResponse",
			domainType: reflect.TypeOf(billingservice.RenewingResponse{}),
			sdkType:    reflect.TypeOf(sdk.SubRenewingResponse{}),
		},
		{
			name:       "UpgradingPayload",
			domainType: reflect.TypeOf(billingservice.UpgradingPayload{}),
			sdkType:    reflect.TypeOf(sdk.SubUpgradingRequest{}),
		},
		{
			name:       "UpgradingResponse",
			domainType: reflect.TypeOf(billingservice.UpgradingResponse{}),
			sdkType:    reflect.TypeOf(sdk.SubUpgradingResponse{}),
		},

		// =====================================================================
		// Billing — checkout hook payloads
		// =====================================================================
		{
			name:       "CheckoutValidatingPayload",
			domainType: reflect.TypeOf(billingservice.CheckoutValidatingPayload{}),
			sdkType:    reflect.TypeOf(sdk.CheckoutValidatingRequest{}),
		},
		{
			name:       "CheckoutValidatingResponse",
			domainType: reflect.TypeOf(billingservice.CheckoutValidatingResponse{}),
			sdkType:    reflect.TypeOf(sdk.CheckoutValidatingResponse{}),
		},
		{
			name:       "SubCreatingPayload",
			domainType: reflect.TypeOf(billingservice.SubCreatingPayload{}),
			sdkType:    reflect.TypeOf(sdk.SubCreatingRequest{}),
		},
		{
			name:       "SubCreatingResponse",
			domainType: reflect.TypeOf(billingservice.SubCreatingResponse{}),
			sdkType:    reflect.TypeOf(sdk.SubCreatingResponse{}),
		},
		{
			name:       "CheckoutCompletedPayload",
			domainType: reflect.TypeOf(billingservice.CheckoutCompletedPayload{}),
			sdkType:    reflect.TypeOf(sdk.CheckoutCompletedNotification{}),
		},

		// =====================================================================
		// Billing — async notification payloads
		// =====================================================================
		{
			name:       "ActivatedPayload",
			domainType: reflect.TypeOf(billingservice.ActivatedPayload{}),
			sdkType:    reflect.TypeOf(sdk.SubActivatedNotification{}),
		},

		// =====================================================================
		// MultiSub — traffic lifecycle hook payloads
		// =====================================================================
		{
			name:       "SubLimitingPayload",
			domainType: reflect.TypeOf(multisubservice.SubLimitingPayload{}),
			sdkType:    reflect.TypeOf(sdk.SubLimitingRequest{}),
		},
		{
			name:       "SubLimitingResponse",
			domainType: reflect.TypeOf(multisubservice.SubLimitingResponse{}),
			sdkType:    reflect.TypeOf(sdk.SubLimitingResponse{}),
		},
		{
			name:       "SubUnlimitingPayload",
			domainType: reflect.TypeOf(multisubservice.SubUnlimitingPayload{}),
			sdkType:    reflect.TypeOf(sdk.SubUnlimitingRequest{}),
		},
		{
			name:       "SubUnlimitingResponse",
			domainType: reflect.TypeOf(multisubservice.SubUnlimitingResponse{}),
			sdkType:    reflect.TypeOf(sdk.SubUnlimitingResponse{}),
		},
		{
			name:       "SubLimitedNotification",
			domainType: reflect.TypeOf(multisubservice.SubLimitedNotification{}),
			sdkType:    reflect.TypeOf(sdk.SubLimitedNotification{}),
		},
		{
			name:       "SubUnlimitedNotification",
			domainType: reflect.TypeOf(multisubservice.SubUnlimitedNotification{}),
			sdkType:    reflect.TypeOf(sdk.SubUnlimitedNotification{}),
		},
		{
			name:       "SubTrafficWarningNotification",
			domainType: reflect.TypeOf(multisubservice.SubTrafficWarningNotification{}),
			sdkType:    reflect.TypeOf(sdk.SubTrafficWarningNotification{}),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assertDomainFieldsCoveredBySDK(t, tc.domainType, tc.sdkType)
		})
	}
}

// assertDomainFieldsCoveredBySDK verifies that every exported field in
// domainType with a non-empty JSON tag has a matching JSON tag in sdkType.
// Matching is by the primary JSON tag name (before the first comma).
func assertDomainFieldsCoveredBySDK(t *testing.T, domainType, sdkType reflect.Type) {
	t.Helper()

	// Build a set of JSON tag names from the SDK type for O(1) lookup.
	sdkTags := make(map[string]string) // json tag name -> field name
	for i := range sdkType.NumField() {
		sf := sdkType.Field(i)
		tag := primaryJSONTag(sf)
		if tag == "" || tag == "-" {
			continue
		}
		sdkTags[tag] = sf.Name
	}

	for i := range domainType.NumField() {
		df := domainType.Field(i)
		tag := primaryJSONTag(df)
		if tag == "" || tag == "-" {
			continue
		}

		assert.Contains(t, sdkTags, tag,
			"domain field %s.%s (json:%q) has no matching JSON tag in %s",
			domainType.Name(), df.Name, tag, sdkType.Name())
	}
}

// primaryJSONTag extracts the primary JSON tag name (before the first comma)
// from a struct field's "json" tag. Returns "" if no tag is set.
func primaryJSONTag(f reflect.StructField) string {
	tag := f.Tag.Get("json")
	if tag == "" {
		return ""
	}
	// Split on comma to handle "name,omitempty".
	for i := range len(tag) {
		if tag[i] == ',' {
			return tag[:i]
		}
	}
	return tag
}
