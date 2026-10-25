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
├── main.go                   — entry point, routes, middleware chain
├── db.go                     — DB init, schema, migrations, session cleanup
├── go.mod / go.sum
├── .air.toml                 — Air live reload config (go install air@latest)
├── .env.example
├── .gitignore
├── Info.md
├── README.md
├── LICENSE
├── handlers/
│   ├── auth.go               — Register, Login, Logout, Settings handlers
│   ├── helpers.go            — render(), renderWithTitle(), renderPartial(), templateFuncs
│   ├── middleware.go         — CSRFMiddleware, AuthMiddleware, RequireLogin, sessions
│   ├── posts.go              — all post/tag/search/bookmark/reaction handlers
│   └── storage.go            — object storage toggle (local disk vs Leapcell S3)
├── models/
│   ├── post.go               — Post/Tag structs, all queries, AttachTags, pagination
│   └── user.go               — User struct, CRUD, auth helpers
├── static/
│   ├── css/
│   │   ├── goblog.css        — all custom styles (extracted from base.html)
│   │   └── tailwind.css      — compiled Tailwind output
│   └── js/
│       └── htmx.min.js
├── tailwind/                 — Tailwind compilation toolchain (Node, not served)
│   ├── src/styles.css
│   ├── tailwind.config.js
│   ├── postcss.config.js
│   ├── package.json
│   └── package-lock.json
└── templates/
    ├── base.html             — layout, nav, CSRF forms, footer
    ├── index.html            — homepage with live search + infinite scroll
    ├── post.html             — post detail with reactions, bookmarks, flash
    ├── form.html             — create/edit post form with tag counter
    ├── dashboard.html        — author dashboard with metrics
    ├── profile.html          — public profile page
    ├── bookmarks.html        — reading list
    ├── tag.html              — tag-filtered post listing
    ├── settings.html         — profile + password forms
    ├── login.html
    ├── register.html
    ├── 404.html
    └── partials/
        ├── post_list.html    — shared infinite scroll post rows + sentinel
        └── search_results.html — HTMX search results dropdown
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