package handlers

import (
	"html/template"
	"net/http"
	"path/filepath"
	"strings"

	"goblog/models"

	"github.com/jmoiron/sqlx"
)

// DB is the shared database connection, set once via Init().
var DB *sqlx.DB

func Init(db *sqlx.DB) {
	DB = db
}

// templateFuncs registers custom functions available in all templates.
// Django parallel: custom template filters registered with @register.filter
var templateFuncs = template.FuncMap{
	// markdown renders a markdown string as safe HTML.
	// template.HTML tells Go not to escape the output.
	// Django parallel: mark_safe() / |safe filter
	"markdown": func(body string) template.HTML {
		return template.HTML(models.RenderMarkdown(body))
	},
	// upper converts a string to uppercase — used for avatar initials.
	"upper": strings.ToUpper,
	// add performs integer addition — used in dashboard post count template logic.
	// Go templates have no arithmetic operators, so we register this as a func.
	"add": func(a, b int) int { return a + b },
}

// render parses base.html + the given page template (+ any extra partials)
// and executes base.html as the entry point.
// Django parallel: Django's render(request, 'template.html', context) shortcut.
func render(w http.ResponseWriter, r *http.Request, data any, pageTemplate string, extras ...string) {
	type TemplateData struct {
		Data any
		User any
	}

	td := TemplateData{
		Data: data,
		User: CurrentUser(r),
	}

	files := append([]string{"templates/base.html", pageTemplate}, extras...)
	tmpl, err := template.New("").Funcs(templateFuncs).ParseFiles(files...)
	if err != nil {
		http.Error(w, "Template error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if err := tmpl.ExecuteTemplate(w, "base.html", td); err != nil {
		http.Error(w, "Render error: "+err.Error(), http.StatusInternalServerError)
	}
}

// renderPartial renders a template WITHOUT base.html.
// Used for HTMX endpoints that return HTML fragments, not full pages.
//
// The root cause of the previous search bug was here:
// template.New("").ParseFiles(f) creates a set where the only named
// template is the file's base name (e.g. "search_results.html"), not "".
// Calling tmpl.Execute() was executing the unnamed "" template which
// doesn't exist, silently rendering nothing.
// Fix: execute by the file's actual base name.
func renderPartial(w http.ResponseWriter, data any, partialTemplate string) {
	name := filepath.Base(partialTemplate)
	tmpl, err := template.New(name).Funcs(templateFuncs).ParseFiles(partialTemplate)
	if err != nil {
		http.Error(w, "Template error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if err := tmpl.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, "Render error: "+err.Error(), http.StatusInternalServerError)
	}
}