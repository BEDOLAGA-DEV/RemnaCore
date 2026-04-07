package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/aggregate"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/vo"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/sdk"
)

// Hook name constants for subscription lifecycle dispatch points.
// Pre-hooks (sync) are named "{entity}.{verb}ing" and fire before the aggregate
// transition. Post-hooks (async) are named "{entity}.{past_tense}.post" and
// fire after the transaction commits.
const (
	HookSubCancelling     = "subscription.cancelling"
	HookSubCancelledPost  = "subscription.cancelled.post"
	HookSubActivatedPost  = "subscription.activated.post"
	HookSubPausedPost     = "subscription.paused.post"
	HookSubResumedPost    = "subscription.resumed.post"
	HookSubRenewing       = "subscription.renewing"
	HookSubRenewedPost    = "subscription.renewed.post"
	HookSubUpgrading      = "subscription.upgrading"
	HookSubUpgradedPost   = "subscription.upgraded.post"
	HookSubDowngradedPost = "subscription.downgraded.post"
	HookSubExpiredPost    = "subscription.expired.post"
)

// PauseSubscription pauses an active subscription. The read (with FOR UPDATE
// lock), aggregate transition, update, and outbox events are all performed
// inside a single database transaction to prevent TOCTOU races.
func (s *BillingService) PauseSubscription(ctx context.Context, subID string) error {
	var asyncPayload *subAsyncPayload

	err := s.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		sub, err := s.subs.GetByIDForUpdate(txCtx, subID)
		if err != nil {
			return fmt.Errorf("get subscription: %w", err)
		}

		if err := sub.Pause(s.clock.Now()); err != nil {
			return fmt.Errorf("pause subscription: %w", err)
		}

		if err := s.subs.Update(txCtx, sub); err != nil {
			return fmt.Errorf("update subscription: %w", err)
		}

		asyncPayload = &subAsyncPayload{
			SubscriptionID: sub.ID,
			UserID:         sub.UserID,
			PlanID:         sub.PlanID,
		}

		return domainevent.PublishAll(txCtx, s.publisher, sub)
	})
	if err != nil {
		return err
	}

	if asyncPayload != nil {
		s.dispatchAsync(ctx, HookSubPausedPost, asyncPayload)
	}
	return nil
}

// ResumeSubscription resumes a paused subscription back to active. The read
// (with FOR UPDATE lock), aggregate transition, update, and outbox events are
// all performed inside a single database transaction.
func (s *BillingService) ResumeSubscription(ctx context.Context, subID string) error {
	var asyncPayload *subAsyncPayload

	err := s.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		sub, err := s.subs.GetByIDForUpdate(txCtx, subID)
		if err != nil {
			return fmt.Errorf("get subscription: %w", err)
		}

		if err := sub.Resume(s.clock.Now()); err != nil {
			return fmt.Errorf("resume subscription: %w", err)
		}

		if err := s.subs.Update(txCtx, sub); err != nil {
			return fmt.Errorf("update subscription: %w", err)
		}

		asyncPayload = &subAsyncPayload{
			SubscriptionID: sub.ID,
			UserID:         sub.UserID,
			PlanID:         sub.PlanID,
		}

		return domainevent.PublishAll(txCtx, s.publisher, sub)
	})
	if err != nil {
		return err
	}

	if asyncPayload != nil {
		s.dispatchAsync(ctx, HookSubResumedPost, asyncPayload)
	}
	return nil
}

