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

// Personal-cabinet button + /start copy for shop bots.
const (
	// CabinetButtonLabel is the inline-button caption opening the shop cabinet.
	CabinetButtonLabel = "Открыть личный кабинет"
	// CabinetURLHTTPSPrefix gates the Telegram WebApp button (HTTPS-only).
	CabinetURLHTTPSPrefix = "https://"

	// MsgCannotIdentify is sent when the Telegram update carries no user.
	MsgCannotIdentify = "Не удалось вас определить. Попробуйте ещё раз."
	// MsgRegistrationFailed is sent when /start registration errors out.
	MsgRegistrationFailed = "Не удалось завершить регистрацию. Попробуйте позже."
	// MsgWelcomeTemplate greets the user after a successful /start (one %s: name).
	MsgWelcomeTemplate = "Добро пожаловать, %s!\n\nНажмите кнопку ниже, чтобы открыть личный кабинет."
)
