package apply

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/config"
	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/defaults"
	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/run"
	"golang.org/x/sys/unix"
)

func prepareStagingHostLocal(ctx context.Context, runner run.CommandRunner, staging, dest string, state config.State, secrets config.Secrets, layout Layout) error {
	if layout == LayoutThin {
		if err := copyExistingThinHostLocal(ctx, runner, staging, dest, state); err != nil {
			return err
		}
	}
	if err := copyHostHardware(staging, dest, layout); err != nil {
		return err
	}
	if secrets.UserPassword != "" {
		hash := "!mysetup-dry-run-placeholder"
		if !runner.IsDryRun() {
			var err error
			hash, err = hashPassword(ctx, secrets.UserPassword)
			if err != nil {
				return err
			}
		}
		target := filepath.Join(staging, hashedPasswordRel(layout))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, []byte(HashedPasswordNix(hash)), 0o600)
	}
	return copyExistingHashedPassword(ctx, runner, existingHashedPasswordPaths(dest), filepath.Join(staging, hashedPasswordRel(layout)))
}

func copyExistingThinHostLocal(ctx context.Context, runner run.CommandRunner, staging, dest string, state config.State) error {
	if _, _, err := existingUserDirectorySource(dest); err != nil {
		return err
	}
	thinFlake, err := copyExistingThinFlake(dest, filepath.Join(staging, "flake.nix"), state)
	if err != nil {
		return err
	}

	preservedFiles := []string{}
	if thinFlake {
		preservedFiles = []string{"flake.lock", "configuration.nix", "home.nix"}
	}
	if err := copyExistingThinHostLocalFiles(dest, staging, preservedFiles); err != nil {
		return err
	}

	if err := stageExistingUserDirectory(ctx, runner, dest, staging); err != nil {
		return err
	}
	if err := writeUserDefaultTemplate(staging); err != nil {
		return err
	}

	return copyExistingThinSecrets(ctx, runner, dest, staging)
}

