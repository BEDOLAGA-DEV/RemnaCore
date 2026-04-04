# RemnaCore Event Catalog

All domain events emitted across bounded contexts. Each event is published as
JSON via NATS JetStream through the transactional outbox. Payloads use typed
structs (see `event_payloads.go` in each domain package).

---

## Identity Events

Stream: `IDENTITY` | Subjects: `user.>`

### `user.registered`

Emitted when a new platform user registers.

**Payload:** `identity.UserRegisteredPayload`

```json
{
  "type": "user.registered",
  "timestamp": "2026-04-04T12:00:00Z",
  "data": {
    "user_id": "uuid",
    "email": "user@example.com"
  }
}
```

### `user.email_verified`

Emitted when a user verifies their email address.

**Payload:** `identity.EmailVerifiedPayload`

```json
{
  "type": "user.email_verified",
  "timestamp": "2026-04-04T12:00:00Z",
  "data": {
    "user_id": "uuid",
    "email": "user@example.com"
  }
}
```

### `user.logged_in`

Emitted on successful authentication.

**Payload:** `identity.UserLoggedInPayload`

```json
{
  "type": "user.logged_in",
  "timestamp": "2026-04-04T12:00:00Z",
  "data": {
    "user_id": "uuid"
  }
}
```

### `user.profile_updated`

Emitted when a user updates their profile. No factory function yet (reserved).

### `user.password_reset_requested`

Emitted when a user requests a password reset. Notification plugins listen for
this event to send the reset email.

**Payload:** `identity.PasswordResetRequestedPayload`

```json
{
  "type": "user.password_reset_requested",
  "timestamp": "2026-04-04T12:00:00Z",
  "data": {
    "user_id": "uuid",
    "email": "user@example.com",
    "token": "hex-encoded-token"
  }
}
```

### `user.password_reset`

Emitted when a password has been successfully reset.

**Payload:** `identity.PasswordResetPayload`

```json
{
  "type": "user.password_reset",
  "timestamp": "2026-04-04T12:00:00Z",
  "data": {
    "user_id": "uuid"
  }
}
```

---

## Billing Events

Stream: `BILLING` | Subjects: `invoice.>`, `subscription.>`, `family.>`

### `invoice.created`

Emitted when a new invoice is generated for a subscription.

**Payload:** `billing.InvoiceCreatedPayload`

```json
{
  "type": "invoice.created",
  "timestamp": "2026-04-04T12:00:00Z",
  "data": {
    "invoice_id": "uuid",
    "subscription_id": "uuid",
    "user_id": "uuid",
    "amount_cents": 1999
  }
}
```

### `invoice.paid`

Emitted when an invoice is successfully paid. Triggers subscription activation
if the subscription is in trial or past_due status.

**Payload:** `billing.InvoicePaidPayload`

```json
{
  "type": "invoice.paid",
  "timestamp": "2026-04-04T12:00:00Z",
  "data": {
    "invoice_id": "uuid",
    "subscription_id": "uuid",
    "user_id": "uuid",
    "amount_cents": 1999
  }
}
```

### `invoice.failed`

Emitted when an invoice payment fails.

**Payload:** `billing.InvoiceFailedPayload`

```json
{
  "type": "invoice.failed",
  "timestamp": "2026-04-04T12:00:00Z",
  "data": {
    "invoice_id": "uuid",
    "subscription_id": "uuid",
    "user_id": "uuid",
    "reason": "card_declined"
  }
}
```

### `invoice.refunded`

Emitted when an invoice is refunded.

**Payload:** `billing.InvoiceRefundedPayload`

```json
{
  "type": "invoice.refunded",
  "timestamp": "2026-04-04T12:00:00Z",
  "data": {
    "invoice_id": "uuid",
    "subscription_id": "uuid",
    "user_id": "uuid",
    "amount_cents": 1999
  }
}
```

### `subscription.created`

Emitted when a new subscription is created (starts in trial by default).

**Payload:** `billing.SubCreatedPayload`

```json
{
  "type": "subscription.created",
  "timestamp": "2026-04-04T12:00:00Z",
  "data": {
    "subscription_id": "uuid",
    "user_id": "uuid",
    "plan_id": "uuid"
  }
}
```

### `subscription.activated`

Emitted when a subscription transitions to active. The BillingEventConsumer
enriches this event with plan/family data and routes it to the
MultiSubOrchestrator for Remnawave provisioning.

**Payload:** `billing.SubActivatedPayload`

```json
{
  "type": "subscription.activated",
  "timestamp": "2026-04-04T12:00:00Z",
  "data": {
    "subscription_id": "uuid",
    "user_id": "uuid"
  },
  "entity_id": "uuid"
}
```

### `subscription.cancelled`

Emitted when a subscription is cancelled. Triggers Remnawave deprovisioning.

