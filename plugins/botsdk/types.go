package botsdk

// Update is the inbound Telegram update passed to a bot handler. It mirrors the
// host's bothost.Update wire shape.
type Update struct {
	ChatID       int64  `json:"chat_id"`
	From         User   `json:"from"`
	Text         string `json:"text"`
	IsCallback   bool   `json:"is_callback"`
	CallbackID   string `json:"callback_id"`
	CallbackData string `json:"callback_data"`
	MessageID    int    `json:"message_id"`
}

// User is the Telegram user that produced an update.
type User struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Username  string `json:"username"`
}

// DisplayName derives a human display name: first (+ last) name, falling back to
// the username. Mirrors the host's bothost.DisplayName so WASM and built-in bots
// render identically.
func DisplayName(u User) string {
	name := u.FirstName
	if u.LastName != "" {
		name += " " + u.LastName
	}
	if name == "" {
		name = u.Username
	}
	return name
}

// Button is one inline-keyboard button. Exactly one of URL / WebAppURL /
// CallbackData carries the action.
type Button struct {
	Text         string `json:"text"`
	URL          string `json:"url,omitempty"`
	WebAppURL    string `json:"web_app_url,omitempty"`
	CallbackData string `json:"callback_data,omitempty"`
}

// Keyboard is an inline keyboard: rows of buttons.
type Keyboard struct {
	Rows [][]Button `json:"rows"`
}

// TariffPrice is one billing-period option of a tariff offer.
type TariffPrice struct {
	Days     int    `json:"days"`
	Amount   int64  `json:"amount"`
	Currency string `json:"currency"`
	Label    string `json:"label"`
	PlanID   string `json:"plan_id"`
}

// TariffOffer is a purchasable tariff with its billing-period options.
type TariffOffer struct {
	PlanID      string        `json:"plan_id"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Periods     []TariffPrice `json:"periods"`
}

// Subscription is the bot view of a billing subscription.
type Subscription struct {
	ID        string `json:"id"`
	PlanID    string `json:"plan_id"`
	Status    string `json:"status"`
	ExpiresAt string `json:"expires_at,omitempty"`
}

// Invoice is the bot view of a billing invoice.
type Invoice struct {
	ID       string `json:"id"`
	Status   string `json:"status"`
	Amount   int64  `json:"amount"`
	Currency string `json:"currency"`
}

// Wallet is the bot view of a single user balance wallet.
type Wallet struct {
	Kind      string `json:"kind"`
	Currency  string `json:"currency"`
	Balance   int64  `json:"balance"`
	Available int64  `json:"available"`
}

// CheckoutResult is the outcome of a started checkout flow.
type CheckoutResult struct {
	CheckoutURL    string `json:"checkout_url"`
	SubscriptionID string `json:"subscription_id"`
	InvoiceID      string `json:"invoice_id"`
	Provider       string `json:"provider"`
}
