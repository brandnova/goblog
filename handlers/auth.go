package handlers

import (
	"net/http"

	"goblog/models"
)

// -----------------------------------------------------------------------
// Auth page handlers
// Session management and middleware live in middleware.go.
// -----------------------------------------------------------------------

// RegisterPage renders the blank registration form.
// GET /register
func RegisterPage(w http.ResponseWriter, r *http.Request) {
	render(w, r, nil, "templates/register.html")
}

// Register processes the registration form submission.
// POST /register
// Django parallel: a view calling User.objects.create_user()
func Register(w http.ResponseWriter, r *http.Request) {
	username := r.FormValue("username")
	email    := r.FormValue("email")
	password := r.FormValue("password")
	confirm  := r.FormValue("confirm_password")

	// Re-populate fields on error so the user doesn't retype everything.
	// Django forms do this automatically via form.data.
	formData := map[string]string{
		"Username": username,
		"Email":    email,
	}

	if password != confirm {
		formData["Error"] = "Passwords do not match."
		render(w, r, formData, "templates/register.html")
		return
	}

	if len(password) < 8 {
		formData["Error"] = "Password must be at least 8 characters."
		render(w, r, formData, "templates/register.html")
		return
	}

	if err := models.CreateUser(DB, username, email, password); err != nil {
		// SQLite UNIQUE constraint violation = username or email already taken
		formData["Error"] = "That username or email is already registered."
		render(w, r, formData, "templates/register.html")
		return
	}

	// Redirect to login with a flag so LoginPage can show a success message
	http.Redirect(w, r, "/login?registered=1", http.StatusSeeOther)
}

// LoginPage renders the login form.
// GET /login
func LoginPage(w http.ResponseWriter, r *http.Request) {
	data := map[string]string{}
	if r.URL.Query().Get("registered") == "1" {
		data["Success"] = "Account created successfully. Please log in."
	}
	render(w, r, data, "templates/login.html")
}

// Login processes the login form submission.
// POST /login
// Django parallel: django.contrib.auth.views.LoginView
func Login(w http.ResponseWriter, r *http.Request) {
	email    := r.FormValue("email")
	password := r.FormValue("password")

	user, err := models.GetUserByEmail(DB, email)
	if err != nil || !models.CheckPassword(password, user.PasswordHash) {
		// Deliberately vague — never reveal whether email exists or password was wrong
		render(w, r, map[string]string{
			"Error": "Invalid email or password.",
		}, "templates/login.html")
		return
	}

	if err := CreateSession(w, user.ID); err != nil {
		http.Error(w, "Could not create session.", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// Logout destroys the current session and redirects to home.
// POST /logout — POST prevents logout via a simple link or CSRF attack.
// Django enforces the same thing in its logout view.
func Logout(w http.ResponseWriter, r *http.Request) {
	DestroySession(w, r)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}