package cabinetbot

import (
	"fmt"
	"strings"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/telegram/bothost"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/money"
)

// Callback-data prefixes for the cabinet-bot inline keyboards.
const (
	callbackPrefixPlan = "plan:"
	callbackPrefixBuy  = "buy:"
)

// User-facing copy (RU, matching the existing plugin copy style).
const (
	msgNoOffers        = "Пока нет доступных тарифов."
	msgOffersHeader    = "Доступные тарифы — выберите, чтобы посмотреть детали:"
	msgNoSubscriptions = "У вас пока нет подписок. Наберите /plans, чтобы выбрать тариф."
	msgSubsHeader      = "Ваши подписки:"
	msgNoWallets       = "Баланс пока пуст."
	msgWalletsHeader   = "Ваш баланс:"
	msgNoInvoices      = "Неоплаченных счетов нет."
	msgInvoicesHeader  = "Неоплаченные счета:"
)

// formatMoney renders a minor-unit amount (cents/kopecks) as major units,
// delegating to money.Money.String so the sign handling and formatting match
// the rest of the platform.
func formatMoney(amount int64, currency string) string {
	return money.NewMoney(amount, money.Currency(currency)).String()
}

// formatOffers renders the /plans reply: a short header plus one keyboard
// button per offer opening its detail view.
func formatOffers(offers []bothost.TariffOffer) (string, bothost.Keyboard) {
	if len(offers) == 0 {
		return msgNoOffers, bothost.Keyboard{}
	}
	var kb bothost.Keyboard
	for _, o := range offers {
		label := o.Name
		if len(o.Periods) > 0 {
			p := o.Periods[0]
			label = fmt.Sprintf("%s — от %s", o.Name, formatMoney(p.Amount, p.Currency))
		}
		kb.Rows = append(kb.Rows, []bothost.Button{{
			Text:         label,
			CallbackData: callbackPrefixPlan + o.PlanID,
		}})
	}
	return msgOffersHeader, kb
}

// formatOfferDetail renders one offer with a buy button per billing period.
func formatOfferDetail(o bothost.TariffOffer) (string, bothost.Keyboard) {
	var b strings.Builder
	b.WriteString(o.Name)
	if o.Description != "" {
		b.WriteString("\n")
		b.WriteString(o.Description)
	}
	var kb bothost.Keyboard
	for _, p := range o.Periods {
		b.WriteString(fmt.Sprintf("\n• %s — %s", p.Label, formatMoney(p.Amount, p.Currency)))
		kb.Rows = append(kb.Rows, []bothost.Button{{
			Text:         fmt.Sprintf("Купить: %s — %s", p.Label, formatMoney(p.Amount, p.Currency)),
			CallbackData: callbackPrefixBuy + p.PlanID,
		}})
	}
	return b.String(), kb
}

// formatSubscriptions renders /my. planNames maps PlanID→display name
// (best-effort; missing entries fall back to the raw PlanID).
func formatSubscriptions(subs []bothost.Subscription, planNames map[string]string) string {
	if len(subs) == 0 {
		return msgNoSubscriptions
	}
	var b strings.Builder
	b.WriteString(msgSubsHeader)
	for _, s := range subs {
		name := planNames[s.PlanID]
		if name == "" {
			name = s.PlanID
		}
		b.WriteString(fmt.Sprintf("\n• %s — %s", name, s.Status))
		if s.ExpiresAt != "" {
			b.WriteString(" (до " + s.ExpiresAt + ")")
		}
	}
	return b.String()
}

func formatWallets(wallets []bothost.Wallet) string {
	if len(wallets) == 0 {
		return msgNoWallets
	}
	var b strings.Builder
	b.WriteString(msgWalletsHeader)
	for _, w := range wallets {
		b.WriteString(fmt.Sprintf("\n• %s: %s (доступно %s)",
			w.Kind, formatMoney(w.Balance, w.Currency), formatMoney(w.Available, w.Currency)))
	}
	return b.String()
}

func formatInvoices(invoices []bothost.Invoice) string {
	if len(invoices) == 0 {
		return msgNoInvoices
	}
	var b strings.Builder
	b.WriteString(msgInvoicesHeader)
	for _, inv := range invoices {
		b.WriteString(fmt.Sprintf("\n• %s — %s (%s)", inv.ID, formatMoney(inv.Amount, inv.Currency), inv.Status))
	}
	return b.String()
}
