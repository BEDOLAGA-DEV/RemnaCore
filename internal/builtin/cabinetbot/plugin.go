// Package cabinetbot is the default built-in Telegram bot plugin. It reproduces
// the original /start behavior — register the Telegram user as a customer of the
// shop and reply with a button opening that shop's personal cabinet — but drives
// it entirely through the bothost op interface, so it is selectable per shop
// (reseller.shop_bots.bot_plugin_slug) and is the default when none is chosen.
package cabinetbot

import (
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/builtin"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/plugin"
)

// Slug identifies the cabinet-bot plugin. NULL/empty bot_plugin_slug resolves
// to this default.
const Slug = "cabinet-bot"

func init() {
	builtin.RegisterPlugin(Plugin())
}

// Plugin returns the built-in plugin definition seeded on startup.
func Plugin() plugin.BuiltInPluginDef {
	return plugin.BuiltInPluginDef{
		Slug:        Slug,
		Name:        "Cabinet Bot",
		Version:     "1.0.0",
		Description: "Registers the Telegram user and opens their shop cabinet (default bot).",
		Author:      "RemnaCore",
		ProvidesBot: true,
	}
}