// RenewSubscription advances the subscription to its next billing period and
// creates a renewal invoice. Before calling the aggregate Renew method, the
// subscription.renewing hook is dispatched to allow plugins to modify renewal
// parameters (price, interval, discount).
//
// If a PendingPlanID was set by a prior Downgrade, the aggregate applies it
// during renewal and the new invoice reflects the downgraded plan's pricing.
func (s *BillingService) RenewSubscription(ctx context.Context, subID string) error {
	var asyncPayload *subAsyncPayload

	err := s.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		sub, err := s.subs.GetByIDForUpdate(txCtx, subID)
		if err != nil {
			return fmt.Errorf("get subscription: %w", err)
		}

		// Dispatch subscription.renewing pre-hook — plugin can modify price/discount.
		hookResp, _ := s.dispatchHook(txCtx, HookSubRenewing, sdk.SubRenewingRequest{
			SubscriptionID: sub.ID,
			UserID:         sub.UserID,
			PlanID:         sub.PlanID,
			CurrentPeriod:  sub.Period.End.Format("2006-01-02"),
		})

		now := s.clock.Now()

		pendingPlanActive := true
		if sub.PendingPlanID != nil {
			pendingPlan, err := s.plans.GetByID(txCtx, *sub.PendingPlanID)
			if err != nil {
				return fmt.Errorf("load pending plan: %w", err)
			}
			pendingPlanActive = pendingPlan.IsActive
		}

		if err := sub.Renew(now, pendingPlanActive); err != nil {
			return fmt.Errorf("renew subscription: %w", err)
		}

		if err := s.subs.Update(txCtx, sub); err != nil {
			return fmt.Errorf("update subscription: %w", err)
		}

		// Load the (possibly downgraded) plan to create the renewal invoice.
		plan, err := s.plans.GetByID(txCtx, sub.PlanID)
		if err != nil {
			return fmt.Errorf("get plan for renewal invoice: %w", err)
		}

		lineItems := buildLineItems(plan, sub.AddonIDs, s.logger)
		inv, err := aggregate.NewInvoice(sub.ID, sub.UserID, lineItems, nil, plan.BasePrice.Currency, now)
		if err != nil {
			return fmt.Errorf("create renewal invoice: %w", err)
		}

		// Apply plugin price/discount overrides from the renewing hook.
		applyRenewalHookOverrides(inv, hookResp, now)

		if err := s.invoices.Create(txCtx, inv); err != nil {
			return fmt.Errorf("persist renewal invoice: %w", err)
		}

		if err := domainevent.PublishAll(txCtx, s.publisher, sub); err != nil {
			return err
		}
		if err := domainevent.PublishAll(txCtx, s.publisher, inv); err != nil {
			return err
		}

		asyncPayload = &subAsyncPayload{
			SubscriptionID: sub.ID,
			UserID:         sub.UserID,
			PlanID:         sub.PlanID,
		}

		return nil
	})
	if err != nil {
		return err
	}

	if asyncPayload != nil {
		s.dispatchAsync(ctx, HookSubRenewedPost, asyncPayload)
	}
	return nil
}

// UpgradeSubscription immediately changes the subscription to a new plan with
// prorated pricing. The subscription.upgrading hook is dispatched before the
// transition to allow plugins to modify proration credit or apply a discount.
func (s *BillingService) UpgradeSubscription(ctx context.Context, subID, newPlanID string) error {
	var asyncPayload *subUpgradeAsyncPayload

	err := s.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		sub, err := s.subs.GetByIDForUpdate(txCtx, subID)
		if err != nil {
			return fmt.Errorf("get subscription: %w", err)
		}

		currentPlan, err := s.plans.GetByID(txCtx, sub.PlanID)
		if err != nil {
			return fmt.Errorf("get current plan: %w", err)
		}

		newPlan, err := s.plans.GetByID(txCtx, newPlanID)
		if err != nil {
			return fmt.Errorf("get new plan: %w", err)
		}

		now := s.clock.Now()
		credit := s.prorate.CalculateUpgradeCredit(currentPlan, sub.Period, now)

		// Dispatch subscription.upgrading pre-hook — plugin can modify credit/discount.
		hookResp, _ := s.dispatchHook(txCtx, HookSubUpgrading, sdk.SubUpgradingRequest{
			SubscriptionID:  sub.ID,
			UserID:          sub.UserID,
			FromPlanID:      sub.PlanID,
			ToPlanID:        newPlanID,
			ProrationCredit: credit.Amount,
		})
		if hookResp != nil {
			var resp sdk.SubUpgradingResponse
			if err := json.Unmarshal(hookResp, &resp); err == nil && resp.CreditOverride != nil {
				credit = vo.NewMoney(*resp.CreditOverride, credit.Currency)
			}
		}

		oldPlanID := sub.PlanID
		newPeriod := vo.NewBillingPeriod(now, newPlan.Interval)
		if err := sub.Upgrade(newPlanID, newPeriod, now); err != nil {
			return fmt.Errorf("upgrade subscription: %w", err)
		}

		if err := s.subs.Update(txCtx, sub); err != nil {
			return fmt.Errorf("update subscription: %w", err)
		}

		// Create a prorated invoice for the upgrade cost.
		upgradeCost, err := s.prorate.CalculateUpgradeCost(newPlan, sub.Period, now, credit)
		if err != nil {
			return fmt.Errorf("calculate upgrade cost: %w", err)
		}

		if upgradeCost.IsPositive() {
			lineItems := []vo.LineItem{
				vo.NewLineItem(newPlan.Name, vo.LineItemPlan, upgradeCost, lineItemQuantityOne),
			}
			inv, err := aggregate.NewInvoice(sub.ID, sub.UserID, lineItems, nil, upgradeCost.Currency, now)
			if err != nil {
				return fmt.Errorf("create upgrade invoice: %w", err)
			}
			if err := s.invoices.Create(txCtx, inv); err != nil {
				return fmt.Errorf("persist upgrade invoice: %w", err)
			}
			if err := domainevent.PublishAll(txCtx, s.publisher, inv); err != nil {
				return err
			}
		}

		if err := domainevent.PublishAll(txCtx, s.publisher, sub); err != nil {
			return err
		}

		asyncPayload = &subUpgradeAsyncPayload{
			SubscriptionID: sub.ID,
			UserID:         sub.UserID,
			FromPlanID:     oldPlanID,
			ToPlanID:       newPlanID,
		}

		return nil
	})
	if err != nil {
		return err
	}

	if asyncPayload != nil {
		s.dispatchAsync(ctx, HookSubUpgradedPost, asyncPayload)
	}
	return nil
}

