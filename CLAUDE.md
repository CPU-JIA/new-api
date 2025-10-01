# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

New API is a next-generation AI model gateway and asset management system built in Go with a React frontend. It serves as a unified proxy/gateway for multiple AI service providers (OpenAI, Claude, Gemini, etc.), providing features like token management, rate limiting, billing, and request routing.

## Development Commands

> **Windows Note**: Commands work on Windows. Use PowerShell for best compatibility. If `make` is not available, use the direct commands shown below.

### Backend Development
- **Start development server**: `go run main.go`
- **Build for production** (PowerShell):
  ```powershell
  $VERSION = git describe --tags
  go build -ldflags "-s -w -X 'one-api/common.Version=$VERSION'" -o new-api.exe
  ```
- **Build for production** (CMD):
  ```cmd
  for /f %i in ('git describe --tags') do set VERSION=%i
  go build -ldflags "-s -w -X one-api/common.Version=%VERSION%" -o new-api.exe
  ```
- **Full build (frontend + backend)**: `make all` (requires Make for Windows)
- **Clean build artifacts**: `make clean` or manually delete files

### Testing
- **Run all tests**: `go test -v ./... -race`
- **Run all tests** (with make): `make test`
- **Test with coverage**: `make test-coverage` (generates coverage.html)
- **Run specific package tests**: `go test -v ./model/...`
- **Run specific test file**: `go test -v ./model/ability_test.go`

### Linting
- **Auto-format code**: `go fmt ./...`
- **Run go vet**: `go vet ./...`
- **Run golangci-lint** (if installed): `golangci-lint run --config .golangci.yml`
- **Run linters with make**: `make lint`
- **Linter configuration**: `.golangci.yml` (enables errcheck, gosimple, govet, gosec, misspell, etc.)

### Frontend Development (uses Bun, not npm)
- **Install dependencies**: `cd web; bun install`
- **Development server**: `cd web; bun run dev`
- **Build for production**: `cd web; bun run build`
- **Format check**: `cd web; bun run lint`
- **Format fix**: `cd web; bun run lint:fix`
- **ESLint**: `cd web; bun run eslint`
- **ESLint fix**: `cd web; bun run eslint:fix`

### Docker Development
- **Start services**: `docker-compose up -d`
- **View logs**: `docker-compose logs -f new-api`
- **Stop services**: `docker-compose down`

## Architecture Overview

### Backend Structure (Go)
- **`main.go`** - Application entry point, embeds frontend assets via `//go:embed`, initializes resources
- **`router/`** - HTTP route definitions (api-router.go, web-router.go, relay-router.go)
- **`controller/`** - HTTP handlers for API endpoints (~39 files)
- **`service/`** - Business logic layer (billing, channels, conversion, etc.)
- **`model/`** - Database models and data access (GORM-based), includes channel caching logic
- **`relay/`** - Core gateway functionality for different AI providers
  - **`relay/channel/`** - Provider-specific implementations
  - **`relay/common/`** - Shared relay logic and utilities
- **`middleware/`** - HTTP middleware (auth, logging, rate limiting)
- **`common/`** - Shared utilities and helper functions
- **`constant/`** - Global constants (strict no-dependency rule - only Go stdlib allowed)
- **`dto/`** - Data transfer objects
- **`logger/`** - Logging utilities
- **`types/`** - Type definitions

### Frontend Structure (React + Vite)
- **Framework**: React 18 with Vite build tool
- **UI Library**: Semi-UI (@douyinfe/semi-ui)
- **State Management**: Context API and component state
- **Routing**: React Router v6
- **HTTP Client**: Axios
- **Styling**: Tailwind CSS + Semi-UI components
- **Charts**: VChart with Semi theme
- **i18n**: react-i18next with auto-detection

### Database Support
- **Default**: SQLite (development)
- **Production**: MySQL ≥5.7.8 or PostgreSQL ≥9.6
- **ORM**: GORM with auto-migration support
- **Caching**: Optional Redis for performance (`REDIS_CONN_STRING`) or memory cache (`MEMORY_CACHE_ENABLED`)

## Key Features Architecture

### AI Provider Integration
- **Request Format Conversion** - OpenAI ↔ Claude ↔ Gemini format translation
- **Rate Limiting** - Per-user and per-channel limits
- **Load Balancing** - Weighted random channel selection
- **Channel Retry** - Configurable retry logic with caching support

### Authentication & Authorization
- Multiple login methods: Traditional, LinuxDO, Telegram, OIDC
- JWT-based session management
- Token-based API access with usage tracking

### Billing & Usage Tracking
- **Token-based billing** - Per-token or per-request pricing
- **Channel billing** - Provider-specific cost calculation
- **Usage analytics** - Dashboard with VChart visualizations
- **Cache billing** - Reduced costs for cache hits (configurable via `提示缓存倍率`)

## Environment Configuration

Key environment variables (see `.env.example` for full list):

### Database & Storage
- **`SQL_DSN`** - Database connection string (e.g., `root:password@tcp(localhost:3306)/oneapi`)
- **`LOG_SQL_DSN`** - Separate database for logs (optional)
- **`SQLITE_PATH`** - SQLite database path (default: `/data`)

