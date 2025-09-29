# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

New API is a next-generation AI model gateway and asset management system built in Go with a React frontend. It serves as a unified proxy/gateway for multiple AI service providers (OpenAI, Claude, Gemini, etc.), providing features like token management, rate limiting, billing, and request routing.

## Development Commands

### Backend Development
- **Start development server**: `go run main.go`
- **Build for production**: `go build -ldflags "-s -w -X 'one-api/common.Version=$(git describe --tags)'" -o new-api`
- **Run with frontend**: `make all` (builds frontend then starts backend)

### Frontend Development (uses Bun, not npm)
- **Install dependencies**: `cd web && bun install`
- **Development server**: `cd web && bun run dev`
- **Build for production**: `cd web && bun run build`
- **Lint check**: `cd web && bun run lint`
- **Lint fix**: `cd web && bun run lint:fix`
- **ESLint**: `cd web && bun run eslint`

### Docker Development
- **Start services**: `docker-compose up -d`
- **View logs**: `docker-compose logs -f new-api`
- **Stop services**: `docker-compose down`

## Architecture Overview

### Backend Structure (Go)
- **`main.go`** - Application entry point, embedded frontend assets
- **`router/`** - HTTP route definitions (api-router.go, web-router.go, relay-router.go)
- **`controller/`** - HTTP handlers for API endpoints (~39 files)
- **`service/`** - Business logic layer (billing, channels, conversion, etc.)
- **`model/`** - Database models and data access (GORM-based)
- **`relay/`** - Core gateway functionality for different AI providers
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

### Database Support
- **Default**: SQLite (development)
- **Production**: MySQL ≥5.7.8 or PostgreSQL ≥9.6
- **ORM**: GORM with auto-migration support
- **Caching**: Optional Redis for performance

## Key Features Architecture

### AI Provider Integration
- **`relay/channel/`** - Provider-specific implementations
- **`relay/common/`** - Shared relay logic
- **Request Format Conversion** - OpenAI ↔ Claude ↔ Gemini format translation
- **Rate Limiting** - Per-user and per-channel limits
- **Load Balancing** - Weighted random channel selection

### Authentication & Authorization
- Multiple login methods: Traditional, LinuxDO, Telegram, OIDC
- JWT-based session management
- Token-based API access with usage tracking

### Billing & Usage Tracking
- **Token-based billing** - Per-token or per-request pricing
- **Channel billing** - Provider-specific cost calculation
- **Usage analytics** - Dashboard with VChart visualizations
- **Cache billing** - Reduced costs for cache hits

## Environment Configuration

Key environment variables (see `.env.example`):
- **`SQL_DSN`** - Database connection string
- **`REDIS_CONN_STRING`** - Redis connection (optional)
- **`SESSION_SECRET`** - Required for multi-node deployment
- **`CRYPTO_SECRET`** - Required for multi-node Redis sharing
- **`STREAMING_TIMEOUT`** - Stream response timeout (default: 300s)
- **`PORT`** - Server port (default: 3000)

## Development Guidelines

### Constant Package Rules
The `constant/` package has strict dependency rules:
- Only Go standard library imports allowed
- No business logic or external package dependencies
- Constants only - see `constant/README.md` for details

### Testing
- Limited test coverage currently
- Channel testing available via `controller/channel-test.go`
- Integration tests run through Docker Compose setup

### Code Style
- **Backend**: Follow Go conventions (gofmt, go vet)
- **Frontend**: ESLint + Prettier configured (single quotes, JSX single quotes)
- **Database**: GORM conventions with proper migrations

## Common Development Patterns

### Adding New AI Provider
1. Create provider implementation in `relay/channel/`
2. Add constants in `constant/`
3. Update routing in `relay/` handlers
4. Add provider-specific models in `model/`
5. Update frontend provider selection UI

### API Endpoint Development
1. Define routes in `router/`
2. Implement handlers in `controller/`
3. Add business logic in `service/`
4. Update models if needed in `model/`
5. Add middleware if required

### Frontend Feature Development
1. Create components in `web/src/components/`
2. Add routing in React Router setup
3. Implement API calls using Axios
4. Style with Semi-UI components + Tailwind
5. Add i18n support for multiple languages