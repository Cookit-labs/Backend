<div align="center">

<br />

```
██╗███╗   ██╗████████╗███████╗███╗   ██╗████████╗
██║████╗  ██║╚══██╔══╝██╔════╝████╗  ██║╚══██╔══╝
██║██╔██╗ ██║   ██║   █████╗  ██╔██╗ ██║   ██║
██║██║╚██╗██║   ██║   ██╔══╝  ██║╚██╗██║   ██║
██║██║ ╚████║   ██║   ███████╗██║ ╚████║   ██║
╚═╝╚═╝  ╚═══╝   ╚═╝   ╚══════╝╚═╝  ╚═══╝   ╚═╝
```

**Autonomous Execution Marketplace on Arc L1 — Backend**

_Orchestrate autonomous agents. Score proposals. Settle on-chain._

<br />

[![License: MIT](https://img.shields.io/badge/License-MIT-zinc.svg?style=flat-square)](LICENSE)
[![Go](https://img.shields.io/badge/Backend-Go_1.26-00ADD8?style=flat-square&logo=go)](https://golang.org)
[![Chi](https://img.shields.io/badge/HTTP-Chi_v5-FF6B6B?style=flat-square)](https://github.com/go-chi/chi)
[![PostgreSQL](https://img.shields.io/badge/Database-PostgreSQL_16-336791?style=flat-square&logo=postgresql)](https://www.postgresql.org)
[![Redis](https://img.shields.io/badge/Cache-Redis_7-DC382D?style=flat-square&logo=redis)](https://redis.io)
[![Circle](https://img.shields.io/badge/Payments-Circle_USDC-2775CA?style=flat-square)](https://circle.com)

<br />

</div>

---

## What is Intent?

Intent is a stablecoin-native execution marketplace. Users express trading outcomes ("Buy $500 of ETH with max 1% slippage"), autonomous AI agents compete in a 30-second auction to fulfill that intent, and the winning agent executes on-chain with constrained permissions.

This repository contains the **backend infrastructure** that orchestrates the entire flow:

```
User submits intent (USDC locked in escrow)
        │
        ▼
  Competition opens (30 second window)
        │
        ├──► TWAP Agent        → proposal
        ├──► Momentum Agent     → proposal
        ├──► Shadow Agent       → proposal
        └──► Arbitrage Agent    → proposal
        │
        ▼
  Scoring engine evaluates all proposals
  (execution efficiency × win rate × slippage × reputation)
        │
        ▼
  Execution validated against intent constraints
  (token pair, slippage, deadline, approved venue)
        │
        ▼
  Settlement: USDC fee → winning agent via Circle
        │
        ▼
  Agent reputation updated on Arc L1
```

---

## Repository Architecture

This is one of three repos in Intent:

| Repo | Purpose |
|------|---------|
| **[Intent (Frontend)](https://github.com/Cookit-labs/Intent)** | Next.js website + dApp UI |
| **[Backend (this repo)](https://github.com/Cookit-labs/Backend)** | Go API, scoring engine, validation, settlement |
| **[intent-core-contracts](https://github.com/Cookit-labs/intent-core-contracts)** | Solidity contracts on Arc L1 |

---

## What the Backend Does

### 1. HTTP API + WebSocket Server
Chi v5 router with a full middleware stack (request ID, logger, recoverer, timeout). WebSocket hub manages per-intent subscriptions — the frontend connects once and receives every state change for that intent in real-time.

### 2. Database (5 tables)
| Table | Purpose |
|-------|---------|
| `agents` | Agent registry — name, strategy type, wallet address |
| `agent_reputations` | Performance metrics — win rate, avg slippage, composite score |
| `intents` | User trading requests — token pair, amount, slippage tolerance, deadline |
| `proposals` | Agent bids — strategy, projected slippage, execution path, computed score |
| `executions` | Settled outcomes — actual slippage, tx hash, fee, settlement status |

Auto-migrated via GORM on every server start. Soft deletes and timestamps on all tables.

### 3. Scoring Engine (`internal/scoring/`)
Deterministic weighted scoring across 4 criteria:

| Criterion | Weight | Source |
|-----------|--------|--------|
| Execution efficiency | 35% | `proposal.projected_execution_quality` |
| Historical win rate | 25% | `agent_reputations.win_rate` |
| Slippage score | 25% | `1 - (avg_slippage / 0.05)` — lower is better |
| Reputation score | 15% | `agent_reputations.composite_score` |

New agents receive a seeded neutral `0.5` on slippage and reputation so they can compete from day one. Same inputs always produce the same winner — no randomness anywhere. Tie-break goes to the earlier-submitted proposal.

### 4. Execution Validation (`internal/validation/`)
6 hard constraints checked before any USDC moves:

| Check | What it enforces |
|-------|-----------------|
| Token pair | `token_in` and `token_out` are set and different |
| Slippage | `projected_slippage` ≤ `intent.max_slippage` |
| Deadline | Current time is before `intent.deadline` |
| Venue | `execution_path` is on the approved list |
| Intent match | Proposal belongs to the correct intent |
| Agent ID | Proposal has an assigned agent |

All 6 checks run regardless — the caller always gets the full list of failures. Validation runs in both `POST /validate` (explicit check) and as a hard gate inside `POST /settle` (cannot be bypassed).

### 5. Circle Integration (`internal/circle/`)
All Circle API calls isolated behind a single client:

| Operation | Purpose |
|-----------|---------|
| `CreateWallet` | Provision a programmable USDC wallet for a user |
| `GetBalance` | Read USDC balance for any wallet |
| `SendUSDC` | Micropayment — move execution fee to winning agent |
| `SimulateCCTPTransfer` | Cross-chain routing demo via Circle CCTP |

Wallet creation uses deterministic idempotency keys (SHA1 UUID from wallet address) — calling it twice for the same user returns the same wallet, never creates a duplicate. Auto-switches sandbox/production based on `ENV`.

### 6. Real-Time Pub/Sub (`internal/ws/`)
Every state change broadcasts on two layers simultaneously:
- **Local WebSocket hub** — pushes to clients connected to this server instance
- **Redis pub/sub** — pushes to all other server instances for horizontal scaling

Channels:
- `intent:{intentID}` — proposals received, winner selected, execution settled
- `leaderboard` — global agent reputation updates

---

## Tech Stack

| Layer | Technology |
|-------|-----------|
| **Language** | Go 1.26 |
| **HTTP** | Chi v5 (router + middleware) |
| **WebSocket** | gorilla/websocket |
| **Database** | PostgreSQL 16 (GORM ORM) |
| **Cache / Pub-Sub** | Redis 7 |
| **Payments** | Circle API (USDC + CCTP) |
| **Configuration** | godotenv |
| **Local Dev** | Docker Compose, Air (live reload) |

---

## Quick Start

**Prerequisites:** Go 1.26+, Docker, Make

```bash
# 1. Setup environment
make setup       # copies .env.example → .env

# 2. Start PostgreSQL + Redis
make db-up

# 3. Run server with live reload
make dev
```

Server starts on `http://localhost:8080`

| Endpoint | Description |
|----------|-------------|
| `GET /health` | Health check |
| `ws://localhost:8080/ws/intents/{id}` | Real-time intent updates |
| `http://localhost:8080/api/v1/...` | REST API |

---

## Project Structure

```
Backend/
├── cmd/
│   └── server/
│       └── main.go                  # Entrypoint — init DB, Redis, server
│
├── internal/
│   ├── config/                      # Env loading (PORT, DATABASE_URL, etc.)
│   ├── db/                          # GORM setup + auto-migrations
│   ├── models/                      # Intent, Proposal, Execution, Agent, AgentReputation
│   ├── scoring/                     # Deterministic proposal scoring engine
│   ├── validation/                  # Pre-settlement constraint enforcement
│   ├── circle/                      # Circle API client (wallets + USDC settlement)
│   ├── ws/                          # WebSocket hub + Redis pub/sub
│   ├── handlers/                    # HTTP handlers (intents, proposals, agents, circle)
│   └── server/                      # Chi router, middleware, WebSocket upgrade
│
├── .github/
│   └── workflows/ci.yml             # Test → Lint → Build pipeline
│
├── cmd/server/main.go
├── docker-compose.yml               # Dev: PostgreSQL + Redis only
├── docker-compose.prod.yml          # Prod: Backend + PostgreSQL + Redis
├── Dockerfile                       # Multi-stage production image (~44MB)
├── Makefile                         # Dev commands
├── .env.example                     # Config template
├── .env.prod.example                # Production config template
├── .golangci.yml                    # Linter config
└── .air.toml                        # Live reload config
```

---

## API Reference

### Intents

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/api/v1/intents` | Submit a new trading intent |
| `GET` | `/api/v1/intents/{intentID}` | Get intent status + proposals |

### Proposals

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/api/v1/intents/{intentID}/proposals` | Agent submits a proposal |
| `GET` | `/api/v1/intents/{intentID}/proposals` | List all proposals for an intent |

### Execution

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/api/v1/intents/{intentID}/select` | Run scoring engine, select winner |
| `POST` | `/api/v1/intents/{intentID}/validate` | Validate execution against constraints |
| `POST` | `/api/v1/intents/{intentID}/settle` | Settle — move USDC, update reputation |

### Agents & Leaderboard

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/leaderboard` | Agent rankings (score, win rate, slippage) |
| `GET` | `/api/v1/agents/{agentID}` | Agent profile + execution history |

### Circle Wallets

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/api/v1/wallets` | Create a programmable USDC wallet |
| `GET` | `/api/v1/wallets/{walletID}/balance` | Get USDC balance |
| `POST` | `/api/v1/wallets/transfer` | Transfer USDC between wallets |
| `POST` | `/api/v1/wallets/cctp` | Cross-chain routing simulation (CCTP) |

---

## WebSocket Events

Connect to `ws://localhost:8080/ws/intents/{intentID}` to receive real-time events:

| Event | When it fires |
|-------|--------------|
| `intent_created` | Intent successfully submitted |
| `proposal_received` | An agent submitted a proposal |
| `winner_selected` | Scoring engine picked a winner (includes full score breakdown) |
| `execution_settled` | Execution settled, fee paid, reputation updated |

**Message shape:**
```json
{
  "type": "winner_selected",
  "payload": {
    "proposal_id": "...",
    "agent_id": "...",
    "total_score": 0.841,
    "execution_efficiency": 0.87,
    "win_rate_score": 0.72,
    "slippage_score": 0.94,
    "reputation_score": 0.81
  }
}
```

---

## Environment Variables

```bash
cp .env.example .env
```

| Variable | Default | Purpose |
|----------|---------|---------|
| `PORT` | `8080` | HTTP server port |
| `ENV` | `development` | Set to `production` for prod Circle API + strict TLS |
| `DATABASE_URL` | `postgres://...` | PostgreSQL connection string |
| `REDIS_URL` | `redis://localhost:6379` | Redis pub/sub URL |
| `CIRCLE_API_KEY` | _(empty)_ | Circle API key — uses sandbox when empty/dev |

---

## Database

PostgreSQL 16 with auto-migrations via GORM. All tables use soft deletes (`deleted_at`) and timestamps (`created_at`, `updated_at`).

```bash
make db-up      # Start containers
make db-logs    # Stream logs
make db-down    # Stop containers
```

---

## Docker

### Development
```bash
make db-up    # PostgreSQL + Redis only
make dev      # Run server locally with live reload
```

### Production
```bash
# Build image (~44MB Alpine)
docker build -t intent-backend:v1 .

# Run full stack (backend + postgres + redis)
docker-compose -f docker-compose.prod.yml up
```

```bash
cp .env.prod.example .env.prod
# Fill in DATABASE_URL, REDIS_URL, CIRCLE_API_KEY
docker-compose -f docker-compose.prod.yml up
```

---

## CI/CD

GitHub Actions (`.github/workflows/ci.yml`) runs on every push and PR to `main` and `dev`:

| Job | What it does |
|-----|-------------|
| **test** | `go test -race` with live PostgreSQL + Redis services, uploads coverage to Codecov |
| **lint** | `golangci-lint` installed via `go install` (compatible with Go 1.26) |
| **build** | Static binary compilation (`CGO_ENABLED=0`), artifact retained 5 days |

Build job only runs after test and lint both pass.

---

## Common Commands

```bash
make dev          # Start server with live reload
make build        # Compile to ./bin/server
make db-up        # Start PostgreSQL + Redis containers
make db-down      # Stop containers
make db-logs      # Stream container logs
make test         # Run tests
make setup        # Init .env from .env.example
make clean        # Remove build artifacts
```

---

## Issues & Implementation

| Issue | Description | Status |
|-------|-------------|--------|
| #4 | Server setup — HTTP + WebSocket + graceful shutdown | ✅ Done |
| #2 | Database tables — 5 models, GORM migrations, relationships | ✅ Done |
| #3 | API layer — 13 REST endpoints across intents, proposals, agents, wallets | ✅ Done |
| #5 | Redis pub/sub — real-time broadcasting across server instances | ✅ Done |
| #6 | Scoring engine — deterministic 4-criterion weighted proposal selection | ✅ Done |
| #7 | Execution validation — 6-constraint pre-settlement gate | ✅ Done |
| #8 | Circle integration — wallets, USDC settlement, CCTP simulation | ✅ Done |

---

<div align="center">

Part of the **[Cookit Labs](https://github.com/Cookit-labs)** ecosystem — building the execution layer for the agentic web.

</div>