// DowngradeSubscription schedules a plan change for the next billing period.
// The actual switch happens during the next Renew. No pre-hook is dispatched
// because the downgrade is deferred (not immediately applied).
func (s *BillingService) DowngradeSubscription(ctx context.Context, subID, newPlanID string) error {
	var asyncPayload *subUpgradeAsyncPayload

	err := s.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		sub, err := s.subs.GetByIDForUpdate(txCtx, subID)
		if err != nil {
			return fmt.Errorf("get subscription: %w", err)
		}

		oldPlanID := sub.PlanID
		if err := sub.Downgrade(newPlanID, s.clock.Now()); err != nil {
			return fmt.Errorf("downgrade subscription: %w", err)
		}

		if err := s.subs.Update(txCtx, sub); err != nil {
			return fmt.Errorf("update subscription: %w", err)
		}

		asyncPayload = &subUpgradeAsyncPayload{
			SubscriptionID: sub.ID,
			UserID:         sub.UserID,
			FromPlanID:     oldPlanID,
			ToPlanID:       newPlanID,
		}

		return domainevent.PublishAll(txCtx, s.publisher, sub)
	})
	if err != nil {
		return err
	}

	if asyncPayload != nil {
		s.dispatchAsync(ctx, HookSubDowngradedPost, asyncPayload)
	}
	return nil
}

// ExpireSubscription moves a subscription to the expired terminal state. This
// is a system-initiated action (e.g., triggered by a scheduler when the billing
// period elapses without renewal). No pre-hook is dispatched.
func (s *BillingService) ExpireSubscription(ctx context.Context, subID string) error {
	var asyncPayload *subAsyncPayload

	err := s.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		sub, err := s.subs.GetByIDForUpdate(txCtx, subID)
		if err != nil {
			return fmt.Errorf("get subscription: %w", err)
		}

		if err := sub.Expire(s.clock.Now()); err != nil {
			return fmt.Errorf("expire subscription: %w", err)
		}

		if err := s.subs.Update(txCtx, sub); err != nil {
			return fmt.Errorf("update subscription: %w", err)
		}

		asyncPayload = &subAsyncPayload{
			SubscriptionID: sub.ID,
			UserID:         sub.UserID,
			PlanID:         sub.PlanID,
		}

		return domainevent.PublishAll(txCtx, s.publisher, sub)
	})
	if err != nil {
		return err
	}

	if asyncPayload != nil {
		s.dispatchAsync(ctx, HookSubExpiredPost, asyncPayload)
	}
	return nil
}

// subAsyncPayload is the internal payload for simple post-commit async hooks
// that only need subscription identification fields.
type subAsyncPayload struct {
	SubscriptionID string `json:"subscription_id"`
	UserID         string `json:"user_id"`
	PlanID         string `json:"plan_id"`
}

// subUpgradeAsyncPayload is the internal payload for upgrade/downgrade
// post-commit async hooks that include both old and new plan IDs.
type subUpgradeAsyncPayload struct {
	SubscriptionID string `json:"subscription_id"`
	UserID         string `json:"user_id"`
	FromPlanID     string `json:"from_plan_id"`
	ToPlanID       string `json:"to_plan_id"`
}

// applyRenewalHookOverrides applies price and discount overrides from the
// subscription.renewing hook response to a renewal invoice. The invoice must be
// in draft status. Hook parse failures are silently ignored (fallback to
// original pricing).
func applyRenewalHookOverrides(inv *aggregate.Invoice, hookResp json.RawMessage, now time.Time) {
	if hookResp == nil {
		return
	}
	var resp sdk.SubRenewingResponse
	if err := json.Unmarshal(hookResp, &resp); err != nil {
		return
	}

	var subtotal *int64
	if resp.PriceOverride != nil {
		subtotal = resp.PriceOverride
	}

	var discount *int64
	if resp.DiscountPercent != nil {
		pct := int64(*resp.DiscountPercent)
		// Floor rounding: see vo.RoundFloor and Money.PercentOf documentation.
		discountAmount := inv.Subtotal.PercentOf(pct, vo.RoundFloor).Amount
		discount = &discountAmount
	}

	if subtotal != nil || discount != nil {
		reason := "renewal hook override"
		if err := inv.ApplyPricingModification(subtotal, discount, reason, now); err != nil {
			slog.Warn("failed to apply pricing modification",
				slog.String("invoice_id", inv.ID),
				slog.Any("error", err),
			)
		}
	}
}
