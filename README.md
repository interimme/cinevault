# CineVault API

CineVault is a movie database REST API built using Go that allows users to create, read, update, and delete movie records. The API provides endpoints for user authentication, movie management, and more.

## Table of Contents

- [Project Structure](#project-structure)
- [Features](#features)
- [Installation](#installation)
- [Configuration](#configuration)
- [Usage](#usage)
- [API Endpoints](#api-endpoints)
- [Database Migrations](#database-migrations)
- [Production Setup](#production-setup)
- [Contributing](#contributing)

## Project Structure

```plaintext
CineVault/
│
├── bin/                  # Compiled binaries
│   └── linux_amd64/
│       └── api
├── cmd/                  # Main application code
│   └── api/
│       ├── context.go
│       ├── errors.go
│       ├── healthcheck.go
│       ├── helpers.go
│       ├── main.go
│       ├── middleware.go
│       ├── movies.go
│       ├── routes.go
│       ├── server.go
│       ├── tokens.go
│       └── users.go
├── examples/             # Example applications
│   └── cors/
│       ├── preflight/
│       │   └── main.go
│       └── simple/
│           └── main.go
├── internal/             # Internal application packages
│   ├── data/             # Database models and data validation
│   │   ├── filters.go
│   │   ├── models.go
│   │   ├── movies.go
│   │   ├── permissions.go
│   │   ├── runtime.go
│   │   ├── tokens.go
│   │   └── users.go
│   ├── jsonlog/          # JSON logging package
│   │   └── jsonlog.go
│   └── mailer/           # Email sending package
│       ├── mailer.go
│       └── templates/    # Email templates
│           ├── token_activation.tmpl
│           ├── token_password_reset.tmpl
│           └── user_welcome.tmpl
├── migrations/           # Database migration scripts
│   ├── 000001_create_movies_table.up.sql
│   ├── 000001_create_movies_table.down.sql
│   ├── ... (other migration files)
│   └── 000006_add_permissions.up.sql
├── remote/               # Remote deployment configuration
│   └── production/
│       ├── api.service
│       └── Caddyfile
├── setup/                # Setup scripts
│   └── 01.sh
├── tmp/                  # Temporary files
│   └── largefile.json
├── vendor/               # Vendor directory for dependencies
├── .envrc                # Environment variables file
├── .gitignore            # Git ignore file
├── go.mod                # Go module file
├── go.sum                # Go module dependencies
└── Makefile              # Makefile for building the project
```

## Features

- **RESTful API** for managing movie records.
- **User authentication** with JWT tokens.
- **Rate limiting** to control the number of requests.
- **CORS support** for cross-origin requests.
- **Email notifications** for user activation and password resets.
- **Database migration** scripts for easy setup and updates.

## Installation

To set up the CineVault API, follow these steps:

1. **Clone the repository:**
   ```bash
   git clone https://github.com/yourusername/cinevault.git
   cd cinevault
   ```

2. **Install dependencies:**
   ```bash
   go mod download
   ```

3. **Compile the API:**
   ```bash
   make build
   ```

4. **Run the API server:**
   ```bash
   ./bin/linux_amd64/api
   ```

## Configuration

CineVault uses environment variables for configuration. You can set these variables in a `.envrc` file or export them directly in your shell.

Key environment variables:

- `CINEVAULT_DB_DSN`: Data source name for PostgreSQL database.
- `JWT_SECRET`: Secret key for signing JWT tokens.
- `SMTP_HOST`, `SMTP_PORT`, `SMTP_USERNAME`, `SMTP_PASSWORD`, `SMTP_SENDER`: Configuration for email server.

## Usage

Start the API server using the `make run` command or by directly executing the compiled binary:

```bash
make run
```

The server will start on `localhost:4000` by default.

## API Endpoints

- **Health Check:** `GET /v1/healthcheck`
- **Movies:**
  - `GET /v1/movies`
  - `POST /v1/movies`
  - `GET /v1/movies/:id`
  - `PATCH /v1/movies/:id`
  - `DELETE /v1/movies/:id`
- **Users:**
  - `POST /v1/users` - Register a new user
  - `PUT /v1/users/activated` - Activate a user account
  - `PUT /v1/users/password` - Reset user password
- **Tokens:**
  - `POST /v1/tokens/authentication` - Obtain authentication token
  - `POST /v1/tokens/activation` - Request activation token
  - `POST /v1/tokens/password-reset` - Request password reset token

## Database Migrations

To apply database migrations, use a tool like `migrate`:

```bash
migrate -path ./migrations -database 'postgres://user:pass@localhost:5432/cinevault?sslmode=disable' up
```

## Production Setup

For production, use the provided `api.service` and `Caddyfile` for setting up the API server and reverse proxy.

1. **Deploy API using systemd:**
   - Copy `api.service` to `/etc/systemd/system/`
   - Start and enable the service:
     ```bash
     sudo systemctl start api
     sudo systemctl enable api
     ```

2. **Configure Caddy for HTTPS:**
   - Use the `Caddyfile` to set up your domain and SSL.

3. **Run setup script:**
   - Execute the `01.sh` script for initial server setup.

## Contributing

Feel free to contribute! You can open an issue to start a discussion or submit a pull request with your suggested changes.
