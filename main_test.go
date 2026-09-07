package main

import (
	"html/template"
	"io/fs"
	"testing"
)

func TestEmbeddedTemplatesParse(t *testing.T) {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		t.Fatalf("sub filesystem: %v", err)
	}
	if _, err := template.ParseFS(sub, "templates/*.html"); err != nil {
		t.Fatalf("parse templates: %v", err)
	}
}