**Payload:** `billing.SubCancelledPayload`

```json
{
  "type": "subscription.cancelled",
  "timestamp": "2026-04-04T12:00:00Z",
  "data": {
    "subscription_id": "uuid",
    "user_id": "uuid",
    "reason": "user_requested"
  }
}
```

### `subscription.renewed`

Emitted when a subscription is renewed for a new billing period.

**Payload:** `billing.SubRenewedPayload`

```json
{
  "type": "subscription.renewed",
  "timestamp": "2026-04-04T12:00:00Z",
  "data": {
    "subscription_id": "uuid",
    "user_id": "uuid"
  }
}
```

### `subscription.upgraded`

Emitted when a subscription is upgraded to a higher plan.

**Payload:** `billing.SubUpgradedPayload`

```json
{
  "type": "subscription.upgraded",
  "timestamp": "2026-04-04T12:00:00Z",
  "data": {
    "subscription_id": "uuid",
    "user_id": "uuid",
    "from_plan_id": "uuid",
    "to_plan_id": "uuid"
  }
}
```

### `subscription.downgraded`

Emitted when a subscription is downgraded to a lower plan.

**Payload:** `billing.SubDowngradedPayload`

```json
{
  "type": "subscription.downgraded",
  "timestamp": "2026-04-04T12:00:00Z",
  "data": {
    "subscription_id": "uuid",
    "user_id": "uuid",
    "from_plan_id": "uuid",
    "to_plan_id": "uuid"
  }
}
```

### `subscription.trial_started`

Emitted when a trial period begins.

**Payload:** `billing.SubTrialStartedPayload`

```json
{
  "type": "subscription.trial_started",
  "timestamp": "2026-04-04T12:00:00Z",
  "data": {
    "subscription_id": "uuid",
    "user_id": "uuid",
    "plan_id": "uuid"
  }
}
```

### `subscription.trial_ending`

Emitted when a trial is about to expire (days_remaining indicates urgency).

**Payload:** `billing.SubTrialEndingPayload`

```json
{
  "type": "subscription.trial_ending",
  "timestamp": "2026-04-04T12:00:00Z",
  "data": {
    "subscription_id": "uuid",
    "user_id": "uuid",
    "days_remaining": 3
  }
}
```

### `subscription.paused`

Emitted when a subscription is paused. Triggers Remnawave deprovisioning.

**Payload:** `billing.SubPausedPayload`

```json
{
  "type": "subscription.paused",
  "timestamp": "2026-04-04T12:00:00Z",
  "data": {
    "subscription_id": "uuid",
    "user_id": "uuid"
  }
}
```

### `subscription.resumed`

Emitted when a paused subscription is resumed. Triggers re-provisioning.

**Payload:** `billing.SubResumedPayload`

```json
{
  "type": "subscription.resumed",
  "timestamp": "2026-04-04T12:00:00Z",
  "data": {
    "subscription_id": "uuid",
    "user_id": "uuid"
  }
}
```

### `family.member_added`

Emitted when a family member is added to a subscription's family group.

**Payload:** `billing.FamilyMemberAddedPayload`

```json
{
  "type": "family.member_added",
  "timestamp": "2026-04-04T12:00:00Z",
  "data": {
    "family_group_id": "uuid",
    "owner_id": "uuid",
    "member_id": "uuid"
  }
}
```

### `family.member_removed`

Emitted when a family member is removed from a subscription's family group.

**Payload:** `billing.FamilyMemberRemovedPayload`

```json
{
  "type": "family.member_removed",
  "timestamp": "2026-04-04T12:00:00Z",
  "data": {
    "family_group_id": "uuid",
    "owner_id": "uuid",
    "member_id": "uuid"
  }
}
```

---

## MultiSub Events

Stream: `REMNAWAVE` | Subjects: `binding.>`

### `binding.provisioned`

Emitted when a Remnawave binding is created and the corresponding Remnawave
user is successfully provisioned.

**Payload:** `multisub.BindingProvisionedPayload`

```json
{
  "type": "binding.provisioned",
  "timestamp": "2026-04-04T12:00:00Z",
  "data": {
    "binding_id": "uuid",
    "subscription_id": "uuid",
    "remnawave_uuid": "uuid",
    "purpose": "base"
  }
}
```

### `binding.deprovisioned`

Emitted when a Remnawave binding is removed and the user is deleted from
the Remnawave panel.

**Payload:** `multisub.BindingDeprovisionedPayload`

```json
{
  "type": "binding.deprovisioned",
  "timestamp": "2026-04-04T12:00:00Z",
  "data": {
    "binding_id": "uuid",
    "subscription_id": "uuid",
    "remnawave_uuid": "uuid"
  }
}
```

### `binding.sync_failed`

Emitted when binding synchronisation with Remnawave fails.