func copyExistingThinHostLocalFiles(dest, staging string, names []string) error {
	for _, name := range names {
		source := filepath.Join(dest, name)
		if _, err := os.Stat(source); err == nil {
			target := filepath.Join(staging, name)
			if err := copyFile(source, target); err != nil {
				return err
			}
			if name == "configuration.nix" {
				if err := migrateConfigurationUserImport(target); err != nil {
					return err
				}
			}
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func copyExistingThinSecrets(ctx context.Context, runner run.CommandRunner, dest, staging string) error {
	for _, secretsDir := range existingSecretsDirs(dest) {
		if info, err := os.Stat(secretsDir); err == nil {
			if !info.IsDir() {
				return fmt.Errorf("secrets path is not a directory: %s", secretsDir)
			}
			if err := copyHostLocalTree(ctx, runner, secretsDir, filepath.Join(staging, "secrets")); err != nil {
				return fmt.Errorf("stage secrets: %w", err)
			}
			return nil
		} else if os.IsPermission(err) {
			if err := sudoCopyHostLocalTree(ctx, runner, secretsDir, filepath.Join(staging, "secrets")); err != nil {
				return fmt.Errorf("stage secrets: %w", err)
			}
			return nil
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func stageExistingUserDirectory(ctx context.Context, runner run.CommandRunner, dest, staging string) error {
	source, exists, err := existingUserDirectorySource(dest)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	if err := copyHostLocalTree(ctx, runner, source, filepath.Join(staging, "user")); err != nil {
		return fmt.Errorf("stage user directory: %w", err)
	}
	return nil
}

func existingUserDirectorySource(dest string) (string, bool, error) {
	userDir := filepath.Join(dest, "user")
	legacyDir := filepath.Join(dest, "private")
	userExists, err := ordinaryDirectory(userDir, "user")
	if err != nil {
		return "", false, err
	}
	legacyExists, err := ordinaryDirectory(legacyDir, "legacy user")
	if err != nil {
		return "", false, err
	}
	if userExists && legacyExists {
		return "", false, fmt.Errorf("both user and legacy private directories exist; migrate manually before applying")
	}
	if userExists {
		return userDir, true, nil
	}
	if legacyExists {
		return legacyDir, true, nil
	}
	return "", false, nil
}

func ordinaryDirectory(path, label string) (bool, error) {
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return false, fmt.Errorf("unsupported %s path: %s", label, path)
	}
	return true, nil
}

func migrateConfigurationUserImport(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	migrated := rewriteLegacyUserImport(string(data))
	if migrated == string(data) {
		return nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	return os.WriteFile(path, []byte(migrated), info.Mode().Perm())
}

func rewriteLegacyUserImport(text string) string {
	rewritten, _ := rewriteNixCode(text, 0, false)
	return rewritten
}

var (
	nixURITokenPattern   = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*:[A-Za-z0-9%/?:@&=+$,_.!~*'-]+`)
	nixSearchPathPattern = regexp.MustCompile(`^<[A-Za-z0-9._+-]+(?:/[A-Za-z0-9._+-]+)*>`)
	nixPathTokenPattern  = regexp.MustCompile(`^[A-Za-z0-9._+-]*(?:/[A-Za-z0-9._+-]+)+/?`)
	nixHomePathPattern   = regexp.MustCompile(`^~/(?:[A-Za-z0-9._+-]+/)*[A-Za-z0-9._+-]*`)
	nixInPathLiteral     = regexp.MustCompile(`^[A-Za-z0-9._+/-]+`)
	nixIdentifierPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_'-]*`)
)

const (
	legacyUserPathPrefix    = "./private/"
	canonicalUserPathPrefix = "./user/"
)

func rewriteNixCode(text string, index int, stopAtClosingBrace bool) (string, int) {
	var out strings.Builder
	for index < len(text) {
		if stopAtClosingBrace && text[index] == '}' {
			out.WriteByte('}')
			return out.String(), index + 1
		}
		if text[index] == '#' {
			end := strings.IndexByte(text[index:], '\n')
			if end < 0 {
				out.WriteString(text[index:])
				return out.String(), len(text)
			}
			end += index + 1
			out.WriteString(text[index:end])
			index = end
			continue
		}
		if strings.HasPrefix(text[index:], "/*") {
			end := nixBlockCommentEnd(text, index)
			out.WriteString(text[index:end])
			index = end
			continue
		}
		if text[index] == '"' {
			rewritten, end := rewriteNixQuotedString(text, index)
			out.WriteString(rewritten)
			index = end
			continue
		}
		if strings.HasPrefix(text[index:], "''") {
			rewritten, end := rewriteNixIndentedString(text, index)
			out.WriteString(rewritten)
			index = end
			continue
		}
		if rewritten, end, ok := rewriteNixInterpolatedPath(text, index); ok {
			out.WriteString(rewritten)
			index = end
			continue
		}
		if stopAtClosingBrace && text[index] == '{' {
			out.WriteByte('{')
			rewritten, end := rewriteNixCode(text, index+1, true)
			out.WriteString(rewritten)
			index = end
			continue
		}
		if replacement, end, ok := rewriteNixToken(text, index); ok {
			out.WriteString(replacement)
			index = end
			continue
		}
		out.WriteByte(text[index])
		index++
	}
	return out.String(), index
}

func rewriteNixQuotedString(text string, index int) (string, int) {
	var out strings.Builder
	out.WriteByte('"')
	index++
	for index < len(text) {
		if text[index] == '\\' {
			out.WriteByte(text[index])
			index++
			if index < len(text) {
				out.WriteByte(text[index])
				index++
			}
			continue
		}
		if text[index] == '"' {
			out.WriteByte('"')
			return out.String(), index + 1
		}
		if strings.HasPrefix(text[index:], "${") {
			out.WriteString("${")
			rewritten, end := rewriteNixCode(text, index+2, true)
			out.WriteString(rewritten)
			index = end
			continue
		}
		out.WriteByte(text[index])
		index++
	}
	return out.String(), index
}

func rewriteNixIndentedString(text string, index int) (string, int) {
	var out strings.Builder
	out.WriteString("''")
	index += 2
	for index < len(text) {
		if strings.HasPrefix(text[index:], "''") && index+2 < len(text) {
			switch text[index+2] {
			case '$', '\'':
				out.WriteString(text[index : index+3])
				index += 3
				continue
			case '\\':
				end := index + 3
				if end < len(text) {
					end++
				}
				out.WriteString(text[index:end])
				index = end
				continue
			}
		}
		if strings.HasPrefix(text[index:], "''") {
			out.WriteString("''")
			return out.String(), index + 2
		}
		if strings.HasPrefix(text[index:], "${") {
			out.WriteString("${")
			rewritten, end := rewriteNixCode(text, index+2, true)
			out.WriteString(rewritten)
			index = end
			continue
		}
		out.WriteByte(text[index])
		index++
	}
	return out.String(), index
}

func nixBlockCommentEnd(text string, index int) int {
	depth := 1
	index += 2
	for index < len(text) && depth > 0 {
		switch {
		case strings.HasPrefix(text[index:], "/*"):
			depth++
			index += 2
		case strings.HasPrefix(text[index:], "*/"):
			depth--
			index += 2
		default:
			index++
		}
	}
	return index
}

func rewriteNixToken(text string, index int) (string, int, bool) {
	remaining := text[index:]
	if token := nixURITokenPattern.FindString(remaining); token != "" {
		return token, index + len(token), true
	}
	if token := nixSearchPathPattern.FindString(remaining); token != "" {
		return token, index + len(token), true
	}
	if strings.HasPrefix(remaining, "//") {
		return "//", index + 2, true
	}
	if token := nixPathTokenPattern.FindString(remaining); token != "" {
		end := index + len(token)
		return rewriteLegacyNixPathToken(token), end, true
	}
	if token := nixHomePathPattern.FindString(remaining); token != "" {
		return token, index + len(token), true
	}
	if token := nixIdentifierPattern.FindString(remaining); token != "" {
		return token, index + len(token), true
	}
	return "", index, false
}

func rewriteLegacyNixPathToken(token string) string {
	switch {
	case token == "./private":
		return "./user"
	case strings.HasPrefix(token, legacyUserPathPrefix):
		return canonicalUserPathPrefix + strings.TrimPrefix(token, legacyUserPathPrefix)
	default:
		return token
	}
}

func rewriteNixInterpolatedPath(text string, index int) (string, int, bool) {
	path := nixFilesystemPathToken(text[index:])
	if path == "" {
		return "", index, false
	}
	var out strings.Builder
	out.WriteString(rewriteLegacyNixPathToken(path))
	cursor := index + len(path)
	if literal := nixInPathLiteral.FindString(text[cursor:]); literal != "" {
		out.WriteString(literal)
		cursor += len(literal)
	}
	if !strings.HasPrefix(text[cursor:], "${") {
		return "", index, false
	}
	for cursor < len(text) {
		if strings.HasPrefix(text[cursor:], "${") {
			out.WriteString("${")
			rewritten, end := rewriteNixCode(text, cursor+2, true)
			out.WriteString(rewritten)
			cursor = end
			continue
		}
		literal := nixInPathLiteral.FindString(text[cursor:])
		if literal == "" {
			break
		}
		out.WriteString(literal)
		cursor += len(literal)
	}
	return out.String(), cursor, true
}

func nixFilesystemPathToken(text string) string {
	if token := nixPathTokenPattern.FindString(text); token != "" {
		return token
	}
	return nixHomePathPattern.FindString(text)
}

func copyExistingThinFlake(dest, target string, state config.State) (bool, error) {
	source := filepath.Join(dest, "flake.nix")
	data, err := os.ReadFile(source)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if !isThinWrapperFlake(string(data)) {
		return false, nil
	}
	text := string(data)
	if isGeneratedMySetupWrapperFlake(text) {
		migrated, changed, err := migrateGeneratedThinFlake(text, state)
		if err != nil {
			return false, fmt.Errorf("regenerate generated Wahrwelt wrapper: %w", err)
		}
		if changed {
			info, err := os.Stat(source)
			if err != nil {
				return false, err
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return false, err
			}
			return true, os.WriteFile(target, []byte(migrated), info.Mode().Perm())
		}
		return true, copyFile(source, target)
	}
	return true, copyFile(source, target)
}

func isThinWrapperFlake(text string) bool {
	return strings.Contains(text, "wahrwelt.lib.mkWahrweltHost") ||
		strings.Contains(text, "mysetup.lib.mkMySetupHost") ||
		containsWahrweltNixOSFlakeURL(text)
}

func isGeneratedMySetupWrapperFlake(text string) bool {
	for _, marker := range []string{
		"# Generated by Wahrwelt installer.",
		"# Generated by MySetup installer.",
	} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	if !containsWahrweltNixOSFlakeURL(text) {
		return false
	}
	hostMarkerFound := false
	for _, marker := range []string{
		`nixosConfigurations.${hostname} = wahrwelt.lib.mkWahrweltHost {`,
		`nixosConfigurations.${hostname} = mysetup.lib.mkMySetupHost {`,
	} {
		if strings.Contains(text, marker) {
			hostMarkerFound = true
			break
		}
	}
	if !hostMarkerFound {
		return false
	}
	descriptionFound := strings.Contains(text, `description = "Host-local Wahrwelt NixOS wrapper";`) ||
		strings.Contains(text, `description = "Host-local MySetup NixOS wrapper";`)
	if !descriptionFound {
		return false
	}
	for _, marker := range []string{
		`hostVars = ./host-vars.nix;`,
		`hardware = ./hardware-configuration.nix;`,
		`extraModules = [ ./configuration.nix ];`,
		`if builtins.pathExists ./home.nix then [ ./home.nix ] else [ ];`,
	} {
		if !strings.Contains(text, marker) {
			return false
		}
	}
	return true
}

func containsWahrweltNixOSFlakeURL(text string) bool {
	for _, flakeURL := range config.KnownWahrweltFlakeURLs() {
		if strings.Contains(text, flakeURL) {
			return true
		}
	}
	return false
}

func copyHostLocalTree(ctx context.Context, runner run.CommandRunner, source, target string) error {
	if err := copyTree(ctx, runner, source, target); err != nil {
		if sudoErr := sudoCopyHostLocalTree(ctx, runner, source, target); sudoErr != nil {
			return err
		}
	}
	return nil
}

func sudoCopyHostLocalTree(ctx context.Context, runner run.CommandRunner, source, target string) error {
	uid := strconv.Itoa(os.Getuid())
	gid := strconv.Itoa(os.Getgid())
	if err := runner.Command(ctx, "sudo", "mkdir", "-p", target); err != nil {
		return err
	}
	return runner.Command(
		ctx,
		"sudo",
		"rsync",
		"-a",
		"--delete",
		"--checksum",
		"--chown",
		uid+":"+gid,
		source+"/",
		target+"/",
	)
}

func existingHashedPasswordPaths(dest string) []string {
	return []string{
		filepath.Join(dest, "hashed-password.nix"),
		filepath.Join(dest, "hosts", "NixOS", "hashed-password.nix"),
	}
}

func existingSecretsDirs(dest string) []string {
	return []string{
		filepath.Join(dest, "secrets"),
		filepath.Join(dest, "hosts", "NixOS", "secrets"),
	}
}

func copyExistingHashedPassword(ctx context.Context, runner run.CommandRunner, sources []string, target string) error {
	for _, source := range sources {
		copied, err := copyExistingHashedPasswordFile(ctx, runner, source, target)
		if err != nil || copied {
			return err
		}
	}
	return nil
}

func copyExistingHashedPasswordFile(ctx context.Context, runner run.CommandRunner, source, target string) (bool, error) {
	if _, err := os.Stat(source); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		if os.IsPermission(err) {
			return true, sudoCopyHostLocalFile(ctx, runner, source, target)
		}
		return false, err
	}
	if err := copyFile(source, target); err != nil {
		if os.IsPermission(err) {
			return true, sudoCopyHostLocalFile(ctx, runner, source, target)
		}
		return false, err
	}
	return true, nil
}

func sudoCopyHostLocalFile(ctx context.Context, runner run.CommandRunner, source, target string) error {
	uid := strconv.Itoa(os.Getuid())
	gid := strconv.Itoa(os.Getgid())
	if err := runner.Command(ctx, "sudo", "install", "-D", "-m", "600", "-o", uid, "-g", gid, source, target); err != nil {
		return fmt.Errorf("copy existing hashed-password.nix with sudo: %w; provide --user-password-file to generate a fresh hash", err)
	}
	return nil
}

func hashPassword(ctx context.Context, password string) (string, error) {
	rounds := fmt.Sprintf("--rounds=%d", defaults.ShaCryptRounds)
	cmd := exec.CommandContext(ctx, "mkpasswd", "-sm", "sha-512", rounds)
	cmd.Stdin = strings.NewReader(password)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("mkpasswd failed: %w", err)
	}
	hash := strings.TrimSpace(string(out))
	if hash == "" {
		return "", fmt.Errorf("mkpasswd produced empty hash")
	}
	return hash, nil
}

//nolint:gocyclo // Publication preflight and descriptor handoff stay linear so every early return fails closed.
func writeStagedUserDefault(ctx context.Context, runner run.CommandRunner, staging, dest string, layout Layout) error {
	if layout != LayoutThin {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	source, sourceExists, err := stagedUserDefaultSource(staging)
	if err != nil {
		return err
	}
	if !sourceExists {
		return nil
	}
	target := filepath.Join(dest, "user", "default.nix")
	if runner.IsDryRun() {
		needsPublication, err := userDefaultNeedsPublication(target)
		if err != nil {
			return err
		}
		if !needsPublication {
			return nil
		}
		fmt.Println("write user/default.nix")
		return nil
	}
	sourcePinned, err := openPinnedRegularFile(source, "staged user default")
	if err != nil {
		return err
	}
	defer closeFile(sourcePinned.file)
	sourceData, err := readPinnedRegularFile(sourcePinned)
	if err != nil {
		return fmt.Errorf("read staged user/default.nix: %w", err)
	}
	publicationSource, cleanupPublicationSource, err := createSealedPinnedSource("wahrwelt-user-default", sourceData)
	if err != nil {
		return fmt.Errorf("snapshot staged user/default.nix: %w", err)
	}
	defer cleanupPublicationSource()
	root, err := openPinnedParentDirectory(filepath.Join(dest, "user"), "user default root")
	if err != nil {
		return err
	}
	defer closeFile(root)
	userDir, exists, err := openPinnedDirectoryEntryAt(root, "user", filepath.Join(dest, "user"), "user")
	if err != nil {
		return err
	}
	if !exists {
		return publishStagedUserDefaultWithDirectoryCreate(ctx, runner, publicationSource, root, dest, target)
	}
	defer closeFile(userDir.file)
	if err := verifyPinnedUserDestination(root, userDir, dest); err != nil {
		return err
	}
	targetExists, err := preservableUserDefaultAt(userDir.file, filepath.Base(target), target)
	if err != nil {
		return err
	}
	if targetExists {
		return verifyPinnedUserDestination(root, userDir, dest)
	}
	return publishStagedUserDefault(ctx, runner, publicationSource, root, userDir, target)
}

func stagedUserDefaultSource(staging string) (string, bool, error) {
	source := filepath.Join(staging, "user", "default.nix")
	exists, err := preservableUserDefault(source)
	if err != nil || !exists {
		return source, exists, err
	}
	info, err := os.Lstat(source)
	if err != nil {
		return source, false, err
	}
	return source, info.Mode()&os.ModeSymlink == 0, nil
}

func userDefaultNeedsPublication(target string) (bool, error) {
	if err := ordinaryPathParent(target, "user default"); err != nil {
		return false, err
	}
	exists, err := preservableUserDefault(target)
	return !exists, err
}

const privilegedCreateUserDefaultScript = `
set -eu

root_fd=$1
source_fd=$2
root_display=$3
python_bin=$4

root_id=$(stat -Lc '%d:%i' -- "$root_fd")
visible_root_matches() {
	[ ! -L "$root_display" ] && [ -d "$root_display" ] &&
		[ "$(stat -c '%d:%i' -- "$root_display" 2>/dev/null || true)" = "$root_id" ]
}
user_absent() {
	[ ! -e "$root_fd/user" ] && [ ! -L "$root_fd/user" ]
}
if ! visible_root_matches || ! user_absent; then
	printf '%s\n' "user directory appeared or destination root changed before publication: $root_display/user" >&2
	exit 17
fi

` + privilegedPinnedDirectoryCreatorScript + `

umask 077
create_pinned_directory "$root_fd" ".wahrwelt-user-default-recovery-" "$python_bin"
candidate_name=$created_name
candidate_id=$created_id
candidate_fd=/proc/self/fd/8
cd -- "$candidate_fd"
install -m 0644 -- "$source_fd" default.nix
[ ! -L default.nix ] && [ -f default.nix ]
cmp -s -- "$source_fd" default.nix
chmod 0755 -- .

candidate_matches() {
	[ ! -L "$root_fd/$candidate_name" ] && [ -d "$root_fd/$candidate_name" ] &&
		[ "$(stat -c '%d:%i' -- "$root_fd/$candidate_name" 2>/dev/null || true)" = "$candidate_id" ] &&
		[ "$(stat -Lc '%d:%i' -- "$candidate_fd")" = "$candidate_id" ]
}
restore_created_user() {
	user_id=$(stat -c '%d:%i' -- "$root_fd/user" 2>/dev/null || true)
	if [ "$user_id" = "$candidate_id" ] &&
		mv -T --no-copy --update=none-fail -- "$root_fd/user" "$root_fd/$candidate_name"; then
		printf '%s\n' "created user directory retained for recovery at $root_display/$candidate_name" >&2
		exit 1
	fi
	if [ -n "$user_id" ] && [ ! -e "$root_fd/$candidate_name" ] && [ ! -L "$root_fd/$candidate_name" ] &&
		mv -T --no-copy --update=none-fail -- "$root_fd/user" "$root_fd/$candidate_name"; then
		restored_id=$(stat -c '%d:%i' -- "$root_fd/$candidate_name" 2>/dev/null || true)
		if [ "$restored_id" = "$user_id" ]; then
			printf '%s\n' "unexpected user directory restored at $root_display/$candidate_name; expected recovery retained through pinned descriptor $candidate_fd" >&2
			exit 1
		fi
	fi
	printf '%s\n' "created user directory publication has an uncertain recovery under pinned $root_display" >&2
	exit 1
}

if ! visible_root_matches || ! user_absent || ! candidate_matches; then
	printf '%s\n' "user directory publication precondition changed; recovery retained at $root_display/$candidate_name" >&2
	exit 1
fi
if ! mv -T --no-copy --update=none-fail -- "$root_fd/$candidate_name" "$root_fd/user"; then
	printf '%s\n' "user directory collision; recovery retained at $root_display/$candidate_name" >&2
	exit 17
fi
moved_id=$(stat -c '%d:%i' -- "$root_fd/user" 2>/dev/null || true)
if [ "$moved_id" != "$candidate_id" ]; then
	restore_created_user
fi
if ! visible_root_matches; then
	restore_created_user
fi
printf '%s\n' "$candidate_id"
`

//nolint:gocyclo // Post-publication identity and content checks form one fail-closed transaction.
func publishStagedUserDefaultWithDirectoryCreate(ctx context.Context, runner run.CommandRunner, source *pinnedRegularFile, root *os.File, dest, target string) error {
	if source == nil || source.file == nil || root == nil {
		return fmt.Errorf("missing pinned user/default.nix creation handles")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	publishCtx, cancel := cleanupContext(ctx)
	defer cancel()
	pythonPath, err := privilegedPythonPath()
	if err != nil {
		return err
	}
	token, err := runner.Output(
		publishCtx,
		"sudo", "sh", "-c", privilegedCreateUserDefaultScript, "--",
		fileDescriptorPath(root), fileDescriptorPath(source.file), dest, pythonPath,
	)
	runtime.KeepAlive(root)
	runtime.KeepAlive(source.file)
	if err != nil {
		return fmt.Errorf("create user directory with default.nix: %w", err)
	}
	userDir, exists, err := openPinnedDirectoryEntryAt(root, "user", filepath.Join(dest, "user"), "user")
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("created user directory is absent beneath pinned destination: %s", filepath.Join(dest, "user"))
	}
	defer closeFile(userDir.file)
	userFD, err := checkedFileDescriptor(userDir.file, "created user directory")
	if err != nil {
		return err
	}
	var stat unix.Stat_t
	if err := unix.Fstat(userFD, &stat); err != nil {
		return fmt.Errorf("stat created user directory: %w", err)
	}
	if got := fmt.Sprintf("%d:%d", stat.Dev, stat.Ino); token != got {
		return fmt.Errorf("created user directory changed before pin: %s", filepath.Join(dest, "user"))
	}
	if err := verifyPinnedUserDestination(root, userDir, dest); err != nil {
		return err
	}
	published, exists, err := openPinnedRegularFileAt(userDir.file, filepath.Base(target), target, "user default")
	if err != nil {
		return fmt.Errorf("verify created user/default.nix: %w", err)
	}
	if !exists {
		return fmt.Errorf("created user/default.nix is absent: %s", target)
	}
	defer closeFile(published.file)
	sameContent, err := samePinnedContent(source, published)
	if err != nil {
		return fmt.Errorf("read created user/default.nix: %w", err)
	}
	if !sameContent {
		return fmt.Errorf("created user/default.nix content changed during publication: %s", target)
	}
	return verifyPinnedUserDestination(root, userDir, dest)
}

func preservableUserDefaultAt(parent *os.File, name, display string) (bool, error) {
	parentFD, err := checkedFileDescriptor(parent, "user default parent")
	if err != nil {
		return false, err
	}
	var info unix.Stat_t
	if err := unix.Fstatat(parentFD, name, &info, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		if errors.Is(err, unix.ENOENT) {
			return false, nil
		}
		return false, &os.PathError{Op: "lstat", Path: display, Err: err}
	}
	switch info.Mode & unix.S_IFMT {
	case unix.S_IFREG, unix.S_IFLNK:
		return true, nil
	default:
		return false, fmt.Errorf("unsupported user/default.nix path: %s", display)
	}
}

func verifyPinnedUserDestination(root *os.File, userDir *pinnedDirectoryEntry, dest string) error {
	display := filepath.Join(dest, "user")
	if err := verifyPinnedDirectoryEntry(root, "user", display, "user", userDir); err != nil {
		return err
	}
	return verifyPinnedDirectoryVisible(root, dest, "user default root")
}

func publishStagedUserDefault(ctx context.Context, runner run.CommandRunner, sourcePinned *pinnedRegularFile, root *os.File, userDir *pinnedDirectoryEntry, target string) error {
	if err := publishPinnedWithCleanupContext(ctx, runner, "user-default", userDir.file, target, sourcePinned, nil); err != nil {
		if resolveErr := resolveUserDefaultPublishErrorAt(userDir.file, filepath.Base(target), target, err); resolveErr != nil {
			return resolveErr
		}
		return verifyPinnedUserDestination(root, userDir, filepath.Dir(filepath.Dir(target)))
	}
	published, exists, err := openPinnedRegularFileAt(userDir.file, filepath.Base(target), target, "user default")
	if err != nil {
		return fmt.Errorf("verify published user/default.nix: %w", err)
	}
	if !exists {
		return fmt.Errorf("verify published user/default.nix: target is absent: %s", target)
	}
	defer closeFile(published.file)
	sameContent, err := samePinnedContent(sourcePinned, published)
	if err != nil {
		return fmt.Errorf("read published user/default.nix: %w", err)
	}
	if !sameContent {
		return fmt.Errorf("published user/default.nix content changed during publication: %s", target)
	}
	return verifyPinnedUserDestination(root, userDir, filepath.Dir(filepath.Dir(target)))
}

func resolveUserDefaultPublishErrorAt(parent *os.File, name, display string, publishErr error) error {
	if !isUserDefaultPublishCollision(publishErr) {
		return fmt.Errorf("publish user/default.nix: %w", publishErr)
	}
	targetExists, err := preservableUserDefaultAt(parent, name, display)
	if err != nil {
		return err
	}
	if targetExists {
		return nil
	}
	return fmt.Errorf("user/default.nix collision reported without a target: %w", publishErr)
}

func isUserDefaultPublishCollision(err error) bool {
	var exitErr *exec.ExitError
	return errors.As(err, &exitErr) && exitErr.ExitCode() == 17
}

func openPinnedDirectoryPath(path, label string) (*pinnedDirectoryEntry, error) {
	parent, err := openPinnedParentDirectory(path, label)
	if err != nil {
		return nil, err
	}
	defer closeFile(parent)
	directory, exists, err := openPinnedDirectoryEntryAt(parent, filepath.Base(path), path, label)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, &os.PathError{Op: "open", Path: path, Err: unix.ENOENT}
	}
	return directory, nil
}

func verifyPublishedRegularFileAt(parent *os.File, name, display, label string, expectedTarget *pinnedRegularFile, expectedData []byte) error {
	published, exists, err := openPinnedRegularFileAt(parent, name, display, label)
	if err != nil {
		return fmt.Errorf("verify published %s: %w", label, err)
	}
	if !exists {
		return fmt.Errorf("verify published %s: target is absent: %s", label, display)
	}
	defer closeFile(published.file)
	if expectedTarget != nil && sameRegularFile(published.info, expectedTarget.info) {
		return fmt.Errorf("published %s target was not replaced: %s", label, display)
	}
	actualData, err := readPinnedRegularFile(published)
	if err != nil {
		return fmt.Errorf("read published %s: %w", label, err)
	}
	if !bytes.Equal(actualData, expectedData) {
		return fmt.Errorf("published %s content changed during publication: %s", label, display)
	}
	if published.info.Mode().Perm() != 0o600 {
		return fmt.Errorf("published %s mode = %04o, want 0600: %s", label, published.info.Mode().Perm(), display)
	}
	return nil
}

//nolint:gocyclo // Dry-run preservation and pinned publication checks are intentionally explicit and linear.
func writeStagedHashedPassword(ctx context.Context, runner run.CommandRunner, staging, dest string, secrets config.Secrets, layout Layout) error {
	source := filepath.Join(staging, hashedPasswordRel(layout))
	sourcePinned, err := openPinnedRegularFile(source, "staged hashed-password.nix")
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer closeFile(sourcePinned.file)
	expectedData, err := readPinnedRegularFile(sourcePinned)
	if err != nil {
		return fmt.Errorf("read staged hashed-password.nix: %w", err)
	}
	publicationSource, cleanupPublicationSource, err := createSealedPinnedSource("wahrwelt-hashed-password", expectedData)
	if err != nil {
		return fmt.Errorf("snapshot staged hashed-password.nix: %w", err)
	}
	defer cleanupPublicationSource()
	target := filepath.Join(dest, hashedPasswordRel(layout))
	if runner.IsDryRun() {
		targetInfo, err := regularFileOrAbsent(target, "hashed-password.nix")
		if err != nil {
			return fmt.Errorf("target %s is not a regular file", hashedPasswordRel(layout))
		}
		if secrets.UserPassword == "" && targetInfo != nil {
			return nil
		}
		fmt.Printf("write %s\n", hashedPasswordRel(layout))
		return nil
	}
	parent, err := openPinnedParentDirectory(target, "hashed-password.nix")
	if err != nil {
		return err
	}
	defer closeFile(parent)
	expectedTarget, exists, err := openPinnedRegularFileAt(parent, filepath.Base(target), target, "hashed-password.nix")
	if err != nil {
		return fmt.Errorf("target %s is not a regular file: %w", hashedPasswordRel(layout), err)
	}
	if expectedTarget != nil {
		defer closeFile(expectedTarget.file)
	}
	if secrets.UserPassword == "" && exists {
		return verifyPinnedDirectoryVisible(parent, filepath.Dir(target), "hashed-password.nix")
	}
	if err := publishPinnedWithCleanupContext(ctx, runner, "hashed-password", parent, target, publicationSource, expectedTarget); err != nil {
		return err
	}
	if err := verifyPublishedRegularFileAt(parent, filepath.Base(target), target, "hashed-password.nix", expectedTarget, expectedData); err != nil {
		return err
	}
	return verifyPinnedDirectoryVisible(parent, filepath.Dir(target), "hashed-password.nix")
}

const privilegedPublishSecretsScript = `
set -eu

parent_fd=$1
source_fd=$2
parent_display=$3
python_bin=$4
target_name=secrets

parent_id=$(stat -Lc '%d:%i' -- "$parent_fd")
source_id=$(stat -Lc '%d:%i' -- "$source_fd")
visible_parent_matches() {
	[ ! -L "$parent_display" ] && [ -d "$parent_display" ] &&
		[ "$(stat -c '%d:%i' -- "$parent_display" 2>/dev/null || true)" = "$parent_id" ]
}
target_absent() {
	[ ! -e "$parent_fd/$target_name" ] && [ ! -L "$parent_fd/$target_name" ]
}
if ! visible_parent_matches || ! target_absent; then
	printf '%s\n' "secrets target appeared or destination root changed before publication: $parent_display/$target_name" >&2
	exit 17
fi

` + privilegedPinnedDirectoryCreatorScript + `

umask 077
create_pinned_directory "$parent_fd" ".wahrwelt-secrets-recovery-" "$python_bin"
candidate_name=$created_name
candidate_id=$created_id
candidate_fd=/proc/self/fd/8
if [ "$(stat -Lc '%d:%i' -- "$source_fd")" != "$source_id" ]; then
	printf '%s\n' "staged secrets directory changed before copy; recovery retained at $parent_display/$candidate_name" >&2
	exit 1
fi
if [ "$(id -u)" = 0 ]; then
	rsync -a --delete --checksum --chown root:root "$source_fd/" "$candidate_fd/"
else
	rsync -a --delete --checksum "$source_fd/" "$candidate_fd/"
fi
# Nix store paths intentionally strip owner write bits. Secrets are published
# from the validated store tree, so restore a private writable shape without
# granting group/other access.
chmod -R u+rwX,go-rwx -- "$candidate_fd"
candidate_matches() {
	[ ! -L "$parent_fd/$candidate_name" ] && [ -d "$parent_fd/$candidate_name" ] &&
		[ "$(stat -c '%d:%i' -- "$parent_fd/$candidate_name" 2>/dev/null || true)" = "$candidate_id" ] &&
		[ "$(stat -Lc '%d:%i' -- "$candidate_fd")" = "$candidate_id" ]
}
if ! candidate_matches || ! visible_parent_matches; then
	printf '%s\n' "secrets candidate or destination root changed; recovery retained at $parent_display/$candidate_name" >&2
	exit 1
fi
if ! target_absent; then
	printf '%s\n' "secrets target appeared during publication; recovery retained at $parent_display/$candidate_name" >&2
	exit 17
fi
if ! mv -T --no-copy --update=none-fail -- "$parent_fd/$candidate_name" "$parent_fd/$target_name"; then
	printf '%s\n' "secrets target collision; recovery retained at $parent_display/$candidate_name" >&2
	exit 17
fi
target_id=$(stat -c '%d:%i' -- "$parent_fd/$target_name" 2>/dev/null || true)
if [ "$target_id" != "$candidate_id" ]; then
	if [ -n "$target_id" ] && [ ! -e "$parent_fd/$candidate_name" ] && [ ! -L "$parent_fd/$candidate_name" ] &&
		mv -T --no-copy --update=none-fail -- "$parent_fd/$target_name" "$parent_fd/$candidate_name"; then
		restored_id=$(stat -c '%d:%i' -- "$parent_fd/$candidate_name" 2>/dev/null || true)
		if [ "$restored_id" = "$target_id" ]; then
			printf '%s\n' "unexpected secrets candidate restored at $parent_display/$candidate_name; expected recovery retained through pinned descriptor $candidate_fd" >&2
			exit 1
		fi
	fi
	printf '%s\n' "secrets target changed after publication; expected tree retained through pinned descriptor $candidate_fd" >&2
	exit 1
fi
if ! visible_parent_matches; then
	if [ ! -e "$parent_fd/$candidate_name" ] && [ ! -L "$parent_fd/$candidate_name" ]; then
		mv -T --no-copy --update=none-fail -- "$parent_fd/$target_name" "$parent_fd/$candidate_name" || true
	fi
	printf '%s\n' "secrets destination root changed after publication; recovery retained through pinned $parent_display" >&2
	exit 1
fi
printf '%s\n' "$candidate_id"
`

//nolint:gocyclo // Directory publication keeps its complete identity chain visible in one transaction.
func writeStagedSecrets(ctx context.Context, runner run.CommandRunner, staging, dest string, layout Layout) error {
	if layout != LayoutThin {
		return nil
	}
	source := filepath.Join(staging, "secrets")
	sourceInfo, err := os.Lstat(source)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if sourceInfo.Mode()&os.ModeSymlink != 0 || !sourceInfo.IsDir() {
		return fmt.Errorf("staged secrets path is not a directory: %s", source)
	}
	target := filepath.Join(dest, "secrets")
	if runner.IsDryRun() {
		if targetInfo, err := os.Lstat(target); err == nil {
			if targetInfo.Mode()&os.ModeSymlink != 0 || !targetInfo.IsDir() {
				return fmt.Errorf("target secrets path is not a directory: %s", target)
			}
			return nil
		} else if !os.IsNotExist(err) {
			return err
		}
		fmt.Println("write secrets/")
		return nil
	}
	sourcePinned, err := openPinnedDirectoryPath(source, "staged secrets")
	if err != nil {
		return err
	}
	defer closeFile(sourcePinned.file)
	root, err := openPinnedParentDirectory(target, "secrets root")
	if err != nil {
		return err
	}
	defer closeFile(root)
	existing, exists, err := openPinnedDirectoryEntryAt(root, filepath.Base(target), target, "secrets")
	if err != nil {
		if _, lstatErr := os.Lstat(target); lstatErr == nil {
			return fmt.Errorf("target secrets path is not a directory: %s", target)
		}
		return err
	}
	if existing != nil {
		defer closeFile(existing.file)
	}
	if exists {
		if err := verifyPinnedDirectoryEntry(root, filepath.Base(target), target, "secrets", existing); err != nil {
			return err
		}
		return verifyPinnedDirectoryVisible(root, dest, "secrets root")
	}
	publishCtx, cancel := cleanupContext(ctx)
	defer cancel()
	pythonPath, err := privilegedPythonPath()
	if err != nil {
		return err
	}
	token, err := runner.Output(publishCtx, "sudo", "sh", "-c", privilegedPublishSecretsScript, "--",
		fileDescriptorPath(root), fileDescriptorPath(sourcePinned.file), dest, pythonPath)
	runtime.KeepAlive(root)
	runtime.KeepAlive(sourcePinned.file)
	if err != nil {
		return fmt.Errorf("publish secrets directory: %w", err)
	}
	published, exists, err := openPinnedDirectoryEntryAt(root, filepath.Base(target), target, "secrets")
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("published secrets directory is absent: %s", target)
	}
	defer closeFile(published.file)
	publishedFD, err := checkedFileDescriptor(published.file, "published secrets")
	if err != nil {
		return err
	}
	var stat unix.Stat_t
	if err := unix.Fstat(publishedFD, &stat); err != nil {
		return fmt.Errorf("stat published secrets directory: %w", err)
	}
	if got := fmt.Sprintf("%d:%d", stat.Dev, stat.Ino); token != got {
		return fmt.Errorf("published secrets directory changed before pin: %s", target)
	}
	if err := verifyPinnedDirectoryEntry(root, filepath.Base(target), target, "secrets", published); err != nil {
		return err
	}
	if err := verifyPinnedDirectoryVisible(root, dest, "secrets root"); err != nil {
		return err
	}
	if published.info.Mode().Perm() != 0o700 {
		return fmt.Errorf("published secrets directory mode = %04o, want 0700: %s", published.info.Mode().Perm(), target)
	}
	return nil
}
