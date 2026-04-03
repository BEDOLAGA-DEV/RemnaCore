package telegram

// Bot command names (without the leading slash).
const (
	CmdStart     = "start"
	CmdPlans     = "plans"
	CmdSubscribe = "subscribe"
	CmdMy        = "my"
	CmdTraffic   = "traffic"
	CmdSupport   = "support"
	CmdReferral  = "referral"
)

// Callback data prefixes used for inline keyboard buttons.
const (
	CallbackPrefixPlan    = "plan:"
	CallbackPrefixAddon   = "addon:"
	CallbackPrefixConfirm = "confirm:"
	CallbackPrefixCancel  = "cancel:"
)

// MaxMessageLength is the Telegram Bot API limit for a single message.
const MaxMessageLength = 4096
