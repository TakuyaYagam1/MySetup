package tui

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/charmbracelet/huh"

	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/apply"
	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/cleanup"
	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/config"
	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/doctor"
	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/paths"
)

var errBackToSections = errors.New("back to section selector")

type Options struct {
	Paths    paths.Options
	DryRun   bool
	Layout   apply.Layout
	LockMode apply.LockMode
}

type session struct {
	state    config.State
	secrets  config.Secrets
	paths    paths.Options
	dryRun   bool
	layout   apply.Layout
	lockMode apply.LockMode
	selected int
}

var sections = []string{
	"General",
	"User",
	"Passwords",
	"Region",
	"Display",
	"Packages",
	"Services",
	"Dots",
	"Cleanup",
	"Doctor",
	"Apply",
	"Quit",
}

func Run(ctx context.Context, opts Options) error {
	state, err := loadInitialState(opts.Paths)
	if err != nil {
		return err
	}
	s := &session{
		state:    state,
		paths:    opts.Paths,
		dryRun:   opts.DryRun,
		layout:   opts.Layout,
		lockMode: opts.LockMode,
	}

	for {
		selected, cursor, err := chooseSection(s.state, s.secrets, detectExistingSecrets(s.paths), s.selected)
		if err != nil {
			return err
		}
		s.selected = cursor
		if selected == "Quit" {
			return nil
		}
		previousState := s.state
		previousSecrets := s.secrets
		sectionSaved, err := handleSectionResult(s, previousState, previousSecrets, runSection(ctx, s, selected))
		if err != nil {
			if errors.Is(err, huh.ErrUserAborted) {
				return nil
			}
			if err := showSectionError(err); err != nil {
				return err
			}
			continue
		}
		if sectionSaved && isDirtySection(selected) {
			if err := config.Save(s.paths.DraftPath, s.state); err != nil {
				return fmt.Errorf("save draft state: %w", err)
			}
		}
	}
}

func handleSectionResult(
	s *session,
	previousState config.State,
	previousSecrets config.Secrets,
	err error,
) (bool, error) {
	if err == nil {
		return true, nil
	}
	if errors.Is(err, errBackToSections) {
		s.state = previousState
		s.secrets = previousSecrets
		return false, nil
	}
	return false, err
}

func runSection(ctx context.Context, s *session, selected string) error {
	handlers := map[string]func() error{
		"General":   func() error { return editGeneral(s) },
		"User":      func() error { return editUser(s) },
		"Region":    func() error { return editRegion(s) },
		"Display":   func() error { return editDisplay(ctx, s) },
		"Packages":  func() error { return editPackages(s) },
		"Services":  func() error { return editServices(s) },
		"Dots":      func() error { return editDots(s) },
		"Passwords": func() error { return editSecrets(s) },
		"Cleanup":   func() error { return runCleanup(ctx, s) },
		"Doctor":    func() error { return showDoctor(ctx, s) },
		"Apply":     func() error { return runApply(ctx, s) },
	}
	handler, ok := handlers[selected]
	if !ok {
		return fmt.Errorf("unknown section %q", selected)
	}
	return handler()
}

func isDirtySection(selected string) bool {
	return selected != "Apply" && selected != "Doctor" && selected != "Cleanup" && selected != "Passwords"
}

func loadInitialState(opts paths.Options) (config.State, error) {
	if _, err := os.Stat(opts.StatePath); err == nil {
		return config.LoadExisting(opts.StatePath)
	} else if !os.IsNotExist(err) {
		return config.State{}, fmt.Errorf("stat state %s: %w", opts.StatePath, err)
	}
	if _, err := os.Stat(opts.DraftPath); err == nil {
		return config.LoadExisting(opts.DraftPath)
	} else if !os.IsNotExist(err) {
		return config.State{}, fmt.Errorf("stat draft %s: %w", opts.DraftPath, err)
	}
	return config.Default(), nil
}

func runApply(ctx context.Context, s *session) error {
	if err := config.Validate(s.state); err != nil {
		return err
	}
	confirm := false
	if err := newForm(
		huh.NewNote().
			Title("Apply summary").
			Description(summary(s.state, s.secrets, detectExistingSecrets(s.paths), s.paths.StatePath)),
		huh.NewConfirm().
			Title("Run dry-build and then ask before switch?").
			Affirmative("Apply").
			Negative("Back").
			Value(&confirm),
	).Run(); err != nil {
		return err
	}
	if !confirm {
		return nil
	}
	if err := apply.Run(ctx, apply.Options{
		Paths:     s.paths,
		State:     s.state,
		Secrets:   s.secrets,
		DryRun:    s.dryRun,
		AssumeYes: false,
		Layout:    s.layout,
		LockMode:  s.lockMode,
	}); err != nil {
		return err
	}
	s.state.Dots.NeovimCleanState = false
	return nil
}

func showDoctor(ctx context.Context, s *session) error {
	report, err := doctor.Report(ctx, doctor.Options{Paths: s.paths, State: s.state})
	if err != nil {
		return err
	}
	return showNote("Doctor", report)
}

func runCleanup(ctx context.Context, s *session) error {
	home, err := cleanup.Home(cleanup.Options{State: s.state})
	if err != nil {
		return err
	}
	report := cleanup.ReportForHome(home)
	confirm := false
	if err := newForm(
		huh.NewConfirm().
			Title("Cleanup").
			Description(report + "\nOnly safe MySetup-managed leftovers are touched.\nThis removes preview wallpapers and Noctalia wallpaper cache files.").
			Value(&confirm),
	).Run(); err != nil {
		return err
	}
	if !confirm {
		return nil
	}
	var out bytes.Buffer
	if err := cleanup.Run(ctx, cleanup.Options{
		Paths:  s.paths,
		State:  s.state,
		DryRun: s.dryRun,
		Yes:    true,
		Stdout: &out,
	}); err != nil {
		return err
	}
	return showNote("Cleanup complete", out.String())
}

func showNote(title, description string) error {
	return newForm(
		huh.NewNote().
			Title(title).
			Description(description + "\nPress Enter or Ctrl+S to return."),
	).Run()
}

func showSectionError(err error) error {
	if err == nil {
		return nil
	}
	if displayErr := showNote("Error", err.Error()); displayErr != nil && !errors.Is(displayErr, errBackToSections) {
		return displayErr
	}
	return nil
}
