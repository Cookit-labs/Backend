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
  Backend scoring engine evaluates proposals
        │
        ▼
  Winning agent selected and executed
        │
        ▼
  Execution validated on Arc L1
        │
        ▼
  Settlement: USDC moved → agent fee paid → reputation updated
```

---

## Repository Architecture

This is one of three repos in Intent:

| Repo | Purpose |
|------|---------|
| **[Intent (Frontend)](https://github.com/Cookit-labs/Intent)** | Next.js website + dApp UI |
| **[Backend (this repo)](https://github.com/Cookit-labs/Backend)** | Go API, scoring engine, settlement orchestration |
| **[intent-core-contracts](https://github.com/Cookit-labs/intent-core-contracts)** | Solidity contracts on Arc L1 |

---

## Backend Scope

The backend provides:

### 1. **HTTP API + WebSocket Server**
- REST endpoints for intent submission, status polling, leaderboard queries
- WebSocket subscriptions per intent for real-time competition updates
- Health checks and admin endpoints

### 2. **Core Data Models**
- `Intent` — user's trading outcome request
- `Proposal` — agent's bid to execute (strategy, projected slippage, execution path)
- `Execution` — settled outcome (actual slippage, tx hash, fee)
- `Agent` — agent registry (TWAP, Momentum, Shadow, etc.)
- `AgentReputation` — on-chain seeded performance metrics (win rate, avg slippage, score)

### 3. **Scoring Engine**
Deterministic proposal selection: weights execution efficiency, historical win rate, average slippage performance, and composite reputation into a single score. Winner selected.

### 4. **Execution Validation**
Pre-settlement constraint enforcement:
- Token pair matches intent spec
- Slippage within tolerance bounds
- Deadline not passed
- Venue is approved

### 5. **Circle Integration**
- Programmable wallet management (one per user)
- USDC balance reads
- Micropayment settlement (agent execution fee)
- Optional CCTP simulation for multi-chain routing

### 6. **Real-Time Pub/Sub**
Redis-backed WebSocket broadcasting — frontend sees live proposal submissions, scoring, and winner selection as it happens.

---

## Tech Stack

| Layer | Technology |
|-------|-----------|
| **Language** | Go 1.26 |
| **HTTP** | Chi v5 (router) |
| **WebSocket** | gorilla/websocket |
| **Database** | PostgreSQL 16 (GORM ORM) |
| **Cache/Pub-Sub** | Redis 7 |
| **Configuration** | godotenv |
| **Local Dev** | Docker Compose, Air (live reload) |

---

## Development

### Prerequisites
- Go 1.26+
- Docker & Docker Compose
- Make (optional but recommended)

### Quick Start

**1. Setup environment and start services**
```bash
make setup
make db-up
```

**2. Run the server**
```bash
make dev
```

Server starts on `http://localhost:8080`
- Health check: `GET /health`
- WebSocket: `ws://localhost:8080/ws/intents/{intentID}`
- API: `http://localhost:8080/api/v1/...` (populated in Issue #3)

**3. Or build and run manually**
```bash
go build -o bin/server ./cmd/server
./bin/server
```

---

## Project Structure

```
Backend/
├── cmd/
│   └── server/              # Entrypoint
│       └── main.go
│
├── internal/
│   ├── config/              # Env loading
│   ├── db/                  # Database layer (GORM setup, migrations)
│   ├── models/              # Data models (Intent, Proposal, Execution, etc.)
│   ├── server/              # HTTP server + router setup
│   ├── ws/                  # WebSocket hub (subscriptions, broadcast)
│   └── handlers/            # API endpoints (populated in Issue #3)
│
├── go.mod / go.sum
├── Makefile                 # Development commands
├── docker-compose.yml       # Postgres + Redis
├── .env.example             # Environment template
└── .air.toml               # Live reload config
```

---

## Environment Variables

```bash
# Copy from template and edit as needed
cp .env.example .env
```

| Variable | Default | Purpose |
|----------|---------|---------|
| `PORT` | 8080 | HTTP server port |
| `ENV` | development | Environment mode |
| `DATABASE_URL` | postgres://... | PostgreSQL connection string |
| `REDIS_URL` | redis://localhost:6379 | Redis pub/sub URL |
| `CIRCLE_API_KEY` | (empty) | Circle API key for wallet/settlement |

---

## Database

PostgreSQL with auto-migrations via GORM.

**Tables:**
- `agents` — agent registry
- `agent_reputations` — performance scores (synced to Arc L1)
- `intents` — user requests
- `proposals` — agent bids
- `executions` — settled outcomes

**Start services:**
```bash
make db-up
```

**View logs:**
```bash
make db-logs
```

**Stop:**
```bash
make db-down
```

---

## Common Commands

```bash
make dev          # Start server with live reload (watching for changes)
make build        # Compile server binary to ./bin/server
make db-up        # Start PostgreSQL and Redis containers
make db-down      # Stop containers
make db-logs      # Stream container logs
make test         # Run tests
make setup        # Initialize environment (.env from .env.example)
make clean        # Remove build artifacts
```

---

## Development Workflow

### Adding new API endpoints

1. Create handler in `internal/handlers/`
2. Mount route in `internal/server/buildRouter()`
3. Use `srv.DB()` to access database, `srv.Hub()` to broadcast WebSocket events

### Example: Submit Intent

```go
// POST /api/v1/intents
func (h *Handler) CreateIntent(w http.ResponseWriter, r *http.Request) {
    var req CreateIntentRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    intent := &models.Intent{
        ID:          uuid.New().String(),
        UserWallet:  req.UserWallet,
        TokenIn:     req.TokenIn,
        TokenOut:    req.TokenOut,
        AmountIn:    req.AmountIn,
        MaxSlippage: req.MaxSlippage,
        Deadline:    req.Deadline,
        Status:      "pending",
    }

    if err := h.db.Create(intent).Error; err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(intent)
}
```

### Broadcasting real-time updates

```go
// When a proposal arrives
h.hub.Broadcast(intentID, "proposal_received", proposal)

// Frontend receives via WebSocket:
// { "type": "proposal_received", "payload": {...proposal...} }
```

---

## Issues & Implementation Order

Issue tracking for planned backend work:

| Issue | Description | Status |
|-------|-------------|--------|
| #4 | Server setup (HTTP + WebSocket) | ✅ Done |
| #2 | Database tables (models + migrations) | ✅ Done |
| #3 | API layer (all REST endpoints) | 🔄 In progress |
| #5 | Redis pub/sub integration | 🔄 Planned |
| #6 | Scoring engine | 🔄 Planned |
| #7 | Execution validation service | 🔄 Planned |
| #8 | Circle integration | 🔄 Planned |

---

## Next Steps

- **Issue #3** — Implement all REST API endpoints (`POST /intents`, `GET /leaderboard`, etc.)
- **Issue #5** — Wire Redis pub/sub to WebSocket hub
- **Issue #6** — Build scoring engine logic
- **Issue #7** — Execution validation layer
- **Issue #8** — Circle wallet + USDC settlement

---

<div align="center">

Part of the **[Cookit Labs](https://github.com/Cookit-labs)** ecosystem - building the execution layer for the agentic web.

</div>
