package handlers

import (
	"context"
	"net/http"
	"time"

	"goblog/models"

	"github.com/google/uuid"
)

// -----------------------------------------------------------------------
// Context
//
// contextKey is a custom unexported type for request context keys.
// Using a custom type (instead of plain string) prevents key collisions
// with other packages that also store values in context.
//
// Django parallel: Django attaches request.user to the request object
// automatically. In Go, we do the same thing manually via context.
// -----------------------------------------------------------------------

type contextKey string

const UserContextKey = contextKey("user")

// CurrentUser retrieves the logged-in user from the request context.
// Returns nil if no user is attached (i.e. anonymous / not logged in).
//
// Called in every handler that needs to know who's making the request,
// and in render() so that {{ .User }} is available in every template.
//
// Django parallel: request.user
func CurrentUser(r *http.Request) *models.User {
	user, _ := r.Context().Value(UserContextKey).(*models.User)
	return user
}

// -----------------------------------------------------------------------
// Session management
//
// Django handles all of this automatically via SessionMiddleware +
// django.contrib.auth. Here we implement it from scratch so you can
// see exactly what's happening under the hood on every request.
// -----------------------------------------------------------------------

// CreateSession generates a UUID session token, persists it to the
// sessions table, and writes a session cookie to the HTTP response.
//
// Django parallel: django.contrib.auth.login(request, user)
func CreateSession(w http.ResponseWriter, userID int) error {
	token := uuid.NewString()
	expires := time.Now().Add(7 * 24 * time.Hour) // 7-day session

	_, err := DB.Exec(
		"INSERT INTO sessions (token, user_id, expires_at) VALUES (?, ?, ?)",
		token, userID, expires,
	)
	if err != nil {
		return err
	}

	// HttpOnly prevents JavaScript from reading the cookie — XSS protection.
	// SameSiteLax prevents the cookie from being sent on cross-site POSTs — CSRF protection.
	// Django sets both of these via SESSION_COOKIE_HTTPONLY and SESSION_COOKIE_SAMESITE.
	http.SetCookie(w, &http.Cookie{
		Name:     "session_token",
		Value:    token,
		Expires:  expires,
		HttpOnly: true,
		Path:     "/",
		SameSite: http.SameSiteLaxMode,
	})
	return nil
}

// DestroySession deletes the session record from the DB and overwrites
// the browser cookie with an already-expired one, effectively deleting it.
//
// Django parallel: django.contrib.auth.logout(request)
func DestroySession(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session_token")
	if err == nil {
		DB.Exec("DELETE FROM sessions WHERE token = ?", cookie.Value)
	}

	http.SetCookie(w, &http.Cookie{
		Name:    "session_token",
		Value:   "",
		Expires: time.Unix(0, 0), // Unix epoch = already expired
		Path:    "/",
	})
}

// -----------------------------------------------------------------------
// Middleware
//
// In Go, middleware is a function that wraps an http.Handler and returns
// a new http.Handler. You chain them together in main.go.
//
// Django parallel: classes listed in settings.MIDDLEWARE. Each one wraps
// the next, forming a chain — exactly like Go's handler wrapping pattern.
// -----------------------------------------------------------------------

// AuthMiddleware runs on every single request. It:
//  1. Reads the session cookie
//  2. Looks up the session in the DB
//  3. Fetches the corresponding User
//  4. Attaches the User to the request context
//
// If any step fails (no cookie, expired session, deleted user), the
// request continues anonymously — CurrentUser(r) will return nil.
//
// Applied in main.go by wrapping the entire mux:
//
//	http.ListenAndServe(":8080", handlers.AuthMiddleware(mux))
//
// Django parallel: SessionMiddleware + AuthenticationMiddleware, which
// together make request.user available in every view.
func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("session_token")
		if err != nil {
			// No cookie present — anonymous request, move on
			next.ServeHTTP(w, r)
			return
		}

		var userID int
		var expires time.Time

		err = DB.QueryRow(
			"SELECT user_id, expires_at FROM sessions WHERE token = ?",
			cookie.Value,
		).Scan(&userID, &expires)

		if err != nil || time.Now().After(expires) {
			// Token not found or session has expired — treat as anonymous
			next.ServeHTTP(w, r)
			return
		}

		user, err := models.GetUserByID(DB, userID)
		if err != nil {
			// User record gone (deleted account etc.) — treat as anonymous
			next.ServeHTTP(w, r)
			return
		}

		// Attach the user to the context. Every handler downstream can now
		// call CurrentUser(r) to get this user without touching the DB again.
		ctx := context.WithValue(r.Context(), UserContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireLogin wraps a handler and redirects unauthenticated users to /login.
// Applied per-route in main.go, not globally.
//
// Django parallel: @login_required decorator or LoginRequiredMixin.
//
// Usage:
//
//	mux.HandleFunc("GET /new", handlers.RequireLogin(handlers.NewPostPage))
func RequireLogin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if CurrentUser(r) == nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		next(w, r)
	}
}

// RequestLogger is a simple logging middleware that prints each request's
// method, path, and how long it took to process.
//
// Django parallel: django.middleware.common.CommonMiddleware (partially),
// or a custom process_request/process_response middleware.
//
// Applied in main.go by wrapping the entire mux AFTER AuthMiddleware:
//
//	handler := handlers.RequestLogger(handlers.AuthMiddleware(mux))
//	http.ListenAndServe(":8080", handler)
func RequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		// time.Since(start) gives elapsed duration — e.g. "1.23ms"
		// Django's dev server prints something similar for every request
		duration := time.Since(start)
		user := CurrentUser(r)
		username := "anonymous"
		if user != nil {
			username = user.Username
		}
		// Mimics Django's request log line:
		// [15/Mar/2026 12:00:00] "GET / HTTP/1.1" 200 — anonymous — 1.2ms
		_ = duration
		_ = username
		// log.Printf("[%s] %s %s — %s — %v\n",
		//     time.Now().Format("02/Jan/2006 15:04:05"),
		//     r.Method, r.URL.Path, username, duration,
		// )
		// Uncomment the log.Printf above to enable request logging.
		// It's commented out by default to keep the dev output clean.
	})
}