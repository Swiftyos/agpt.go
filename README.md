# Go Chatbot API

A Go backend API for a ChatGPT-like chatbot application, designed to work with Next.js frontends using the Vercel AI SDK.

## Features

- **Authentication**: Username/password and Google OAuth login
- **JWT Tokens**: Access and refresh token flow
- **Chat Sessions**: Create, manage, and delete chat sessions
- **Message History**: Persistent chat history stored in PostgreSQL
- **Streaming**: AI SDK Data Stream Protocol for real-time responses
- **PostgreSQL**: All data including caching stored in PostgreSQL
- **SQLC**: Type-safe database queries

## Tech Stack

- **Go 1.22+**
- **Chi Router**: Lightweight HTTP router
- **pgx**: PostgreSQL driver with connection pooling
- **SQLC**: Generate type-safe Go code from SQL
- **JWT**: Token-based authentication
- **OpenAI API**: LLM integration (GPT-4o by default)

## Project Structure

```
.
├── cmd/
│   └── api/
│       └── main.go           # Application entry point
├── internal/
│   ├── config/
│   │   └── config.go         # Configuration management
│   ├── database/
│   │   ├── db.go             # Database connection
│   │   └── queries/          # SQL queries for SQLC
│   ├── handlers/
│   │   ├── auth.go           # Auth endpoints
│   │   ├── chat.go           # Chat endpoints
│   │   └── sessions.go       # Session endpoints
│   ├── middleware/
│   │   ├── auth.go           # JWT authentication
│   │   └── cors.go           # CORS configuration
│   ├── services/
│   │   ├── auth.go           # Auth business logic
│   │   ├── chat.go           # Chat business logic
│   │   └── llm.go            # LLM integration
│   └── streaming/
│       └── protocol.go       # AI SDK stream protocol
├── migrations/
│   └── 001_initial.sql       # Database schema
├── .env.example              # Environment variables template
├── Makefile                  # Build and development commands
├── sqlc.yaml                 # SQLC configuration
└── go.mod                    # Go module definition
```

## Quick Start

### Prerequisites

- Go 1.22+
- PostgreSQL 14+
- OpenAI API key

### Installation

1. Clone the repository:
```bash
git clone <repository-url>
cd agpt.go
```

2. Install dependencies:
```bash
make deps
```

3. Install development tools:
```bash
make tools
```

4. Copy environment file and configure:
```bash
cp .env.example .env
# Edit .env with your configuration
```

5. Create database and run migrations:
```bash
make db-create
make db-migrate
```

6. Generate SQLC code:
```bash
make sqlc
```

7. Run the server:
```bash
make run
```

## API Endpoints

### Authentication

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/auth/register` | Register new user |
| POST | `/api/v1/auth/login` | Login with email/password |
| POST | `/api/v1/auth/refresh` | Refresh access token |
| POST | `/api/v1/auth/logout` | Logout (revoke refresh token) |
| GET | `/api/v1/auth/google` | Initiate Google OAuth |
| GET | `/api/v1/auth/google/callback` | Google OAuth callback |

### User

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/me` | Get current user profile |

### Chat Sessions

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/sessions` | Create new chat session |
| GET | `/api/v1/sessions` | List all sessions |
| GET | `/api/v1/sessions/:id` | Get session details |
| PATCH | `/api/v1/sessions/:id` | Update session |
| DELETE | `/api/v1/sessions/:id` | Delete session |

### Messages

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/sessions/:id/messages` | Get session messages |
| POST | `/api/v1/sessions/:id/messages` | Send message (non-streaming) |
| POST | `/api/v1/sessions/:id/messages/stream` | Send message (streaming) |

## Streaming Protocol

The streaming endpoint implements the [Vercel AI SDK Data Stream Protocol](https://ai-sdk.dev/docs/ai-sdk-ui/stream-protocol):

```
0:text chunk\n        # Text part
f:{"messageId":"..."}\n  # Start message
d:{"finishReason":"stop"}\n  # Finish with usage
```

### Next.js Integration

```typescript
import { useChat } from 'ai/react';

export function Chat() {
  const { messages, input, handleInputChange, handleSubmit } = useChat({
    api: 'http://localhost:8080/api/v1/sessions/{sessionId}/messages/stream',
    headers: {
      Authorization: `Bearer ${accessToken}`,
    },
  });

  return (
    <div>
      {messages.map((m) => (
        <div key={m.id}>{m.content}</div>
      ))}
      <form onSubmit={handleSubmit}>
        <input value={input} onChange={handleInputChange} />
      </form>
    </div>
  );
}
```

## Development

```bash
# Run with hot reload
make dev

# Run tests
make test

# Run linter
make lint

# Generate SQLC code after modifying queries
make sqlc
```

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `PORT` | Server port | `8080` |
| `ENVIRONMENT` | Environment mode | `development` |
| `CORS_ORIGINS` | Allowed CORS origins | `http://localhost:3000` |
| `DB_HOST` | PostgreSQL host | `localhost` |
| `DB_PORT` | PostgreSQL port | `5432` |
| `DB_USER` | PostgreSQL user | `postgres` |
| `DB_PASSWORD` | PostgreSQL password | `postgres` |
| `DB_NAME` | Database name | `chatbot` |
| `JWT_SECRET` | JWT signing secret | (required) |
| `OPENAI_API_KEY` | OpenAI API key | (required) |
| `OPENAI_MODEL` | OpenAI model | `gpt-4o` |
| `GOOGLE_CLIENT_ID` | Google OAuth client ID | (optional) |
| `GOOGLE_CLIENT_SECRET` | Google OAuth secret | (optional) |

## License

MIT
