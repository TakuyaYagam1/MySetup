package tui

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/apply"
	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/config"
	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/paths"
)

func TestLoadInitialStatePrefersMachineState(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")
	draftPath := filepath.Join(dir, "draft.json")

	machine := config.Default()
	machine.Host.Hostname = "machine"
	if err := config.Save(statePath, machine); err != nil {
		t.Fatal(err)
	}
	draft := config.Default()
	draft.Host.Hostname = "draft"
	if err := config.Save(draftPath, draft); err != nil {
		t.Fatal(err)
	}

	got, err := loadInitialState(paths.Options{StatePath: statePath, DraftPath: draftPath})
	if err != nil {
		t.Fatal(err)
	}
	if got.Host.Hostname != "machine" {
		t.Fatalf("expected machine state to win, got %q", got.Host.Hostname)
	}
}

func TestLoadInitialStateCarriesOpaqueMachineStateProof(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")
	if err := config.Save(statePath, config.Default()); err != nil {
		t.Fatal(err)
	}

	_, proof, err := loadInitialStateWithProof(paths.Options{StatePath: statePath})
	if err != nil {
		t.Fatal(err)
	}
	if reflect.DeepEqual(proof, apply.LoadedStateProof{}) {
		t.Fatal("machine state load did not retain its ownership proof")
	}
}

func TestLoadInitialStateFallsBackToDraft(t *testing.T) {
	dir := t.TempDir()
	draftPath := filepath.Join(dir, "draft.json")
	draft := config.Default()
	draft.Host.Hostname = "draft"
	if err := config.Save(draftPath, draft); err != nil {
		t.Fatal(err)
	}

	got, err := loadInitialState(paths.Options{
		StatePath: filepath.Join(dir, "missing.json"),
		DraftPath: draftPath,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Host.Hostname != "draft" {
		t.Fatalf("expected draft fallback, got %q", got.Host.Hostname)
	}
}

func TestLoadInitialStateDoesNotTreatDraftAsMachineStateProof(t *testing.T) {
	dir := t.TempDir()
	draftPath := filepath.Join(dir, "draft.json")
	if err := config.Save(draftPath, config.Default()); err != nil {
		t.Fatal(err)
	}

	_, proof, err := loadInitialStateWithProof(paths.Options{
		StatePath: filepath.Join(dir, "missing.json"),
		DraftPath: draftPath,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(proof, apply.LoadedStateProof{}) {
		t.Fatal("draft load unexpectedly received machine-state ownership proof")
	}
}

func TestLoadInitialStateReturnsStateLoadError(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")
	if err := config.Save(statePath, config.Default()); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(statePath, []byte("{ nope\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := loadInitialState(paths.Options{StatePath: statePath}); err == nil {
		t.Fatal("expected invalid state load error")
	}
}
