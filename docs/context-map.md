# RemnaCore Context Map

## Bounded Contexts

| Context | Type | Responsibility |
|---------|------|---------------|
| **Identity** | Generic | User registration, authentication, profiles |
| **Billing** | Core | Plans, subscriptions, invoices, family groups |
| **Multi-Sub** | Core | Platform User → N Remnawave Users orchestration |
| **Payment** | Supporting | Payment processing (plugin-driven facade) |
| **Reseller** | Supporting | White-label tenants, commissions |

## Relationships

```
┌──────────┐   Published Language    ┌──────────────┐
│ Identity │ ──── (UserDTO) ───────► │   Billing    │
│          │◄── Conformist ──────── │              │
└────┬─────┘                        └──────┬───────┘
     │                                     │
     │ Domain Events                       │ Domain Events
     │ (user.registered,                   │ (subscription.activated,
     │  user.email_verified)               │  invoice.paid)
     │                                     │
     │                                     ▼
     │                              ┌──────────────┐
     │                              │  Multi-Sub   │
     │                              │ Orchestrator │
     │                              └──────┬───────┘
     │                                     │
     │                                     │ ACL (RemnawaveGateway port)
     │                                     │
     │                                     ▼
     │                    ┌───────────────────────────┐
     │                    │  REMNAWAVE (External)      │
     │                    │  Conformist with ACL       │
     │                    └───────────────────────────┘
     │
     │                    ┌──────────────┐
     └──────────────────► │   Reseller   │
       Domain Events      └──────────────┘
       (user.registered)
```

## Integration Patterns

| Upstream | Downstream | Pattern | Mechanism | ACL Type |
|----------|-----------|---------|-----------|----------|
| Identity → Billing | Domain Events | NATS JetStream | `user.registered`, `user.email_verified` | -- |
| Identity → Reseller | Domain Events | NATS JetStream | `user.registered` | -- |
| Billing → Multi-Sub | Domain Events | NATS JetStream + Outbox | `subscription.activated`, `.cancelled`, `.paused`, `.resumed` | `PlanSnapshot` (ACL in multisub domain) |
| Billing → Payment | ACL Port | In-process interface | `billing.PaymentGateway` | `CreateChargeRequest/Result` (ACL in billing domain) |
| Multi-Sub → Remnawave | ACL Port | HTTP API | `multisub.RemnawaveGateway` | `CreateRemnawaveUserRequest/Result` |
| Payment → Plugins | Hook Dispatch | WASM + NATS | `hookdispatch.Dispatcher` | -- |
| Remnawave → Platform | Webhooks | HTTP + HMAC | `remnawave.WebhookHandler` → Domain Events | Event translation in ACL adapter |

## Shared Kernel (pkg/)

| Package | Purpose | Used By |
|---------|---------|--------|
| `pkg/domainevent` | Event type + Publisher interface | All contexts |
| `pkg/clock` | Clock interface for deterministic time | All services |
| `pkg/txmanager` | Transaction runner interface | Billing service |
| `pkg/tracing` | OpenTelemetry span helper | All services |
| `pkg/hookdispatch` | Hook dispatcher interface | Billing (checkout), Infra (router) |
| `pkg/naming` | Username generation + PlatformTag | Multi-Sub, Remnawave adapter |

## Anti-Corruption Layers

| Context | ACL Type | Location | Translates |
|---------|----------|----------|-----------|
| Multi-Sub | `PlanSnapshot` | `multisub/plan_snapshot.go` | `billing.Plan` → `multisub.PlanSnapshot` |
| Multi-Sub | `RemnawaveGateway` | `multisub/gateway.go` | Domain port → `remnawave.GatewayAdapter` |
| Multi-Sub | `PlanProvider` | `multisub/ports.go` | Domain port → `billing_lookup.go` adapter |
| Multi-Sub | `SubscriptionProvider` | `multisub/ports.go` | Domain port → `billing_lookup.go` adapter |
| Billing | `PaymentGateway` | `billing/payment_gateway.go` | Domain port → `payment.PaymentFacade` |

## Enforcement

Architecture tests in `tests/archtest/boundaries_test.go` enforce:

- Domain packages never import adapter/gateway/plugin/infra
- Bounded contexts never import each other
- Plugin package never imports adapter/gateway
