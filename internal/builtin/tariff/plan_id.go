package tariff

import (
	"fmt"

	gouuid "github.com/google/uuid"
)

// DerivePlanID computes the billing PlanID for one period of a tariff.
//
// For single-period tariffs (hasPricingPeriods == false) the PlanID equals
// the tariff document ID directly. For multi-period tariffs
// (hasPricingPeriods == true) the PlanID is a deterministic UUIDv5 derived
// from the document ID and the period duration in days.
//
// The derivation is the canonical source of truth shared by syncTariffToPlan
// and the tariff reader: plans.list PlanIDs MUST equal the PlanIDs checkout
// resolves, so both sides must call this function — never inline the logic.
func DerivePlanID(docID string, durationDays int, hasPricingPeriods bool) (string, error) {
	if !hasPricingPeriods {
		return docID, nil
	}
	ns, err := gouuid.Parse(docID)
	if err != nil {
		return "", fmt.Errorf("invalid tariff UUID %q: %w", docID, err)
	}
	return gouuid.NewSHA1(ns, []byte(fmt.Sprintf("period_%d", durationDays))).String(), nil
}
