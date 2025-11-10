# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a management dashboard web application for [baru-reso-headless-container](https://github.com/hantabaru1014/baru-reso-headless-container), designed to manage Resonite headless containers. The system provides session management, container lifecycle management, and a web-based dashboard for monitoring and controlling headless Resonite instances.

## Development Commands

### Frontend (React + Vite)
- `pnpm dev` - Start development server
- `pnpm build` - Build for production
- `pnpm preview` - Preview production build
- `pnpm lint` - Run ESLint and Prettier checks
- `pnpm lint:fix` - Auto-fix ESLint and Prettier issues
- `pnpm typecheck` - Run TypeScript type checking
- `pnpm storybook` - Start Storybook development server
- `pnpm build-storybook` - Build Storybook

### Backend (Go)
- `make build.cli` - Build CLI binary to `./bin/brhcli`
- `make build.docker` - Build Docker image
- `make gen.proto` - Generate protobuf code
- `make gen.wire` - Generate dependency injection code
- `make gen.sqlc` - Generate SQL query code
- `make lint.proto` - Format and lint protobuf files
- `make exec.psql` - Connect to PostgreSQL database

### Database Migrations
- `./bin/migrate create -ext sql -dir db/migrations <file_name>` - Create new migration file
- `./bin/migrate -path db/migrations -database "${DB_URL}" up` - Run migrations
- `./brhcli user create <email> <password> <userID>` - Create admin user

## Architecture Overview

### Backend Structure
The Go backend follows Clean Architecture principles:

- **Domain Layer** (`domain/`): Core business entities and errors
- **Use Case Layer** (`usecase/`): Business logic and application services
- **Adapter Layer** (`adapter/`): External interfaces (repositories, gRPC services)
- **Infrastructure** (`db/`, `lib/`): Database, logging, external API clients

### Key Components

#### Dependency Injection
Uses Google Wire for dependency injection. Main wire configuration in `app/wire.go`:
- `InitializeServer()` - Sets up web server dependencies
- `InitializeCli()` - Sets up CLI dependencies

#### Database
- PostgreSQL with migrations in `db/migrations/`
- SQLC for type-safe SQL query generation
- Queries defined in `db/queries/`

#### gRPC Services
- Protocol buffer definitions in `proto/`
- Generated code in `pbgen/`
- Service implementations in `adapter/rpc/`

#### Host Management
- Docker integration for container lifecycle management
- Host connector abstraction in `adapter/hostconnector/`
- Container image management and version resolution

### Frontend Structure
Modern React application with:

- **React Router** for navigation
- **TanStack Query** for server state management
- **Jotai** for client state management
- **Connect-Web** for gRPC-Web communication
- **Tailwind CSS + shadcn/ui** for styling (components in `front/src/components/ui/`)
- **React Hook Form + Zod** for form handling
- **Vitest + Storybook** for testing

Key directories:
- `front/src/components/` - Reusable UI components
- `front/src/components/ui/` - shadcn/ui components
- `front/src/pages/` - Page components
- `front/src/hooks/` - Custom React hooks
- `front/src/libs/` - Utility functions

## Development Workflow

1. **Code Generation**: Run `make gen.proto`, `make gen.wire`, and `make gen.sqlc` after schema changes
2. **Database Changes**: Create migration files using `./bin/migrate create -ext sql -dir db/migrations <file_name>` and run with `./bin/migrate -path db/migrations -database "${DB_URL}" up`
3. **Frontend Development**: Use `pnpm dev` for hot reload during development
4. **Before Committing**: Run `pnpm lint` and `pnpm typecheck` to ensure code quality

## Important Notes

- The system manages Docker containers running Resonite headless instances
- Authentication is handled through JWT tokens
- The frontend communicates with the backend via gRPC-Web
- Database migrations use golang-migrate CLI tool
- The CLI tool (`brhcli`) provides administrative functions
- Use pnpm for all frontend package management