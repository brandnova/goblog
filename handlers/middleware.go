package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"goblog/models"

	"github.com/google/uuid"
)

// -----------------------------------------------------------------------
// Context keys
// -----------------------------------------------------------------------

type contextKey string

const (
	UserContextKey = contextKey("user")
	csrfContextKey = contextKey("csrf")
)

// CurrentUser retrieves the logged-in user from the request context.
// Returns nil if no user is attached (i.e. anonymous / not logged in).
// Django parallel: request.user
func CurrentUser(r *http.Request) *models.User {
	user, _ := r.Context().Value(UserContextKey).(*models.User)
	return user
}

// CSRFToken retrieves the CSRF token for the current request from context.
// Called in render() so {{ .CSRFToken }} is available in every template.
func CSRFToken(r *http.Request) string {
	token, _ := r.Context().Value(csrfContextKey).(string)
	return token
}

// -----------------------------------------------------------------------
// CSRF protection
//
// How it works:
//  1. CSRFMiddleware runs on every request. It reads (or generates) a
//     CSRF token from a cookie and attaches it to the request context.
//  2. render() passes the token to every template as {{ .CSRFToken }}.
//  3. Every HTML form includes a hidden field: <input name="csrf_token" value="{{ .CSRFToken }}">.
//  4. ValidateCSRF() is called at the top of every state-changing POST
//     handler. It compares the submitted field value against the cookie.
//
// This is the "double submit cookie" pattern — the simplest CSRF
// defence that doesn't require server-side token storage.
//
// Django parallel: django.middleware.csrf.CsrfViewMiddleware +
// {% csrf_token %} in every form.
// -----------------------------------------------------------------------

// csrfCookieName is the name of the CSRF cookie.
const csrfCookieName = "csrf_token"

// generateCSRFToken returns a cryptographically random 32-byte hex string.
func generateCSRFToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// CSRFMiddleware reads the CSRF cookie on every request.
// If no cookie exists it generates a new token and sets the cookie.
// Either way, the token is attached to the request context so handlers
// and render() can access it without touching cookies again.
func CSRFMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var token string

		cookie, err := r.Cookie(csrfCookieName)
		if err != nil || cookie.Value == "" {
			// No cookie — generate a fresh token and set it
			token = generateCSRFToken()
			http.SetCookie(w, &http.Cookie{
				Name:     csrfCookieName,
				Value:    token,
				Path:     "/",
				HttpOnly: false, // must be readable by the form field comparison
				SameSite: http.SameSiteLaxMode,
				// No Expires — this is a session cookie, deleted when browser closes
			})
		} else {
			token = cookie.Value
		}

		ctx := context.WithValue(r.Context(), csrfContextKey, token)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// ValidateCSRF compares the submitted csrf_token form field against the
// cookie value. Returns false and writes a 403 if they don't match.
//
// Call this at the top of every state-changing POST handler:
//
//	if !ValidateCSRF(w, r) { return }
//
// HTMX handlers (BookmarkToggle, ReactionToggle) are exempt because
// they don't use HTML forms — they fire programmatic requests from
// within the same origin, which SameSite=Lax already protects.
func ValidateCSRF(w http.ResponseWriter, r *http.Request) bool {
	cookie, err := r.Cookie(csrfCookieName)
	if err != nil || cookie.Value == "" {
		http.Error(w, "Invalid CSRF token", http.StatusForbidden)
		return false
	}

	submitted := r.FormValue("csrf_token")
	if submitted == "" || submitted != cookie.Value {
		http.Error(w, "Invalid CSRF token", http.StatusForbidden)
		return false
	}

	return true
}

// -----------------------------------------------------------------------
// Session management
// -----------------------------------------------------------------------

// CreateSession generates a UUID session token, persists it to the
// sessions table, and writes a session cookie to the HTTP response.
// Django parallel: django.contrib.auth.login(request, user)
func CreateSession(w http.ResponseWriter, userID int) error {
	token := uuid.NewString()
	expires := time.Now().Add(7 * 24 * time.Hour)

	_, err := DB.Exec(
		"INSERT INTO sessions (token, user_id, expires_at) VALUES ($1, $2, $3)",
		token, userID, expires,
	)
	if err != nil {
		return err
	}

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

// DestroySession deletes the session record from the DB and expires the cookie.
// Django parallel: django.contrib.auth.logout(request)
func DestroySession(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session_token")
	if err == nil {
		DB.Exec("DELETE FROM sessions WHERE token = $1", cookie.Value)
	}

	http.SetCookie(w, &http.Cookie{
		Name:    "session_token",
		Value:   "",
		Expires: time.Unix(0, 0),
		Path:    "/",
	})
}

// -----------------------------------------------------------------------
// Middleware chain
// -----------------------------------------------------------------------

// AuthMiddleware reads the session cookie, looks up the user, and
// attaches them to the request context on every request.
// Django parallel: SessionMiddleware + AuthenticationMiddleware
func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("session_token")
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}

		var userID int
		var expires time.Time

		err = DB.QueryRow(
			"SELECT user_id, expires_at FROM sessions WHERE token = $1",
			cookie.Value,
		).Scan(&userID, &expires)

		if err != nil || time.Now().After(expires) {
			next.ServeHTTP(w, r)
			return
		}

		user, err := models.GetUserByID(DB, userID)
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}

		ctx := context.WithValue(r.Context(), UserContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireLogin wraps a handler and redirects unauthenticated users to /login.
// Django parallel: @login_required decorator or LoginRequiredMixin.
func RequireLogin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if CurrentUser(r) == nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		next(w, r)
	}
}

// RequestLogger is a simple logging middleware.
// Uncomment the log.Printf line to enable request logging.
func RequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		duration := time.Since(start)
		user := CurrentUser(r)
		username := "anonymous"
		if user != nil {
			username = user.Username
		}
		_ = duration
		_ = username
		// log.Printf("[%s] %s %s — %s — %v\n",
		//     time.Now().Format("02/Jan/2006 15:04:05"),
		//     r.Method, r.URL.Path, username, duration,
		// )
	})
}