package multisub

// AddonSnapshotType categorises an addon by what it provides.
// This mirrors billing's AddonType but belongs to the multisub context's
// Anti-Corruption Layer — multisub never depends on billing/aggregate directly.
type AddonSnapshotType string

const (
	AddonSnapshotTraffic AddonSnapshotType = "traffic"
	AddonSnapshotNodes   AddonSnapshotType = "nodes"
	AddonSnapshotFeature AddonSnapshotType = "feature"
)

// PlanSnapshot is the multisub context's read-only view of a billing plan.
// It is an Anti-Corruption Layer type: the multisub bounded context receives
// this instead of billing/aggregate.Plan, ensuring that changes in the billing
// domain do not propagate into multisub.
//
// Translation from billing.Plan to PlanSnapshot happens at the adapter boundary
// (e.g. the NATS billing event consumer).
type PlanSnapshot struct {
	ID                   string
	TrafficLimitBytes    int64
	MaxRemnawaveBindings int
	Addons               []AddonSnapshot
}

// AddonSnapshot is the multisub context's read-only view of a billing addon.
type AddonSnapshot struct {
	ID                string
	Name              string
	Type              AddonSnapshotType
	ExtraTrafficBytes int64
	ExtraNodes        []string
}
