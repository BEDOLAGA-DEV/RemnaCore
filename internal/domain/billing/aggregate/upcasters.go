package aggregate

import "github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"

// defaultPlanTier is the plan_tier value assigned to v1 events that predate
// the plan_tier field introduced in v2.
const defaultPlanTier = "standard"

// SubActivatedV1ToV2 adds the plan_tier field that was introduced in v2.
// v1 events don't have plan_tier; v2 consumers expect it.
type SubActivatedV1ToV2 struct{}

// FromVersion returns the version this upcaster reads (v1).
func (SubActivatedV1ToV2) FromVersion() int { return 1 }

// Upcast transforms the payload from v1 to v2 by adding plan_tier if missing.
func (SubActivatedV1ToV2) Upcast(data map[string]any) map[string]any {
	if _, ok := data["plan_tier"]; !ok {
		data["plan_tier"] = defaultPlanTier
	}
	return data
}

// Compile-time interface check.
var _ domainevent.Upcaster = SubActivatedV1ToV2{}