**Payload:** `multisub.BindingSyncFailedPayload`

```json
{
  "type": "binding.sync_failed",
  "timestamp": "2026-04-04T12:00:00Z",
  "data": {
    "binding_id": "uuid",
    "subscription_id": "uuid",
    "reason": "remnawave API timeout"
  }
}
```

### `binding.sync_completed`

Emitted when a binding is successfully synchronised with Remnawave.

**Payload:** `multisub.BindingSyncCompletedPayload`

```json
{
  "type": "binding.sync_completed",
  "timestamp": "2026-04-04T12:00:00Z",
  "data": {
    "binding_id": "uuid",
    "subscription_id": "uuid"
  }
}
```

### `binding.traffic_exceeded`

Emitted when a binding exceeds its traffic limit. The binding is disabled.

**Payload:** `multisub.BindingTrafficExceededPayload`

```json
{
  "type": "binding.traffic_exceeded",
  "timestamp": "2026-04-04T12:00:00Z",
  "data": {
    "binding_id": "uuid",
    "subscription_id": "uuid",
    "remnawave_uuid": "uuid"
  }
}
```

---

## Payment Events

Stream: `PAYMENT` | Subjects: `payment.>`

### `payment.charge_created`

Emitted when a payment charge is created via a payment plugin (Stripe, BTCPay, etc.).

**Payload:** `payment.ChargeCreatedPayload`

```json
{
  "type": "payment.charge_created",
  "timestamp": "2026-04-04T12:00:00Z",
  "data": {
    "payment_id": "uuid",
    "invoice_id": "uuid",
    "provider": "stripe",
    "external_id": "pi_xxxxx",
    "amount": 1999
  }
}
```

### `payment.charge_completed`

Emitted when a payment is confirmed by the provider webhook.

**Payload:** `payment.ChargeCompletedPayload`

```json
{
  "type": "payment.charge_completed",
  "timestamp": "2026-04-04T12:00:00Z",
  "data": {
    "payment_id": "uuid",
    "invoice_id": "uuid",
    "provider": "stripe",
    "amount": 1999
  }
}
```

### `payment.charge_failed`

Emitted when a payment charge fails.

**Payload:** `payment.ChargeFailedPayload`

```json
{
  "type": "payment.charge_failed",
  "timestamp": "2026-04-04T12:00:00Z",
  "data": {
    "payment_id": "uuid",
    "invoice_id": "uuid",
    "provider": "stripe",
    "reason": "card_declined"
  }
}
```

### `payment.refund_completed`

Emitted when a refund is processed.

**Payload:** `payment.RefundCompletedPayload`

```json
{
  "type": "payment.refund_completed",
  "timestamp": "2026-04-04T12:00:00Z",
  "data": {
    "payment_id": "uuid",
    "invoice_id": "uuid",
    "provider": "stripe",
    "amount": 1999
  }
}
```

### `payment.webhook_received`

Emitted when a payment provider webhook is received and verified.

**Payload:** `payment.WebhookReceivedPayload`

```json
{
  "type": "payment.webhook_received",
  "timestamp": "2026-04-04T12:00:00Z",
  "data": {
    "provider": "stripe",
    "external_id": "pi_xxxxx",
    "status": "succeeded"
  }
}
```

---

## Reseller Events

Stream: `RESELLER` | Subjects: `reseller.>`

### `reseller.tenant_created`

Emitted when a new white-label tenant is created.

**Payload:** `reseller.TenantCreatedPayload`

```json
{
  "type": "reseller.tenant_created",
  "timestamp": "2026-04-04T12:00:00Z",
  "data": {
    "tenant_id": "uuid",
    "owner_user_id": "uuid"
  }
}
```

### `reseller.tenant_updated`

Reserved for tenant configuration changes. No factory function yet.

### `reseller.account_created`

Emitted when a new reseller account is created under a tenant.

**Payload:** `reseller.ResellerCreatedPayload`

```json
{
  "type": "reseller.account_created",
  "timestamp": "2026-04-04T12:00:00Z",
  "data": {
    "reseller_id": "uuid",
    "tenant_id": "uuid",
    "user_id": "uuid"
  }
}
```

### `reseller.commission_created`

Emitted when a commission is recorded for a reseller sale.

**Payload:** `reseller.CommissionCreatedPayload`

```json
{
  "type": "reseller.commission_created",
  "timestamp": "2026-04-04T12:00:00Z",
  "data": {
    "commission_id": "uuid",
    "reseller_id": "uuid",
    "amount": 500
  }
}
```

### `reseller.commission_paid`

Reserved for commission payouts. No factory function yet.

---

## Plugin Events

Stream: `PLUGINS` | Subjects: `plugin.>`

### `plugin.installed`

Emitted when a WASM plugin is installed.

