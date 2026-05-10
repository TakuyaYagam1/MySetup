package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
)

func TestLiveInputReflectsExternalValueChanges(t *testing.T) {
	value := "Moscow"
	field := newLiveInput().
		Title("Weather location").
		Description("City name used by shell/weather widgets if enabled.").
		Value(&value)

	if view := field.View(); !strings.Contains(view, "Moscow") {
		t.Fatalf("expected initial live value in view, got:\n%s", view)
	}

	value = "Oslo"
	if view := field.View(); !strings.Contains(view, "Oslo") {
		t.Fatalf("expected changed live value in view, got:\n%s", view)
	}
}

func TestTimezoneChangeUpdatesLiveWeatherLocation(t *testing.T) {
	weatherLocation := "Moscow"
	timezone := "Europe/Moscow"
	timezoneField := newBottomFilterSelect().
		Title("Timezone").
		Options(
			huh.NewOption("Europe/Moscow", "Europe/Moscow"),
			huh.NewOption("Europe/Oslo", "Europe/Oslo"),
		).
		Value(&timezone).
		OnChange(func(timeZone string) {
			weatherLocation = weatherLocationFromTimeZone(timeZone)
		})
	weatherField := newLiveInput().
		Title("Weather location").
		Value(&weatherLocation)

	_, _ = timezoneField.Update(tea.KeyMsg{Type: tea.KeyDown})
	if weatherLocation != "Oslo" {
		t.Fatalf("expected timezone change to update weather location, got %q", weatherLocation)
	}
	if view := weatherField.View(); !strings.Contains(view, "Oslo") {
		t.Fatalf("expected live weather field to render updated value, got:\n%s", view)
	}
}

func TestBottomFilterSelectRendersSearchBelowOptions(t *testing.T) {
	value := "Europe/Moscow"
	field := newBottomFilterSelect().
		Title("Timezone").
		Description("Pick a timezone.").
		Options(
			huh.NewOption("Europe/Amsterdam", "Europe/Amsterdam"),
			huh.NewOption("Europe/Berlin", "Europe/Berlin"),
			huh.NewOption("Europe/Moscow", "Europe/Moscow"),
		).
		Height(10).
		Value(&value)

	view := field.View()
	optionIndex := strings.Index(view, "Europe/Moscow")
	searchIndex := strings.Index(view, "Search: press / to filter")
	if optionIndex < 0 || searchIndex < 0 {
		t.Fatalf("expected options and bottom search prompt, got:\n%s", view)
	}
	if searchIndex < optionIndex {
		t.Fatalf("expected search prompt below options, got:\n%s", view)
	}
}

func TestBottomFilterSelectEscapeClearsSearch(t *testing.T) {
	value := "Europe/Moscow"
	field := newBottomFilterSelect().
		Title("Timezone").
		Options(
			huh.NewOption("Europe/Amsterdam", "Europe/Amsterdam"),
			huh.NewOption("Europe/Berlin", "Europe/Berlin"),
			huh.NewOption("Europe/Moscow", "Europe/Moscow"),
		).
		Value(&value)

	_, _ = field.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	_, _ = field.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("Berlin")})
	if !field.GetFiltering() {
		t.Fatal("expected filter mode to be active")
	}
	if len(field.filtered) != 1 {
		t.Fatalf("expected one filtered timezone, got %#v", field.filtered)
	}

	_, _ = field.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if field.GetFiltering() {
		t.Fatal("expected escape to leave filter mode")
	}
	if field.filter != "" {
		t.Fatalf("expected escape to clear filter, got %q", field.filter)
	}
	if len(field.filtered) != len(field.options) {
		t.Fatalf("expected escape to restore all options, got %#v", field.filtered)
	}
}
