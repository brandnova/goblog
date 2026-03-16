# GoBlog

A personal blogging platform built with Go — no frameworks, just the standard
library plus a handful of well-chosen packages. Built as a learning project to
explore Go web development from a Django background.

## Features

- **Auth** — Register, login, logout with bcrypt-hashed passwords and server-side sessions
- **Posts** — Create, edit, delete, and publish posts with Markdown support
- **Tags** — Comma-separated tags with filtered listing pages
- **Draft / Published** — Save posts privately before publishing
- **Live Search** — HTMX-powered search with no page reload
- **Cover Images** — Optional file upload per post (local dev only — see note below)
- **Read Time** — Auto-calculated estimated reading time
- **Dashboard** — Personal view of all your posts regardless of status

## Tech Stack

| Concern | Tool |
|---|---|
| Language | Go 1.22+ |
| Router | `net/http` (stdlib, Go 1.22 method routing) |
| Database | PostgreSQL via `github.com/lib/pq` |
| DB Queries | `github.com/jmoiron/sqlx` |
| Password Hashing | `golang.org/x/crypto/bcrypt` |
| Sessions | UUID tokens via `github.com/google/uuid` |
| Markdown | `github.com/gomarkdown/markdown` |
| Env Variables | `github.com/joho/godotenv` |
| Templates | `html/template` (stdlib) |
| CSS | Tailwind CSS (compiled locally) |
| Interactivity | HTMX |
| Fonts | Playfair Display, Source Serif 4, JetBrains Mono |
| Deployment | Leapcell |

## Project Structure

```
goblog/
├── main.go                  # Entry point, routes, server
├── db.go                    # Database init and schema
├── go.mod
├── go.sum
├── .env                     # Local environment variables (git-ignored)
├── handlers/
│   ├── helpers.go           # render(), renderPartial(), DB init, template funcs
│   ├── middleware.go        # AuthMiddleware, RequireLogin, session management
│   ├── auth.go              # Register, Login, Logout handlers
│   └── posts.go             # All post-related handlers
├── models/
│   ├── user.go              # User model, password helpers
│   └── post.go              # Post/Tag models, all queries, markdown helpers
├── templates/
│   ├── base.html            # Base layout (masthead, nav, footer)
│   ├── index.html           # Homepage — post list + live search
│   ├── post.html            # Single post detail
│   ├── form.html            # Create / edit post form
│   ├── dashboard.html       # Author's post management page
│   ├── tag.html             # Posts filtered by tag
│   ├── login.html
│   ├── register.html
│   └── partials/
│       └── search_results.html   # HTMX fragment
├── static/
│   ├── css/
│   │   └── tailwind.css     # Compiled by Tailwind CLI
│   ├── js/
│   │   └── htmx.min.js
│   └── uploads/             # Cover image uploads (git-ignored)
└── tailwind/
    ├── src/styles.css
    ├── tailwind.config.js
    └── package.json
```

## Getting Started

### Prerequisites

- Go 1.22 or higher
- PostgreSQL running locally
- Node.js (for Tailwind CSS compilation)

### Installation

```bash
# Clone the repo
git clone https://github.com/brandnova/goblog.git
cd goblog

# Install Go dependencies
go mod tidy

# Install Tailwind dependencies
cd tailwind
npm install
cd ..
```

### Environment Variables

Create a `.env` file in the project root:

```
DATABASE_URL=postgres://postgres:yourpassword@localhost:5432/goblog?sslmode=disable
PORT=8080
```

Use any online PostgreSQL database you know. Leapcell Database service is a good example example.
For local PostgreSQL setup on Fedora/RHEL:

```bash
sudo dnf install postgresql postgresql-server
sudo postgresql-setup --initdb
sudo systemctl start postgresql
sudo -u postgres psql -c "CREATE DATABASE goblog;"
```

### Running in Development

Open two terminals:

**Terminal 1 — Tailwind watcher:**
```bash
cd tailwind
npx tailwindcss -i ./src/styles.css -o ../static/css/tailwind.css --watch
```

**Terminal 2 — Go server:**
```bash
go run .
```

Visit `http://localhost:8080`

The database schema is created automatically on first run via `initDB()`.
There is no default user — register via `/register`.

### Building for Production

```bash
go build -o goblog .
./goblog
```

## Notes

- Sessions expire after 7 days
- Cover image uploads work in local development but are not supported on
  Leapcell's serverless plan — the filesystem is read-only in that environment.
  Cloudflare R2 is the recommended path for production image storage.
- See `Info.md` for the full deployment guide and feature roadmap

## License

MIT