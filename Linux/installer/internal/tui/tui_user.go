package tui

import (
	"github.com/charmbracelet/huh"

	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/config"
)

func editUser(s *session) error {
	var errors userFormErrors
	for {
		if err := runUserForm(s, errors); err != nil {
			return err
		}
		errors = validateUserForm(s.state)
		if errors.empty() {
			return nil
		}
	}
}

func runUserForm(s *session, errors userFormErrors) error {
	return newForm(
		huh.NewInput().
			Title("Username").
			Description(fieldDescription("Linux account name used by users/user.nix and Home Manager.", errors.username)).
			Value(&s.state.User.Username),
		huh.NewInput().
			Title("Full name").
			Description(fieldDescription("Display name for account metadata. It can stay equal to the username.", errors.fullName)).
			Value(&s.state.User.FullName),
		huh.NewInput().
			Title("Home directory").
			Description(fieldDescription("Usually /home/<username>; dots and HM config are written relative to it.", errors.homeDirectory)).
			Value(&s.state.User.HomeDirectory),
		huh.NewInput().
			Title("Git user.name").
			Description(fieldDescription("Written to Home Manager git config.", errors.gitUsername)).
			Value(&s.state.Git.Username),
		huh.NewInput().
			Title("Git user.email").
			Description(fieldDescription("Written to Home Manager git config and used by commits.", errors.gitEmail)).
			Value(&s.state.Git.Email),
	).Run()
}

type userFormErrors struct {
	username      string
	fullName      string
	homeDirectory string
	gitUsername   string
	gitEmail      string
}

func (e userFormErrors) empty() bool {
	return e.username == "" &&
		e.fullName == "" &&
		e.homeDirectory == "" &&
		e.gitUsername == "" &&
		e.gitEmail == ""
}

func validateUserForm(state config.State) userFormErrors {
	var errors userFormErrors
	fieldErrors := config.ValidateDetailed(state)
	if fieldErrors.Username != "" {
		errors.username = "username must start with a lowercase letter or underscore and use only lowercase letters, digits, _ or -."
	}
	if fieldErrors.FullName != "" {
		errors.fullName = "full name cannot be empty."
	}
	if fieldErrors.HomeDirectory != "" {
		errors.homeDirectory = "home directory must be a clean /home/<username> path."
	}
	if fieldErrors.GitUsername != "" {
		errors.gitUsername = "git user.name cannot be empty."
	}
	if fieldErrors.GitEmail != "" {
		errors.gitEmail = "git user.email must look like name@example.com."
	}
	return errors
}
