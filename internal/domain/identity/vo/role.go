package vo

// Role represents the authorization level of a platform user.
type Role string

const (
	RoleCustomer Role = "customer"
	RoleReseller Role = "reseller"
	RoleAdmin    Role = "admin"
)
