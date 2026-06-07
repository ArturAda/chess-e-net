# Chessemag - Multiplayer Chess Platform

[🇺🇸 English](README.md) | [🇷🇺 Русский](README.ru.md)

[![Build and Test](https://github.com/ArturAda/chess-e-net/actions/workflows/ci.yml/badge.svg)](https://github.com/ArturAda/chess-e-net/actions/workflows/ci.yml)
<!-- coverage-badge -->[![Coverage](https://img.shields.io/badge/Coverage-unknown-lightgrey.svg)](#)<!-- /coverage-badge -->

Chessemag is a modern, extensible multiplayer chess platform built with Go (Golang) and a vanilla JavaScript frontend. It supports classic 8x8 chess alongside custom modern variants (10x10, 12x12), real-time matchmaking, ELO-based ranked play, and interactive game histories.

This project is designed to be a solid foundation for anyone looking to build a real-time turn-based multiplayer game or extend a chess application with new rulesets and custom boards.

## Features

* **Multiplayer Core:** Real-time gameplay via WebSockets with robust connection management and reconnect grace periods.
* **Matchmaking:** Automated queue system supporting both casual and ranked modes based on board size and time controls.
* **Ranked System:** ELO-based rating system scoped by game format (e.g., 8x8 10-minute, 10x10 15-minute).
* **Game History & Analysis:** Detailed recording of every match, supporting step-by-step playback, visual state preservation, and move notation.
* **Extensible Game Engine:** An interface-driven game registry (`core.Registry`) that makes it simple to add new board sizes or entirely new game variants without rewriting the core engine.
* **Customizable Frontend:** Players can upload or select custom pieces, board textures, and background environments, which are synchronized and saved in their game history.
* **Interactive Chat:** Sticker-based emoji chat during live games.

## Tech Stack

### Backend
* **Language:** Go (Golang)
* **Web Framework:** [Gin](https://gin-gonic.com/)
* **Database:** PostgreSQL with [GORM](https://gorm.io/) for ORM
* **Real-time:** WebSockets ([gorilla/websocket](https://github.com/gorilla/websocket))
* **Authentication:** JWT (JSON Web Tokens)

### Frontend
* **Language:** Vanilla JavaScript, HTML5, CSS3
* **Build Tool:** Vite
* **Chess Logic/UI:** Custom implementation for modern boards, leveraging a modified `chessboard.js` for classic 8x8 rendering.

## Getting Started

### Prerequisites

* [Go](https://go.dev/doc/install) (1.20 or later)
* [Node.js](https://nodejs.org/) (for frontend dependencies and Vite)
* [PostgreSQL](https://www.postgresql.org/) database
* [Docker](https://www.docker.com/) and Docker Compose (optional, but recommended for easy setup)

### Environment Variables

Create a `configs/.env` file in the `chess-monolith` directory. You can create one with the following structure:

```env
PORT=8080
DB_DSN=host=localhost user=postgres password=postgres dbname=chess_db port=5432 sslmode=disable
JWT_SECRET=your_super_secret_key_here
FRONTEND_DIST_DIR=frontend/dist
```

### Local Development Setup

1. **Install Backend Dependencies**
   ```bash
   cd chess-monolith
   go mod tidy
   ```

2. **Database Setup**
   Ensure your PostgreSQL instance is running and matches the `DB_DSN` in your `.env` file. Apply the schema migrations:
   ```bash
   make migrate-up
   ```

3. **Build the Frontend**
   ```bash
   cd frontend
   npm install
   npm run build
   cd ..
   ```

4. **Run the Server**
   ```bash
   make run
   # Or using go run directly:
   # go run cmd/server/main.go
   ```
   The application will be available at `http://localhost:8080`.

### Docker Setup

To run the entire stack (Database + Application) via Docker Compose:

```bash
cd chess-monolith
docker-compose up --build
```
This will start PostgreSQL and the Go backend automatically. Note: You still need to ensure the frontend is built into `frontend/dist` before building the Docker image if you want it served by the Go binary.

## Architecture & Code Structure

The project follows a modular monolith architecture, separating concerns into distinct packages.

```text
chess-monolith/
├── cmd/
│   └── server/          # Application entry point (main.go)
├── configs/             # Environment configurations
├── frontend/            # Vanilla JS frontend application (Vite)
├── internal/            # Core backend application code
│   ├── game/            # Game state management, database repository, and HTTP transport
│   │   ├── core/        # Core game interfaces and generic board logic
│   │   ├── modes/       # Specific game rulesets (classic, modern10, modern12)
│   │   └── session/     # Active in-memory game session management
│   ├── matchmaking/     # Queue system and opponent matching logic
│   ├── users/           # User authentication, profiles, and ELO rating logic
│   └── ws/              # WebSocket hub, client connections, and message routing
├── migrations/          # PostgreSQL database migration scripts
├── pkg/                 # Reusable utility packages
│   ├── ebox/            # Custom error handling
│   ├── elo/             # ELO rating calculation logic
│   └── jwtutil/         # JWT generation and validation
└── Makefile             # Task automation (build, run, migrate)
```

## How to Extend the Game

### Adding a New Game Mode (e.g., Chess960, Custom Board Size)

The platform is designed around a `core.Registry` that maps game mode names to their specific rulesets (`core.Ruleset`).

1. **Create the Ruleset implementation:** Implement the `core.Ruleset` interface for your new mode in a new package under `internal/game/modes/`.
   ```go
   // internal/game/modes/mycustommode/mycustommode.go
   type CustomRuleset struct {}
   func (r *CustomRuleset) ValidateMove(board *core.Board, move core.Move, playerColor core.Color) error { ... }
   func (r *CustomRuleset) CheckGameStatus(board *core.Board, currentPlayer core.Color) core.GameStatus { ... }
   // ... implement other interface methods
   ```
2. **Register the Mode:** Expose a `Register` function and call it during server initialization in `cmd/server/main.go`.
   ```go
   // cmd/server/main.go
   func initGameRegistry() *core.Registry {
       registry := core.NewRegistry()
       classic.Register(registry)
       modern.Register(registry)
       mycustommode.Register(registry) // Register your new mode
       return registry
   }
   ```
3. **Update Frontend Matchmaking:** Ensure your frontend can request matchmaking for this new mode name via the `WebSocketMatchmakingStrategy` in `lobby.js`.

### Modifying the Frontend

The frontend uses Vite for bundling. The core logic resides in `frontend/js/`.
* `lobby.js`: Handles UI state, matchmaking, user settings, history viewing, and WebSocket event delegation.
* `board.js`: (If applicable) Extends or manages the rendering of custom board sizes alongside the classic `chessboard.js`.
* `socket.js`: Manages the WebSocket connection lifecycle and payload formatting.

After modifying files in `frontend/`, run `npm run build` in the `frontend/` directory to update the `dist/` folder served by the Go backend.

## API Documentation

While gameplay primarily occurs over WebSockets, the application provides REST endpoints for authentication and history.

* **POST** `/api/register` - Create a new user account.
* **POST** `/api/login` - Authenticate and receive a JWT.
* **GET** `/api/me` - Get current user profile and ratings (Requires Auth).
* **GET** `/api/games` - List game history for the current user (Requires Auth).
* **GET** `/api/games/:id` - Get detailed state and move history for a specific game (Requires Auth).
* **GET** `/api/leaderboard` - Get top players for a specific mode/board size.

## License

This project is open-source and available under the [MIT License](LICENSE).