```json
{
  "type": "plugin.installed",
  "timestamp": "2026-04-04T12:00:00Z",
  "data": {
    "plugin_id": "uuid",
    "slug": "stripe-payment",
    "version": "1.0.0"
  }
}
```

### `plugin.enabled`

Emitted when a plugin is enabled and loaded into the runtime pool.

```json
{
  "type": "plugin.enabled",
  "timestamp": "2026-04-04T12:00:00Z",
  "data": {
    "plugin_id": "uuid",
    "slug": "stripe-payment"
  }
}
```

### `plugin.disabled`

Emitted when a plugin is disabled and unloaded from the runtime pool.

```json
{
  "type": "plugin.disabled",
  "timestamp": "2026-04-04T12:00:00Z",
  "data": {
    "plugin_id": "uuid",
    "slug": "stripe-payment"
  }
}
```

### `plugin.uninstalled`

Emitted when a plugin is completely removed.

```json
{
  "type": "plugin.uninstalled",
  "timestamp": "2026-04-04T12:00:00Z",
  "data": {
    "plugin_id": "uuid",
    "slug": "stripe-payment"
  }
}
```

### `plugin.hot_reloaded`

Emitted when a plugin is atomically replaced with a new version.

```json
{
  "type": "plugin.hot_reloaded",
  "timestamp": "2026-04-04T12:00:00Z",
  "data": {
    "plugin_id": "uuid",
    "slug": "stripe-payment",
    "old_version": "1.0.0",
    "new_version": "2.0.0"
  }
}
```

### `plugin.error`

Emitted when a plugin enters the error state.

```json
{
  "type": "plugin.error",
  "timestamp": "2026-04-04T12:00:00Z",
  "data": {
    "plugin_id": "uuid",
    "slug": "stripe-payment",
    "reason": "out of memory"
  }
}
```

### `plugin.hook.executed`

Emitted after a successful hook invocation.

```json
{
  "type": "plugin.hook.executed",
  "timestamp": "2026-04-04T12:00:00Z",
  "data": {
    "plugin_id": "uuid",
    "slug": "stripe-payment",
    "hook_name": "payment.create_charge",
    "duration_ms": 42
  }
}
```

### `plugin.hook.failed`

Emitted when a hook invocation fails.

```json
{
  "type": "plugin.hook.failed",
  "timestamp": "2026-04-04T12:00:00Z",
  "data": {
    "plugin_id": "uuid",
    "slug": "stripe-payment",
    "hook_name": "payment.create_charge",
    "reason": "wasm execution timeout"
  }
}
```

---

## Infrastructure Events

Stream: `INFRA` | Subjects: `infra.>`, `node.>`

### `node.health.changed`

Emitted when a VPN node transitions between online and offline states.

```json
{
  "type": "node.health.changed",
  "timestamp": "2026-04-04T12:00:00Z",
  "data": {
    "node_id": "uuid",
    "name": "de-fra-01",
    "status": "online",
    "country": "DE"
  }
}
```

---

## Event Envelope

Every domain event includes an optional `entity_id` field that identifies the
source aggregate instance (e.g. subscription ID, invoice ID). Producers set this
via the `domainevent.NewWithEntity` / `NewAtWithEntity` constructors. Events
created with the plain `New` / `NewAt` constructors omit `entity_id` from JSON.

```json
{
  "type": "subscription.activated",
  "timestamp": "2026-04-04T12:00:00Z",
  "data": { "subscription_id": "uuid", "user_id": "uuid" },
  "entity_id": "uuid"
}
```

---

## Ordering Guarantees

Events for the same entity (identified by `entity_id`) are processed serially
by the `BillingEventConsumer` using per-entity locking. This prevents race
conditions where `subscription.activated` and `subscription.cancelled` for the
same subscription could be processed out of order when arriving on different
NATS subjects concurrently.

Events for **different** entities are processed concurrently for maximum
throughput.

### Idempotency

Each event is deduplicated by a business-level key: `{event_type}:{entity_id}`,
stored in `multisub.idempotency_keys` with a 24-hour TTL. This catches outbox
relay re-publishes where the same business event gets a new Watermill transport
UUID. Redelivered events are silently skipped.

For backward compatibility, if `entity_id` is empty (pre-migration events), the
consumer falls back to extracting `subscription_id` from the event data payload.

---

## Dead-Letter Queue

Stream: `DLQ` | Subjects: `dlq.>` | Retention: 30 days

Messages that fail processing after 3 retries are routed to `dlq.<original_subject>`.

```json
{
  "original_subject": "subscription.activated",
  "original_payload": "{...original event JSON...}",
  "error": "lookup subscription sub-123: not found",
  "msg_id": "watermill-uuid",
  "failed_at": "2026-04-04T12:00:00Z",
  "retry_count": 3
}
```
