# GoBlog

A personal blogging platform built with Go — no frameworks, just the standard library plus a handful of well-chosen packages. Built as a learning project to explore Go web development from a Django background.

## Features

- **Auth** — Register, login, logout with bcrypt-hashed passwords and server-side sessions
- **Posts** — Create, edit, delete, and publish posts with Markdown support
- **Tags** — Comma-separated tags with filtered listing pages
- **Draft / Published** — Save posts privately before publishing
- **Live Search** — HTMX-powered search with no page reload
- **Cover Images** — Optional file upload per post
- **Read Time** — Auto-calculated estimated reading time
- **Dashboard** — Personal view of all your posts regardless of status

## Tech Stack

| Concern | Tool |
|---|---|
| Language | Go 1.22+ |
| Router | `net/http` (stdlib, Go 1.22 method routing) |
| Database | SQLite via `github.com/mattn/go-sqlite3` |
| DB Queries | `github.com/jmoiron/sqlx` |
| Password Hashing | `golang.org/x/crypto/bcrypt` |
| Sessions | UUID tokens via `github.com/google/uuid` |
| Markdown | `github.com/gomarkdown/markdown` |
| Templates | `html/template` (stdlib) |
| CSS | Tailwind CSS (compiled locally) |
| Interactivity | HTMX |
| Fonts | Playfair Display, Source Serif 4, JetBrains Mono |

## Project Structure

```
goblog/
├── main.go                  # Entry point, routes, server
├── db.go                    # Database init and schema
├── go.mod
├── go.sum
├── handlers/
│   ├── helpers.go           # render(), renderPartial(), DB init
│   ├── middleware.go        # AuthMiddleware, RequireLogin, sessions
│   ├── auth.go              # Register, Login, Logout handlers
│   └── posts.go             # All post-related handlers
├── models/
│   ├── user.go              # User model, password helpers
│   └── post.go              # Post/Tag models, all queries
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
- GCC (required by `go-sqlite3` which uses CGo) — on Fedora/RHEL: `sudo dnf install gcc`
- Node.js (for Tailwind CSS compilation)

### Installation

```bash
# Clone the repo
git clone https://github.com/yourusername/goblog.git
cd goblog

# Install Go dependencies
go mod tidy

# Install Tailwind dependencies
cd tailwind
npm install
cd ..
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

The database (`blog.db`) is created automatically on first run.

### Building for Production

```bash
go build -o goblog .
./goblog
```

## Environment Notes

- The SQLite database file `blog.db` is created in the project root on startup
- Uploaded cover images are saved to `static/uploads/`
- Sessions expire after 7 days
- There is no default admin user — register via `/register`

## License

MIT