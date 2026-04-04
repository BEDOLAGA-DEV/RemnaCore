# RemnaCore

Modular VPN subscription sales platform built on Go, wrapping [Remnawave](https://github.com/remnawave/panel) as the VPN backend.

## Architecture

- **DDD Modular Monolith** — 9 bounded contexts in a single Go binary
- **Hexagonal Architecture** — domain logic isolated from adapters via interfaces
- **WASM Plugin System** — extend payments, notifications, pricing, anti-fraud via Extism/wazero plugins
- **Multi-Subscription** — one platform user maps to N Remnawave VPN users, each with own subscription link
- **Event-Driven** — 45 domain events flowing through NATS JetStream via Watermill

## Stack

| Layer | Technology |
|---|---|
| Language | Go 1.26.1 |
| HTTP | chi v5 |
| DI | Uber Fx |
| Database | PostgreSQL 18 (sqlc + Atlas migrations) |
| Cache | Valkey 9 (Redis-compatible) |
| Events | NATS JetStream 2.12 + Watermill |
| Plugins | Extism/wazero (WASM) |
| Frontend | React 19, TypeScript, Vite, TanStack Query/Router, Zustand, Tailwind CSS |
| Forms | React Hook Form + Zod |
| i18n | i18next (en + ru) |
| HTTP Client | ky |
| Telegram | go-telegram/bot |
| Observability | slog + zerolog, Prometheus |
| CI | Dagger (Go SDK) |
| Deploy | Docker Compose, Helm, Pulumi (Go) |

## Bounded Contexts

| Context | Type | Description |
|---|---|---|
| **Identity** | Generic | Registration (email + OAuth), JWT auth (ECDSA ES256), password reset |
| **Billing** | Core DDD | Plans, subscriptions (state machine), invoices, family groups, proration |
| **Multi-Sub Orchestrator** | Core DDD | Platform User → N Remnawave Users, provisioning/deprovisioning sagas |
| **Payment** | Plugin Facade | Dispatches to WASM plugins — zero built-in provider logic |
| **Reseller** | Supporting | White-label tenants, commissions, API key auth |
| **Plugin Runtime** | Infrastructure | WASM lifecycle, hook dispatcher, host functions, permissions |
| **Infrastructure** | Infrastructure | Node health monitor, smart router, speed test, subscription proxy |
| **Gateway** | Infrastructure | HTTP handlers, middleware (auth, rate-limit, CORS, tenant) |
| **Telegram** | Supporting | Bot with 7 commands + inline keyboards |

## Quick Start

```bash
# Clone
git clone https://github.com/BEDOLAGA-DEV/RemnaCore.git
cd RemnaCore

# Start all services
cp .env.example .env
# Edit .env with your Remnawave API token and secrets
docker compose up -d

# Apply migrations
make migrate
# Or manually:
# for f in internal/adapter/postgres/migrations/*.sql; do
#     docker compose exec -T platform-db psql -U platform -d remnacore < "$f"
# done

# Restart to pick up schemas
docker compose restart remnacore

# Test
curl http://localhost:4000/healthz
```

## Services

| Port | Service | Note |
|---|---|---|
| 4000 | RemnaCore API | Main API, exposed via Docker |
| 4100 | Subscription Proxy | VPN client configs, internal to binary |
| 4203 | Speed Test Server | Download/upload/ping, internal to binary |
| 3000 | Remnawave Panel | Needs reverse proxy (Caddy) for browser access |

Ports 4100 and 4203 run inside the RemnaCore binary. Expose them in `docker-compose.override.yml` if needed:
```yaml
services:
  remnacore:
    ports:
      - "4100:4100"
      - "4203:4203"
```

## Frontend

```bash
cd web
pnpm install
pnpm -r build
```

- **Cabinet** (user SPA) — plan selection, checkout, subscriptions, traffic, family management
- **Admin** (admin SPA) — users, subscriptions, invoices, plugins, tenants, nodes

Serve via Caddy or any static file server. Cabinet on port 80, admin on a separate port (e.g. 8081).

## Admin Panel

- **Users** — list, search, detail view, role management
- **Subscriptions** — all subscriptions, status, cancel
- **Invoices** — list, payment status
- **Plugins** — install/enable/disable WASM plugins, configure
- **Tenants** — reseller management, branding, commissions
- **Nodes** — Remnawave node health status

Admin access requires `role: admin` on the user account:

```sql
UPDATE identity.platform_users SET role='admin' WHERE email='your@email.com';
```

## Plugins

Three official WASM plugins included:

| Plugin | Hooks |
|---|---|
| `stripe-payment` | `payment.create_charge`, `payment.verify_webhook`, `payment.refund` |
| `email-notification` | `notification.send` + async events |
| `telegram-notification` | `notification.send` + async events |

```bash
# Build a plugin
cd plugins/stripe-payment
GOWORK=off GOOS=wasip1 GOARCH=wasm go build -o plugin.wasm .
```

Write your own:

```bash
# Scaffold
go run ./cmd/vpnctl plugin init --lang go --name my-plugin --hooks pricing.calculate

# Build
cd my-plugin && make build

# Install
go run ./cmd/vpnctl plugin install ./plugin.wasm
```

## Development

```bash
make build          # Compile binary
make test           # Run tests
make lint           # golangci-lint
make gen            # Regenerate sqlc
make migrate        # Apply migrations (Atlas)
make up             # Docker compose up
make down           # Docker compose down
```

## Deploy

**Docker Compose** — development and single-server production

**Helm** — Kubernetes with HPA, PDB, ingress, ServiceMonitor

**Pulumi** — Infrastructure as Code (Go)

## License

AGPL-3.0
