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
func Register(w http.ResponseWriter, r *http.Request) {
	username := r.FormValue("username")
	email    := r.FormValue("email")
	password := r.FormValue("password")
	confirm  := r.FormValue("confirm_password")

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
		formData["Error"] = "That username or email is already registered."
		render(w, r, formData, "templates/register.html")
		return
	}

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
func Login(w http.ResponseWriter, r *http.Request) {
	email    := r.FormValue("email")
	password := r.FormValue("password")

	user, err := models.GetUserByEmail(DB, email)
	if err != nil || !models.CheckPassword(password, user.PasswordHash) {
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
// POST /logout
func Logout(w http.ResponseWriter, r *http.Request) {
	DestroySession(w, r)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// -----------------------------------------------------------------------
// Settings — profile editing
// -----------------------------------------------------------------------

// SettingsPage renders the settings page pre-filled with current values.
// GET /settings
func SettingsPage(w http.ResponseWriter, r *http.Request) {
	data := map[string]any{}
	// ?saved=profile or ?saved=password — drives which success banner shows
	switch r.URL.Query().Get("saved") {
	case "profile":
		data["ProfileSaved"] = true
	case "password":
		data["PasswordSaved"] = true
	}
	render(w, r, data, "templates/settings.html")
}

// UpdateProfile handles the profile info form (POST /settings/profile).
// Updates first name, last name, bio, and email only.
// Completely independent of the password form — no password fields touched.
// Django parallel: UserChangeForm.save()
func UpdateProfile(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r)

	firstName := r.FormValue("first_name")
	lastName  := r.FormValue("last_name")
	bio       := r.FormValue("bio")
	email     := r.FormValue("email")

	if email == "" {
		render(w, r, map[string]any{
			"ProfileError": "Email address cannot be empty.",
			"FirstName":    firstName,
			"LastName":     lastName,
			"Bio":          bio,
			"Email":        email,
		}, "templates/settings.html")
		return
	}

	if err := models.UpdateProfile(DB, user.ID, firstName, lastName, bio, email); err != nil {
		render(w, r, map[string]any{
			"ProfileError": err.Error(),
			"FirstName":    firstName,
			"LastName":     lastName,
			"Bio":          bio,
			"Email":        email,
		}, "templates/settings.html")
		return
	}

	http.Redirect(w, r, "/settings?saved=profile", http.StatusSeeOther)
}

// UpdatePassword handles the password change form (POST /settings/password).
// Only touches the password — profile fields are never read here.
// Django parallel: PasswordChangeForm.save()
func UpdatePassword(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r)

	currentPassword := r.FormValue("current_password")
	newPassword     := r.FormValue("new_password")
	confirmPassword := r.FormValue("confirm_password")

	fail := func(msg string) {
		render(w, r, map[string]any{
			"PasswordError": msg,
		}, "templates/settings.html")
	}

	if currentPassword == "" {
		fail("Please enter your current password.")
		return
	}
	if len(newPassword) < 8 {
		fail("New password must be at least 8 characters.")
		return
	}
	if newPassword != confirmPassword {
		fail("New passwords do not match.")
		return
	}

	if err := models.UpdatePassword(DB, user.ID, currentPassword, newPassword); err != nil {
		fail(err.Error())
		return
	}

	http.Redirect(w, r, "/settings?saved=password", http.StatusSeeOther)
}