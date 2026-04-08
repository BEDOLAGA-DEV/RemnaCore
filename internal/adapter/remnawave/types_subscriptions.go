package remnawave

import (
	"encoding/json"
	"time"
)

// RemnawaveSubscription represents a subscription as returned by the
// Remnawave panel's subscription endpoints.
type RemnawaveSubscription struct {
	UUID            string          `json:"uuid"`
	Username        string          `json:"username"`
	ShortUUID       string          `json:"shortUuid"`
	SubscriptionURL string          `json:"subscriptionUrl"`
	Status          string          `json:"status"`
	CreatedAt       time.Time       `json:"createdAt"`
	UpdatedAt       time.Time       `json:"updatedAt"`
	ExpireAt        time.Time       `json:"expireAt"`
	Extra           json.RawMessage `json:"extra,omitempty"`
}
