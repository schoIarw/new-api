# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

This is an AI API gateway/proxy built with Go. It aggregates 40+ upstream AI providers (OpenAI, Claude, Gemini, Azure, AWS Bedrock, etc.) behind a unified API, with user management, billing, rate limiting, and an admin dashboard. The project is named **new-api** and the organization is **QuantumNous**.

## Imported Conventions

See `AGENTS.md` for detailed rules on:
- Code quality style (early returns, minimize nesting, avoid single-use helpers)
- Backend JSON package usage (must use `common/json.go` wrappers)
- Database compatibility (SQLite, MySQL, PostgreSQL all supported)
- Billing safety invariants (saturation, clamping, overflow prevention)
- Backend test quality standards
- Frontend i18n and UI rules
- Project governance (protected branding, PR authorship disclosure)

## Tech Stack

- **Backend**: Go 1.25+, Gin web framework, GORM v2 ORM
- **Frontend**: React 19, TypeScript, Rsbuild, Base UI, Tailwind CSS (`web/default/`); React 18, Vite, Semi Design (`web/classic/`)
- **Databases**: SQLite, MySQL >= 5.7.8, PostgreSQL >= 9.6
- **Cache**: Redis (go-redis) + in-memory cache
- **Auth**: JWT, WebAuthn/Passkeys, OAuth (GitHub, Discord, OIDC, custom)
- **Frontend package manager**: Bun (preferred over npm/yarn/pnpm)

## Architecture

### Layered Structure (Backend)

```
router/        — HTTP routing (API, relay, dashboard, web)
controller/    — Request handlers (thin, delegates to service/model)
service/       — Business logic (billing, quotas, channel selection, tasks)
model/         — Data models and DB access (GORM), all cross-DB compatible
relay/         — AI API relay/proxy with provider adapters
  relay/channel/ — 40+ provider-specific adapters (openai/, claude/, gemini/, aws/, etc.)
  relay/common/  — Shared relay types and helpers
  relay/helper/  — Request validation, token counting
middleware/    — Auth, rate limiting, CORS, logging, request distribution
setting/       — Configuration management (ratio, model, operation, system, performance)
common/        — Shared utilities (JSON, crypto, Redis, env, rate-limit, quota math)
dto/           — Data transfer objects (request/response structs)
constant/      — Constants (API types, channel types, context keys)
types/         — Type definitions (relay formats, file sources, errors)
i18n/          — Backend i18n (go-i18n, en/zh)
oauth/         — OAuth provider implementations
pkg/           — Internal packages (cachex, ionet, billingexpr)
```

### Frontend (`web/`)

- `web/default/` — Main frontend (React 19, Rsbuild, Base UI, Tailwind CSS)
- `web/classic/` — Classic frontend (React 18, Vite, Semi Design)
- Both themes are built into the Go binary via `//go:embed`

### Key Data Flow

```
Client Request → Gin Router → Middleware Stack → Controller → Service → Model (DB)
                                          ↘ Distributor (channel selection)
                                            → relay/channel/{provider} Adaptor
                                              → upstream AI API → response → billing
```

### Relay System

The `relay/channel/` directory contains an `Adaptor` interface (in `adapter.go`) that each provider implements:
- `Init`, `GetRequestURL`, `SetupRequestHeader`
- `ConvertOpenAIRequest`, `DoRequest`, `DoResponse`
- `GetModelList`, `GetChannelName`

Task-based platforms (Midjourney, Suno, video) implement `TaskAdaptor` separately.

## Development Commands

### Backend (Go)

```bash
# Run directly (requires DB setup via .env or defaults to SQLite)
go run main.go

# Run tests for a specific package
go test ./controller/...
go test ./service/...
go test ./model/...
go test ./common/...

# Run a single test
go test ./controller/ -run TestChannelAuthz -v

# Build binary
go build -ldflags "-s -w -X 'github.com/QuantumNous/new-api/common.Version=$(cat VERSION)'" -o new-api

# Download dependencies
go mod download
```

### Frontend (web/default/)

```bash
cd web/default

# Install dependencies
bun install

# Dev server (port 5173, proxied to API on :3000)
bun run dev

# Production build
bun run build

# Type checking
bun run typecheck

# Lint
bun run lint

# Format
bun run format

# Check formatting
bun run format:check

# Sync i18n translation files
bun run i18n:sync
```

### Full Stack with Make

```bash
# Start full dev environment (Docker API + web dev server)
make dev

# Start only API services via Docker (PostgreSQL + Redis + API)
make dev-api

# Rebuild API service after Go changes
make dev-api-rebuild

# Start only web dev server
make dev-web

# Build both frontend themes
make build-all-web

# Build just default theme
make build-web

# Start API directly (go run)
make start-api

# Reset setup wizard state (dev only)
make reset-setup
```

### Docker

```bash
# Production build
docker build -t new-api .

# Dev environment
docker compose -f docker-compose.dev.yml up -d
docker compose -f docker-compose.dev.yml down
docker compose -f docker-compose.dev.yml up -d --build new-api  # rebuild after Go changes
```

## Key Models and Database

- `model/User` — Users, roles, groups, quota
- `model/Channel` — AI provider configurations (type, key, model mapping)
- `model/Token` — API tokens with rate limits, model restrictions
- `model/Log` — Request logs (supports separate log DB)
- `model/Ability` — Channel-model-group mappings for routing
- `model/Subscription` — Subscription plans and billing
- `model/Redemption` — Redemption codes for top-ups
- `model/Options` — System-wide config key-value store
- `model/Task` — Async task tracking (Midjourney, Suno, video)
- `model/SystemTask` — Scheduled system task management

## Important Patterns

- **New channels**: Add a directory under `relay/channel/` implementing the `Adaptor` interface, register it in `relay/channel/adapter.go`, add a constant in `constant/`, and add a channel type in `common/api_type.go`.
- **Billing**: Always use `common/quota_math.go` helpers (`QuotaFromFloat`, `QuotaRound`, `QuotaFromDecimal`) for quota conversions. Never use bare casts.
- **DB locking**: Use `model.LockForUpdate(tx)` for row locks (handles SQLite compatibility).
- **JSON marshal/unmarshal**: Must use `common.Marshal`, `common.Unmarshal` etc. from `common/json.go` — never `encoding/json` directly.
- **i18n**: Backend uses `nicksnyder/go-i18n/v2`; frontend uses `i18next`/`react-i18next`. Frontend translation keys are English source strings in flat JSON files.
- **Configuration**: Environment variables (`.env` file) + database `options` table. System settings are loaded at startup via `model.InitOptionMap()` and synced periodically.