package tui

import (
	"strings"

	"github.com/charmbracelet/huh"
)

func editSecrets(s *session) error {
	existingSecrets := detectExistingSecrets(s.paths)
	return editSecretsWithReader(s, func() (string, error) {
		return readSecrets(existingSecrets)
	})
}

func editSecretsWithReader(
	s *session,
	read func() (userPassword string, err error),
) error {
	userPassword, err := read()
	if err != nil {
		return err
	}
	s.secrets.UserPassword = userPassword
	return nil
}

func readSecrets(existingSecrets secretAvailability) (string, error) {
	values := secretFormValues{}
	errors := secretFormErrors{}
	for {
		if err := runSecretForm(&values, errors, existingSecrets); err != nil {
			return "", err
		}
		errors = validateSecretFormValues(values, existingSecrets)
		if errors.empty() {
			return values.userPassword, nil
		}
	}
}

type secretFormValues struct {
	userPassword string
	userConfirm  string
}

type secretFormErrors struct {
	userPassword string
	userConfirm  string
}

func (e secretFormErrors) empty() bool {
	return e.userPassword == "" &&
		e.userConfirm == ""
}

func runSecretForm(values *secretFormValues, errors secretFormErrors, existingSecrets secretAvailability) error {
	if err := newForm(
		huh.NewNote().
			Title("Password state").
			Description(secretFormStatus(existingSecrets)),
		huh.NewInput().
			Title("Linux user password").
			Description(passwordFieldDescription("Written as hashed-password.nix during Apply.", existingSecrets.UserPassword, errors.userPassword)).
			EchoMode(huh.EchoModePassword).
			Value(&values.userPassword),
		huh.NewInput().
			Title("Confirm Linux user password").
			Description(fieldDescription("Repeat the Linux user password, or leave blank with the password field to preserve an existing hash.", errors.userConfirm)).
			EchoMode(huh.EchoModePassword).
			Value(&values.userConfirm),
	).Run(); err != nil {
		return err
	}
	return nil
}

func validateSecretFormValues(values secretFormValues, existingSecrets secretAvailability) secretFormErrors {
	var errors secretFormErrors
	errors.userPassword, errors.userConfirm = validateSecretPair(
		values.userPassword,
		values.userConfirm,
		"Linux user password",
		existingSecrets.UserPassword,
	)
	return errors
}

func validateSecretPair(password, confirm, label string, existing secretPresence) (string, string) {
	if password == "" && confirm == "" {
		if existing == secretPresenceExists {
			return "", ""
		}
		return label + " cannot be empty.", label + " confirmation cannot be empty."
	}
	if password == "" {
		return label + " cannot be empty.", ""
	}
	if confirm == "" {
		return "", label + " confirmation cannot be empty."
	}
	if password != confirm {
		return "", label + " does not match confirmation."
	}
	return "", ""
}

func secretFormStatus(existingSecrets secretAvailability) string {
	return strings.Join([]string{
		"Linux user password: " + secretFormPresence(existingSecrets.UserPassword),
		"Leave password and confirmation blank only for entries marked already exists.",
		"Plain passwords are not saved to draft.json.",
	}, "\n")
}

func secretFormPresence(existing secretPresence) string {
	switch existing {
	case secretPresenceExists:
		return "already exists"
	case secretPresenceUnknown:
		return "unknown"
	default:
		return "not found"
	}
}

func passwordFieldDescription(description string, existing secretPresence, errorMessage string) string {
	switch existing {
	case secretPresenceExists:
		description += " Leave password and confirmation blank to preserve the existing value."
	case secretPresenceUnknown:
		description += " Existing value could not be checked; enter a password to replace it."
	default:
		description += " Required because no existing value was detected."
	}
	return fieldDescription(description, errorMessage)
}

func fieldDescription(description, errorMessage string) string {
	if errorMessage == "" {
		return description
	}
	return description + "\n" + formErrorStyle.Render("error: "+errorMessage)
}
