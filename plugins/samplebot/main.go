// Package main implements the sample WASM bot plugin for RemnaCore. It is also
// the reference example for the guest SDK (github.com/BEDOLAGA-DEV/RemnaCore/
// plugins/botsdk): all host communication goes through botsdk, so this file is
// pure bot logic with no wasmimport / memory / envelope boilerplate.
//
// Build with:
//
//	GOOS=wasip1 GOARCH=wasm GOWORK=off go build -o samplebot.wasm .
package main

import (
	botsdk "github.com/BEDOLAGA-DEV/RemnaCore/plugins/botsdk"
)

// bot registers the user, opens their cabinet, and — to prove the guest can
// READ host data — echoes the first available plan's name back as a message.
func bot(u botsdk.Update) error {
	if err := botsdk.RegisterUser(u.From.ID, botsdk.DisplayName(u.From)); err != nil {
		return err
	}
	if err := botsdk.OpenCabinet(u.ChatID, ""); err != nil {
		return err
	}
	offers, err := botsdk.PlansList(botsdk.ChannelTelegram)
	if err != nil {
		return err
	}
	if len(offers) > 0 {
		return botsdk.SendText(u.ChatID, offers[0].Name)
	}
	return nil
}

// handle_update is the exported entry point called by the bot host dispatcher.
//
//go:wasmexport handle_update
func handle_update() int32 {
	return botsdk.Handle(bot)
}

func main() {}
