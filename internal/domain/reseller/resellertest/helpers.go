package resellertest

// StrPtr returns a pointer to the given string value.
// Use this in tests instead of defining a local strPtr helper.
func StrPtr(s string) *string { return &s }
