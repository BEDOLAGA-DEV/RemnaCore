package vo

// BrandingConfig holds the white-label customisation for a tenant.
// It is an immutable value object -- callers should create a new instance
// rather than mutating fields.
type BrandingConfig struct {
	Logo         string `json:"logo"`
	PrimaryColor string `json:"primary_color"`
	AppName      string `json:"app_name"`
	SupportEmail string `json:"support_email"`
	SupportURL   string `json:"support_url"`
}