### Caching
- **`REDIS_CONN_STRING`** - Redis connection (e.g., `redis://user:password@localhost:6379/0`)
- **`MEMORY_CACHE_ENABLED`** - Enable memory cache (default: false, auto-enabled with Redis)
- **`SYNC_FREQUENCY`** - Cache sync frequency in seconds (default: 60)

### Multi-Node Deployment
- **`SESSION_SECRET`** - **Required** for multi-node deployment to keep login state consistent
- **`CRYPTO_SECRET`** - **Required** if sharing Redis across nodes
- **`NODE_TYPE`** - Set to `master` or `slave` for multi-node setups
- **`FRONTEND_BASE_URL`** - Frontend URL for multi-node deployments

### Timeouts & Limits
- **`STREAMING_TIMEOUT`** - Stream response timeout in seconds (default: 300)
- **`RELAY_TIMEOUT`** - All requests timeout (default: 0 = no limit)
- **`PORT`** - Server port (default: 3000)

### Debugging & Logging
- **`DEBUG`** - Enable debug mode
- **`ENABLE_PPROF`** - Enable pprof profiling
- **`ERROR_LOG_ENABLED`** - Record and display error logs (default: false)

### Feature Flags
- **`GENERATE_DEFAULT_TOKEN`** - Generate initial token for new users (default: false)
- **`UPDATE_TASK`** - Update async tasks (Midjourney, Suno) (default: true)
- **`GET_MEDIA_TOKEN`** - Count image tokens (default: true)
- **`DIFY_DEBUG`** - Output Dify workflow info (default: true)
- **`GEMINI_VISION_MAX_IMAGE_NUM`** - Max images for Gemini (default: 16)

## Development Guidelines

### Constant Package Rules ⚠️
The `constant/` package has **strict dependency rules** (see `constant/README.md`):
- **Only Go standard library imports allowed**
- **No business logic or external package dependencies**
- **Constants only** - no functions, no complex logic
- Violating this will cause circular dependencies and break the build

### Code Style
- **Backend**: Follow Go conventions (gofmt, go vet), use golangci-lint
- **Frontend**: ESLint + Prettier configured with single quotes for JS/JSX
- **Database**: GORM conventions with proper migrations

### Testing Guidelines
- Test coverage is limited but growing
- Tests located in `*_test.go` files alongside source
- Key test areas: relay logic, model operations, validators, security
- Channel testing via `controller/channel-test.go`

## Common Development Patterns

### Adding New AI Provider
1. Create provider implementation in `relay/channel/<provider>/`
2. Add provider constants in `constant/` (following rules above)
3. Update routing in `relay/` handlers
4. Add provider-specific models in `model/`
5. Update frontend provider selection UI in `web/src/`
6. Add format conversion if needed in `relay/common/`

### Adding API Endpoint
1. Define routes in `router/api-router.go` or relevant router file
2. Implement handlers in `controller/<feature>-controller.go`
3. Add business logic in `service/<feature>-service.go`
4. Update models if needed in `model/<feature>.go`
5. Add middleware if required in `middleware/`
6. Update frontend API calls in `web/src/`

### Frontend Feature Development
1. Create components in `web/src/components/`
2. Add routing in React Router setup
3. Implement API calls using Axios
4. Style with Semi-UI components + Tailwind classes
5. Add i18n translations for multi-language support
6. Use VChart for data visualizations

### Database Migrations
- GORM auto-migration runs on startup
- Models in `model/` automatically create/update tables
- For complex migrations, add logic in `model/migration.go`

## Architecture Patterns

### Channel Caching System
- Channels are cached in memory/Redis for performance
- Sync frequency controlled by `SYNC_FREQUENCY`
- Cache initialization with panic recovery and retry in `main.go`
- Weighted random selection for load balancing

### Request Flow
1. Request arrives at `router/` layer
2. Passes through `middleware/` (auth, rate limit, logging)
3. Routed to `controller/` handler
4. Controller calls `service/` for business logic
5. Service interacts with `model/` for data access
6. Relay routes to appropriate provider in `relay/channel/`
7. Response formatted and returned

### Session Management
- Cookie-based sessions via gin-contrib/sessions
- JWT tokens for API authentication
- Redis session store for multi-node deployments

## Important Notes

- **Frontend assets are embedded** in the Go binary via `//go:embed web/dist`
- **Bun is used for frontend**, not npm or yarn
- **Channel retry** requires caching to be enabled
- **Multi-node deployments** must set `SESSION_SECRET` and `CRYPTO_SECRET`
- **Docker volumes** must mount `/data` for SQLite persistence

### Windows-Specific Notes
- **Binary output**: Built binaries should be named `new-api.exe` on Windows
- **Path separators**: Go handles both `/` and `\`, but use `/` in Go code for cross-platform compatibility
- **Make**: Install Make for Windows (via Chocolatey: `choco install make`) or use direct Go commands
- **PowerShell vs CMD**: PowerShell is recommended for better script compatibility
- **SQLite on Windows**: Default data path is `./data/` in the working directory
- **Line endings**: Git should be configured with `core.autocrlf=true` for proper line ending handling