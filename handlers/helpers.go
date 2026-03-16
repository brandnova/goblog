package handlers

import (
	"html/template"
	"net/http"
	"strings"

	"goblog/models"

	"github.com/jmoiron/sqlx"
)

// DB is the shared database connection.
// It's set once in main.go via handlers.Init(db).
// Django parallel: Django's db connection being globally available via settings.
var DB *sqlx.DB

// Init is called from main.go to give the handlers package access to the DB.
// Django parallel: AppConfig.ready() — a one-time setup step at startup.
func Init(db *sqlx.DB) {
	DB = db
}

// templateFuncs registers custom functions available in all templates.
// You call template.New("").Funcs(templateFuncs) before parsing any template.
//
// Django parallel: registering a custom template filter with @register.filter.
// e.g. {{ post.body|markdown }} in Django → {{ markdown .Body }} in Go.
var templateFuncs = template.FuncMap{
	// "markdown" converts a markdown string to safe HTML.
	// template.HTML tells Go's engine: "trust this string, don't escape it."
	// Without this wrapper, rendered <p> tags would show as &lt;p&gt; in the browser.
	// Django parallel: marking output as safe with mark_safe() or the |safe filter.
	"markdown": func(body string) template.HTML {
        return template.HTML(models.RenderMarkdown(body))
    },
	"upper":   strings.ToUpper,
    "add":     func(a, b int) int { return a + b },
}

// render parses base.html + the given page template, injects the logged-in
// user into the template data, and executes base.html as the entry point.
//
// This is the single reusable render shortcut used by every page handler.
// Django parallel: Django's render(request, 'template.html', context) shortcut.
//
// Usage:
//
//	render(w, r, someData, "templates/index.html")
func render(w http.ResponseWriter, r *http.Request, data any, pageTemplate string) {
	type TemplateData struct {
		Data any
		User any
	}

	td := TemplateData{
		Data: data,
		User: CurrentUser(r),
	}

	// Funcs() must be called before ParseFiles — it registers "markdown"
	// so templates can call {{ markdown .Data.Body }} without errors.
	tmpl, err := template.New("").Funcs(templateFuncs).ParseFiles(
		"templates/base.html",
		pageTemplate,
	)
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
// The markdown FuncMap is included here too in case a partial ever needs it.
//
// Django parallel: HttpResponse(render_to_string('partial.html', context))
//
// Usage:
//
//	renderPartial(w, someData, "templates/partials/search_results.html")
func renderPartial(w http.ResponseWriter, data any, partialTemplate string) {
	tmpl, err := template.New("").Funcs(templateFuncs).ParseFiles(partialTemplate)
	if err != nil {
		http.Error(w, "Template error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, "Render error: "+err.Error(), http.StatusInternalServerError)
	}
}