package vo

// CommissionStatus represents the current state of a commission.
type CommissionStatus string

const (
	// CommissionPending indicates a commission that has not yet been paid out.
	CommissionPending CommissionStatus = "pending"

	// CommissionPaid indicates a commission that has been paid to the reseller.
	CommissionPaid CommissionStatus = "paid"
)
