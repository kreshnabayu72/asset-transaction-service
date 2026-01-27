# Analytics Reporting Service

The Analytics Reporting Service for BETS V2. This service handles analytics reporting using a distributed architecture powered by Temporal.

## 🏗 Architecture

This project has shifted from a traditional monolithic use-case pattern to a **Temporal Workflow-based architecture**. This ensures durable, reliable, and scalable execution of business logic.

### Core Components

- **API Layer (`internal/api`)**: Handles HTTP requests using Fiber. Controllers are thin wrappers that trigger Temporal Workflows.
- **Workflows (`internal/workflow`)**: Define the orchestration of business logic. They are deterministic and durable.
- **Activities (`internal/activity`)**: Perform side-effects (DB operations, external API calls) and are called by Workflows.
- **Model (`model`)**: Data models and database interactions.

## 🚀 Getting Started

### Prerequisites

- Go 1.25+
- PostgreSQL
- Temporal Server (running locally or remotely)

### Installation

1.  Clone the repository.
2.  Copy `.env.example` to `.env` and configure your environment variables.
3.  Install dependencies:
    ```bash
    go mod download
    ```

### Running the Application

You can run the application using the Makefile or directly with Go.

```bash
# Run the application (HTTP Server + Temporal Worker)
go run main.go
```

## 📂 Project Structure

```
.
├── internal/
│   ├── activity/       # Temporal Activities (Business logic side-effects)
│   ├── workflow/       # Temporal Workflows (Orchestration)
│   ├── api/            # HTTP Controllers and Routes
│   └── app/            # Application initialization
├── model/              # Database models
├── scripts/            # Utility scripts
├── main.go             # Entry point
└── go.mod              # Dependencies
```

## 📏 Development Standards

### Temporal Workflows & Activities

- **Workflows**: Must be deterministic. No direct database calls, no non-deterministic code (random, time, global state). Use `workflow.Now(ctx)` instead of `time.Now()`.
- **Activities**: Contain all side-effects (DB, API). Must be idempotent where possible.
- **Registration**: All workflows and activities must be registered in the worker initialization.

### API & Controllers

- Controllers should **not** contain business logic.
- Use `parser.ParseQuery` for type-safe request parsing.
- Execute workflows using `ts.Client.ExecuteWorkflow`.
- Return responses using `functions.SendResponse`.

### Logging

- Use Go's standard `log/slog` package for structured logging.
  ```go
  slog.Info("Something happened", "key", "value")
  ```

### Imports

- Use direct package names (e.g., `config`, `service`) instead of prefixed aliases (e.g., `util_config`), unless there is a conflict.
- Use `gitea.tagsamurai.local/BETS-V2/ts-utils/v2` for shared utilities.

## 🧪 Testing

Tests should focus on Temporal Workflows and Activities using the Temporal Test Suite.

```bash
go test ./...
```
