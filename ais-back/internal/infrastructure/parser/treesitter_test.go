package parser_test

import (
	"context"
	"testing"

	domainrepo "github.com/bakdaulet/ais/ais-back/internal/domain/repository"
	parserinfra "github.com/bakdaulet/ais/ais-back/internal/infrastructure/parser"
	"github.com/bakdaulet/ais/ais-back/pkg/logger"
)

func newTestParser(t *testing.T) *parserinfra.TreeSitterParser {
	t.Helper()
	log, err := logger.New("error", "console")
	if err != nil {
		t.Fatalf("logger.New: %v", err)
	}
	p, err := parserinfra.NewTreeSitterParser(log)
	if err != nil {
		t.Fatalf("NewTreeSitterParser: %v", err)
	}
	return p
}

// ---------------------------------------------------------------------------
// TypeScript – imports
// ---------------------------------------------------------------------------

func TestParseTypeScript_Imports(t *testing.T) {
	p := newTestParser(t)
	ctx := context.Background()

	src := []byte(`
import { Component, OnInit } from '@angular/core';
import { ApiService } from '../core/services/api.service';
import type { GraphNode } from '../core/models';
import DefaultExport from './some-module';
`)
	result, err := p.ParseFile(ctx, "test.ts", src, domainrepo.LanguageTypeScript)
	if err != nil {
		t.Fatalf("ParseFile error: %v", err)
	}

	if len(result.Imports) != 4 {
		t.Errorf("expected 4 imports, got %d: %+v", len(result.Imports), result.Imports)
	}

	imp := result.Imports[0]
	if imp.Source != "@angular/core" {
		t.Errorf("import[0].Source = %q; want %q", imp.Source, "@angular/core")
	}
	if imp.IsRelative {
		t.Errorf("import[0].IsRelative = true; want false for @angular/core")
	}

	imp2 := result.Imports[1]
	if !imp2.IsRelative {
		t.Errorf("import[1].IsRelative = false; want true for ../core/services/api.service")
	}
	if imp2.Source != "../core/services/api.service" {
		t.Errorf("import[1].Source = %q; want ../core/services/api.service", imp2.Source)
	}
}

// ---------------------------------------------------------------------------
// TypeScript – functions
// ---------------------------------------------------------------------------

func TestParseTypeScript_Functions(t *testing.T) {
	p := newTestParser(t)
	ctx := context.Background()

	src := []byte(`
export function add(a: number, b: number): number {
  return a + b;
}

export const multiply = (a: number, b: number): number => a * b;

async function fetchData(url: string): Promise<Response> {
  return fetch(url);
}
`)
	result, err := p.ParseFile(ctx, "math.ts", src, domainrepo.LanguageTypeScript)
	if err != nil {
		t.Fatalf("ParseFile error: %v", err)
	}

	if len(result.Functions) < 2 {
		t.Errorf("expected at least 2 functions, got %d", len(result.Functions))
	}

	var foundAdd, foundFetch bool
	for _, fn := range result.Functions {
		if fn.Name == "add" {
			foundAdd = true
			if fn.StartLine == 0 {
				t.Error("add.StartLine should not be 0")
			}
		}
		if fn.Name == "fetchData" {
			foundFetch = true
			if !fn.IsAsync {
				t.Error("fetchData should be marked async")
			}
		}
	}
	if !foundAdd {
		t.Error("function 'add' not found in parse result")
	}
	if !foundFetch {
		t.Error("function 'fetchData' not found in parse result")
	}
}

// ---------------------------------------------------------------------------
// TypeScript – classes
// ---------------------------------------------------------------------------

func TestParseTypeScript_Classes(t *testing.T) {
	p := newTestParser(t)
	ctx := context.Background()

	src := []byte(`
class Animal {
  name: string;
  constructor(name: string) { this.name = name; }
  speak(): void {}
}

class Dog extends Animal {
  breed: string;
}

interface Serializable {
  serialize(): string;
}
`)
	result, err := p.ParseFile(ctx, "animals.ts", src, domainrepo.LanguageTypeScript)
	if err != nil {
		t.Fatalf("ParseFile error: %v", err)
	}

	if len(result.Classes) < 2 {
		t.Errorf("expected at least 2 classes, got %d", len(result.Classes))
	}

	var foundDog bool
	for _, cls := range result.Classes {
		if cls.Name == "Dog" {
			foundDog = true
			if cls.Extends != "Animal" {
				t.Errorf("Dog.Extends = %q; want Animal", cls.Extends)
			}
		}
	}
	if !foundDog {
		t.Error("class Dog not found")
	}
}

// ---------------------------------------------------------------------------
// Go – functions and methods
// ---------------------------------------------------------------------------

func TestParseGo_Functions(t *testing.T) {
	p := newTestParser(t)
	ctx := context.Background()

	src := []byte(`
package main

import (
	"fmt"
	"context"
)

func main() {
	fmt.Println("hello")
}

func (s *Service) DoWork(ctx context.Context) error {
	return nil
}

type Service struct {
	name string
}
`)
	result, err := p.ParseFile(ctx, "main.go", src, domainrepo.LanguageGo)
	if err != nil {
		t.Fatalf("ParseFile error: %v", err)
	}

	if len(result.Imports) < 2 {
		t.Errorf("expected at least 2 imports, got %d", len(result.Imports))
	}

	var foundMain, foundDoWork bool
	for _, fn := range result.Functions {
		if fn.Name == "main" {
			foundMain = true
		}
		if fn.Name == "DoWork" {
			foundDoWork = true
			if !fn.IsMethod {
				t.Error("DoWork should be IsMethod=true")
			}
		}
	}
	if !foundMain {
		t.Error("function 'main' not found")
	}
	if !foundDoWork {
		t.Error("method 'DoWork' not found")
	}
}

// ---------------------------------------------------------------------------
// DetectLanguage – takes a file extension string, returns (Language, bool)
// ---------------------------------------------------------------------------

func TestDetectLanguage(t *testing.T) {
	p := newTestParser(t)

	tests := []struct {
		ext      string
		wantLang domainrepo.Language
		wantOK   bool
	}{
		{".ts", domainrepo.LanguageTypeScript, true},
		{".tsx", domainrepo.LanguageTypeScript, true},
		{".go", domainrepo.LanguageGo, true},
		{".js", domainrepo.LanguageJavaScript, true},
		{".jsx", domainrepo.LanguageJavaScript, true},
		{".rb", domainrepo.LanguageUnknown, false},
		{"", domainrepo.LanguageUnknown, false},
	}

	for _, tc := range tests {
		t.Run(tc.ext, func(t *testing.T) {
			lang, ok := p.DetectLanguage(tc.ext)
			if ok != tc.wantOK {
				t.Errorf("DetectLanguage(%q) ok=%v; want %v", tc.ext, ok, tc.wantOK)
			}
			if tc.wantOK && lang != tc.wantLang {
				t.Errorf("DetectLanguage(%q) lang=%q; want %q", tc.ext, lang, tc.wantLang)
			}
		})
	}
}