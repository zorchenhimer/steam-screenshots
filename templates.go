package main

import (
    "fmt"
    "html/template"
    "net/http"
)

var templates map[string]*template.Template

type TemplateData struct {
    Title           string
    Header          map[string]string
    Body            []map[string]template.JS
    ImageMetadata   []Metadata
}

func init_templates() error {
    template_list := []string{
        "main",
        "list",
        //"edit",
    }

    templates = make(map[string]*template.Template)
    for _, t := range template_list {
        if temp, err := template.New(t).ParseFiles("templates/base.html", "templates/" + t + ".html"); err != nil {
            return fmt.Errorf("Unable to load %q template: %s", t, err)
        } else {
            templates[t] = temp
        }
    }

    return nil
}

func renderTemplate(w http.ResponseWriter, name string, data *TemplateData) error {
    template, ok := templates[name]
    if !ok {
        fmt.Fprintf(w, "Error rendering template: Invalid template name %q\n", name)
        return fmt.Errorf("Invalid template: %q", name)
    }

    if err := template.ExecuteTemplate(w, "main", data); err != nil {
        fmt.Fprintf(w, "Error rendering template: %s\n", err)
        return fmt.Errorf("Unable to render template: %s", err)
    }

    return nil
}
