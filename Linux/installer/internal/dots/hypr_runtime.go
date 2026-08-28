package dots

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	migrationv1tov2 "github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/migrations/v1_to_v2"
	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/shellruntime"
	"golang.org/x/sys/unix"
)

const (
	runtimeMutationRemove = "remove"
	runtimeMutationWrite  = "write"
)

type runtimeMutationHook func(operation, path string) error

type runtimeJournalStage string

const (
	runtimeJournalDirectory runtimeJournalStage = "directory"
	runtimeJournalMutation  runtimeJournalStage = "mutation"
)

type runtimeJournalHook func(stage runtimeJournalStage, path string) error

type runtimeBeginHook func(path string) error

type runtimePublication struct {
	path    string
	content string
	mode    os.FileMode
}

type runtimeTransactionPlan struct {
	removals     []string
	publications []runtimePublication
}

type hyprUserRuntimeTransition struct {
	targets            []string
	directEnd4Profile  string
	directEnd4Deferred bool
}

func publishHyprUserNamespaceTransition(home, hyprDir, dotsSource string) (hyprUserRuntimeTransition, error) {
	var result hyprUserRuntimeTransition
	transitionEntrypoint := migrationv1tov2.UserNamespaceTransitionEntrypoint()
	if !hyprUserRuntimePublicationRelevant(hyprDir, transitionEntrypoint) {
		return result, nil
	}
	var err error
	result, err = managedUserNamespaceRuntimeTargets(home, hyprDir)
	if err != nil {
		return result, err
	}
	if result.directEnd4Profile != "" {
		deferred, err := prepareDirectEnd4RuntimeMigration(home, hyprDir, dotsSource, result.directEnd4Profile)
		if err != nil {
			return result, err
		}
		result.directEnd4Deferred = deferred
	}
	if err := publishHyprUserNamespaceRuntimeTargets(result.targets, transitionEntrypoint, nil); err != nil {
		return result, err
	}
	return result, nil
}

func finalizeHyprUserNamespaceRuntime(
	home, hyprDir, dotsSource string,
	transition hyprUserRuntimeTransition,
	hook runtimeMutationHook,
) error {
	if transition.directEnd4Profile != "" {
		if transition.directEnd4Deferred {
			return nil
		}
		profile, _ := shellruntime.ProfileByID(transition.directEnd4Profile)
		if err := publishDirectEnd4MigrationAssets(home, hyprDir, dotsSource, "user"); err != nil {
			return err
		}
		if _, err := validateRuntimeShellAssets(hyprDir, profile.ID); err != nil {
			return err
		}
		return writeHyprRuntimeShellStateForDirectMigrationWithHook(home, hyprDir, transition.directEnd4Profile, hook)
	}
	if !hyprUserRuntimePublicationRelevant(hyprDir, shellruntime.CanonicalEntrypoint()) {
		return nil
	}
	return publishHyprUserNamespaceRuntimeTargets(transition.targets, shellruntime.CanonicalEntrypoint(), hook)
}

func hyprUserRuntimePublicationRelevant(hyprDir, desired string) bool {
	canonicalReadable, legacyReadable := readableHyprUserAdapters(hyprDir)
	if desired == migrationv1tov2.UserNamespaceTransitionEntrypoint() {
		return canonicalReadable != legacyReadable
	}
	return canonicalReadable && !legacyReadable
}

func publishHyprUserNamespaceRuntimeTargets(targets []string, desired string, hook runtimeMutationHook) (resultErr error) {
	plan := runtimeTransactionPlan{publications: make([]runtimePublication, 0, len(targets))}
	for _, path := range targets {
		plan.publications = append(plan.publications, runtimePublication{path: path, content: desired, mode: 0o644})
	}

	tx, err := beginRuntimeFileTransaction(targets, hook)
	if err != nil {
		return err
	}
	defer func() {
		resultErr = tx.finish(resultErr)
	}()
	if err := tx.pinPublicationParents(plan); err != nil {
		return err
	}
	for _, publication := range plan.publications {
		state := tx.states[publication.path]
		if state.snapshot.kind != runtimeSnapshotRegular || !isManagedUserNamespaceRuntimeContent(string(state.snapshot.content)) {
			return fmt.Errorf("managed Hypr user namespace runtime changed before publication: %s", publication.path)
		}
	}
	for _, publication := range plan.publications {
		publication := publication
		if string(tx.states[publication.path].snapshot.content) == desired {
			continue
		}
		if err := tx.mutate(runtimeMutationWrite, publication.path, func() (runtimePathState, error) {
			return publishRuntimeRegularFile(
				publication.path,
				[]byte(publication.content),
				publication.mode,
				tx.states[publication.path],
				tx.parents[filepath.Dir(publication.path)],
			)
		}); err != nil {
			return err
		}
	}
	if err := tx.verifyOwnedResults(); err != nil {
		return err
	}
	for _, path := range targets {
		if _, mutated := tx.owned[path]; mutated {
			continue
		}
		current, err := tx.currentRuntimeState(path)
		if err != nil || !sameRuntimePathState(tx.states[path], current) {
			return fmt.Errorf("managed Hypr user namespace runtime changed during publication: %s", path)
		}
	}
	return nil
}

func managedUserNamespaceRuntimeTargets(home, hyprDir string) (hyprUserRuntimeTransition, error) {
	var result hyprUserRuntimeTransition
	directEnd4 := false
	stateRuntime := shellruntime.RuntimeFile(home, "hyprland.lua")
	state, err := snapshotRuntimePathState(stateRuntime)
	if err != nil {
		return result, err
	}
	switch {
	case state.snapshot.kind == runtimeSnapshotAbsent:
	case state.snapshot.kind == runtimeSnapshotRegular && isDirectEnd4RuntimeContent(string(state.snapshot.content)):
		directEnd4 = true
	case state.snapshot.kind == runtimeSnapshotRegular && isManagedUserNamespaceRuntimeContent(string(state.snapshot.content)):
		result.targets = append(result.targets, stateRuntime)
	default:
		return result, fmt.Errorf("unowned Hypr state runtime collision: %s", stateRuntime)
	}

	topLevel := filepath.Join(hyprDir, "hyprland.lua")
	top, err := snapshotRuntimePathState(topLevel)
	if err != nil {
		return result, err
	}
	switch {
	case top.snapshot.kind == runtimeSnapshotAbsent:
	case top.snapshot.kind == runtimeSnapshotRegular && isDirectEnd4RuntimeContent(string(top.snapshot.content)):
		directEnd4 = true
	case top.snapshot.kind == runtimeSnapshotRegular && isManagedUserNamespaceRuntimeContent(string(top.snapshot.content)):
		result.targets = append(result.targets, topLevel)
	case supportedStableTopLevelRuntime(home, topLevel, top):
	default:
		return result, fmt.Errorf("unowned top-level Hypr runtime collision: %s", topLevel)
	}
	if directEnd4 {
		result.directEnd4Profile = shellruntime.ReadEnd4Variant(shellruntime.End4VariantStatePath(home))
	}
	return result, nil
}

func isDirectEnd4RuntimeContent(content string) bool {
	kind := migrationv1tov2.RecognizeEntrypoint(content, shellruntime.DefaultProfile)
	return kind == migrationv1tov2.EntrypointDirectEnd4 || kind == migrationv1tov2.EntrypointDirectEnd4PC
}

func prepareDirectEnd4RuntimeMigration(home, hyprDir, dotsSource, profileID string) (bool, error) {
	profile, ok := shellruntime.ProfileByID(profileID)
	if !ok || !shellruntime.IsEnd4Profile(profile.ID) {
		return false, fmt.Errorf("unknown direct End4 migration profile: %s", profileID)
	}
	plan := buildRuntimeTransactionPlan(home, hyprDir, profile)
	for _, path := range plan.removals {
		state, err := snapshotRuntimePathState(path)
		if err != nil {
			return false, err
		}
		if err := validateLegacyRuntimeFileState(path, state, home, hyprDir); err != nil {
			return false, err
		}
	}
	for _, publication := range plan.publications {
		state, err := snapshotRuntimePathState(publication.path)
		if err != nil {
			return false, err
		}
		if err := validateDirectEnd4RuntimePublicationState(home, hyprDir, publication, state); err != nil {
			return false, err
		}
	}
	externalDeferred, err := validateDirectEnd4ExternalAssets(hyprDir, dotsSource, profile)
	if err != nil {
		return false, err
	}
	namespace := "user"
	if _, legacyReadable := readableHyprUserAdapters(hyprDir); legacyReadable {
		namespace = "wahrwelt"
	}
	assetDeferred, err := preflightDirectEnd4MigrationAssets(home, hyprDir, dotsSource, namespace)
	return externalDeferred || assetDeferred, err
}

func preflightDirectEnd4MigrationAssets(home, hyprDir, dotsSource, namespace string) (bool, error) {
	plan, assets, err := directEnd4MigrationAssetPlan(hyprDir, dotsSource, namespace)
	if err != nil {
		return false, err
	}
	deferred := false
	for _, publication := range plan.publications {
		state, err := snapshotRuntimePathState(publication.path)
		if err != nil {
			return false, err
		}
		assetDeferred, err := validateDirectEnd4MigrationAssetState(home, publication, state, assets[publication.path])
		if err != nil {
			return false, err
		}
		deferred = deferred || assetDeferred
	}
	return deferred, nil
}

type directEnd4MigrationAsset struct {
	sourceRel string
	destRel   string
	mode      os.FileMode
}

var directEnd4MigrationBaseAssetSpecs = []directEnd4MigrationAsset{
	{sourceRel: "hyprland.lua", destRel: "user/hyprland.lua", mode: 0o644},
	{sourceRel: "lib/wahrwelt.lua", destRel: "lib/wahrwelt.lua", mode: 0o644},
	{sourceRel: "variables.lua", destRel: "variables.lua", mode: 0o644},
	{sourceRel: "scheme/default.lua", destRel: "scheme/default.lua", mode: 0o644},
	{sourceRel: "hyprland/animations.lua", destRel: "hyprland/animations.lua", mode: 0o644},
	{sourceRel: "hyprland/decoration.lua", destRel: "hyprland/decoration.lua", mode: 0o644},
	{sourceRel: "hyprland/env.lua", destRel: "hyprland/env.lua", mode: 0o644},
	{sourceRel: "hyprland/execs.lua", destRel: "hyprland/execs.lua", mode: 0o644},
	{sourceRel: "hyprland/general.lua", destRel: "hyprland/general.lua", mode: 0o644},
	{sourceRel: "hyprland/gestures.lua", destRel: "hyprland/gestures.lua", mode: 0o644},
	{sourceRel: "hyprland/group.lua", destRel: "hyprland/group.lua", mode: 0o644},
	{sourceRel: "hyprland/input.lua", destRel: "hyprland/input.lua", mode: 0o644},
	{sourceRel: "hyprland/keybinds.lua", destRel: "hyprland/keybinds.lua", mode: 0o644},
	{sourceRel: "hyprland/misc.lua", destRel: "hyprland/misc.lua", mode: 0o644},
	{sourceRel: "hyprland/rules.lua", destRel: "hyprland/rules.lua", mode: 0o644},
	{sourceRel: "hyprland/scrolling.lua", destRel: "hyprland/scrolling.lua", mode: 0o644},
	{sourceRel: "shell-common-keybinds.lua", destRel: "shell-common-keybinds.lua", mode: 0o644},
	{sourceRel: "shell-common-rules.lua", destRel: "shell-common-rules.lua", mode: 0o644},
	{sourceRel: "shell-workspace-keybinds.lua", destRel: "shell-workspace-keybinds.lua", mode: 0o644},
	{sourceRel: "vm-keybinds.lua", destRel: "vm-keybinds.lua", mode: 0o644},
	{sourceRel: "end4-adapter.lua", destRel: "end4-adapter.lua", mode: 0o644},
}

var directEnd4MigrationAssetSpecs, directEnd4MigrationAssetSpecsErr = loadDirectEnd4MigrationAssetSpecs()

func loadDirectEnd4MigrationAssetSpecs() ([]directEnd4MigrationAsset, error) {
	if err := shellruntime.ManifestError(); err != nil {
		return nil, fmt.Errorf("shell runtime manifest is invalid: %w", err)
	}
	return buildDirectEnd4MigrationAssetSpecs(shellruntime.ProfileSpecs, shellruntime.HyprScripts)
}

func buildDirectEnd4MigrationAssetSpecs(profiles []shellruntime.Profile, scripts []string) ([]directEnd4MigrationAsset, error) {
	capacity := len(directEnd4MigrationBaseAssetSpecs) + 2*len(profiles) + len(scripts)
	specs := make([]directEnd4MigrationAsset, 0, capacity)
	sources := make(map[string]bool, capacity)
	destinations := make(map[string]string, capacity)
	appendAsset := func(asset directEnd4MigrationAsset) error {
		if !safeDirectEnd4MigrationAssetPath(asset.sourceRel) || !safeDirectEnd4MigrationAssetPath(asset.destRel) {
			return fmt.Errorf("unsafe direct End4 migration asset path: source=%q destination=%q", asset.sourceRel, asset.destRel)
		}
		if sources[asset.sourceRel] {
			return fmt.Errorf("duplicate direct End4 migration asset source: %s", asset.sourceRel)
		}
		if previous, exists := destinations[asset.destRel]; exists {
			return fmt.Errorf("duplicate direct End4 migration asset destination %s from %s and %s", asset.destRel, previous, asset.sourceRel)
		}
		sources[asset.sourceRel] = true
		destinations[asset.destRel] = asset.sourceRel
		specs = append(specs, asset)
		return nil
	}
	for _, asset := range directEnd4MigrationBaseAssetSpecs {
		if err := appendAsset(asset); err != nil {
			return nil, err
		}
	}
	for _, profile := range profiles {
		if profile.Family == shellruntime.End4Family {
			continue
		}
		for _, rel := range []string{profile.Launcher, profile.Keybinds} {
			if err := appendAsset(directEnd4MigrationAsset{sourceRel: rel, destRel: rel, mode: 0o644}); err != nil {
				return nil, fmt.Errorf("profile %s: %w", profile.ID, err)
			}
		}
	}
	for _, script := range scripts {
		if !safeDirectEnd4MigrationAssetPath(script) {
			return nil, fmt.Errorf("unsafe managed Hypr script path: %q", script)
		}
		rel := filepath.ToSlash(filepath.Join("scripts", filepath.FromSlash(script)))
		if err := appendAsset(directEnd4MigrationAsset{sourceRel: rel, destRel: rel, mode: 0o755}); err != nil {
			return nil, err
		}
	}
	return specs, nil
}

func safeDirectEnd4MigrationAssetPath(rel string) bool {
	if rel == "" || strings.Contains(rel, `\`) {
		return false
	}
	clean := filepath.Clean(filepath.FromSlash(rel))
	return clean != "." && !filepath.IsAbs(clean) && filepath.ToSlash(clean) == rel &&
		clean != ".." && !strings.HasPrefix(clean, ".."+string(os.PathSeparator))
}

const historicalCanonicalHyprUserAdapter = `local wahrwelt = require("lib.wahrwelt")

require("hyprland.env")
wahrwelt.optional_require("wahrwelt.env")
require("hyprland.general")
require("hyprland.input")
require("hyprland.misc")
require("hyprland.animations")
require("hyprland.decoration")
require("hyprland.group")
require("hyprland.execs")
require("hyprland.rules")
require("hyprland.gestures")
require("hyprland.scrolling")
require("hyprland.keybinds")
require("vm-keybinds")

-- Loaded last, after every bind above is already registered, so overrides in
-- these files can hl.unbind() a default before re-binding it. Each is
-- optional and independently guarded - create only the ones you need.
wahrwelt.optional_require("wahrwelt.execs")
wahrwelt.optional_require("wahrwelt.general")
wahrwelt.optional_require("wahrwelt.rules")
wahrwelt.optional_require("wahrwelt.keybinds")
`

func validateDirectEnd4ExternalAssets(hyprDir, dotsSource string, profile shellruntime.Profile) (bool, error) {
	configDir := filepath.Dir(hyprDir)
	end4Target := filepath.Join(hyprDir, "end4")
	provenSources, err := shellruntime.ProvenEnd4SourcesFromHomeManager(configDir)
	if err != nil {
		return false, err
	}
	if len(provenSources) == 0 {
		return false, fmt.Errorf("unproven direct End4 profile collision: %s", end4Target)
	}
	if err := shellruntime.ValidateEnd4TargetOwnership(end4Target, provenSources); err != nil {
		return false, err
	}
	markerPath := filepath.Join(end4Target, ".wahrwelt-runtime-contract")
	marker, markerInfo, err := readRegularFileNoFollowResolved(markerPath)
	if err != nil {
		if os.IsNotExist(err) {
			return true, nil
		}
		return false, fmt.Errorf("direct End4 runtime contract unreadable: %s: %w", markerPath, err)
	}
	if string(marker) != "end4-adapter-v1\n" || markerInfo.Mode().Perm() != 0o444 {
		return false, fmt.Errorf("invalid direct End4 runtime contract: %s", markerPath)
	}
	type asset struct {
		label string
		path  string
	}
	assets := []asset{
		{label: "end4 shell launcher", path: filepath.Join(hyprDir, filepath.FromSlash(profile.Launcher))},
		{label: "end4 Hyprland entrypoint", path: filepath.Join(hyprDir, "end4", "hyprland.lua")},
		{label: "end4 hyprlock entrypoint", path: filepath.Join(hyprDir, "end4", "hyprlock.conf")},
		{label: "end4 hypridle entrypoint", path: filepath.Join(hyprDir, "end4", "hypridle.conf")},
		{label: "end4 quickshell config", path: filepath.Join(filepath.Dir(hyprDir), "quickshell", profile.QuickshellConfig, "shell.qml")},
	}
	for _, candidate := range assets {
		info, err := os.Stat(candidate.path)
		if err != nil {
			if os.IsNotExist(err) {
				return false, fmt.Errorf("%s missing for profile %s: %s", candidate.label, profile.ID, candidate.path)
			}
			return false, fmt.Errorf("%s unreadable for profile %s: %s: %w", candidate.label, profile.ID, candidate.path, err)
		}
		if !info.Mode().IsRegular() {
			return false, fmt.Errorf("%s is not a regular file for profile %s: %s", candidate.label, profile.ID, candidate.path)
		}
	}
	launcherSource := filepath.Join(dotsSource, "hypr", filepath.FromSlash(profile.Launcher))
	wantLauncher, _, err := readRegularFileNoFollowResolved(launcherSource)
	if err != nil {
		return false, fmt.Errorf("direct End4 launcher source missing or unreadable: %s: %w", launcherSource, err)
	}
	gotLauncher, _, err := readRegularFileNoFollowResolved(filepath.Join(hyprDir, filepath.FromSlash(profile.Launcher)))
	if err != nil || !bytes.Equal(gotLauncher, wantLauncher) {
		return false, fmt.Errorf("unowned or stale direct End4 launcher collision: %s", filepath.Join(hyprDir, filepath.FromSlash(profile.Launcher)))
	}
	return false, nil
}

func publishDirectEnd4MigrationAssets(home, hyprDir, dotsSource, namespace string) (resultErr error) {
	plan, assets, err := directEnd4MigrationAssetPlan(hyprDir, dotsSource, namespace)
	if err != nil {
		return err
	}
	tx, err := beginRuntimeFileTransaction(plan.paths(), nil)
	if err != nil {
		return err
	}
	defer func() { resultErr = tx.finish(resultErr) }()
	if err := tx.pinPublicationParents(plan); err != nil {
		return err
	}
	if err := validateDirectEnd4MigrationAssetPlan(home, plan, assets, tx); err != nil {
		return err
	}
	if err := applyDirectEnd4MigrationAssetPlan(home, plan, assets, tx); err != nil {
		return err
	}
	return verifyDirectEnd4MigrationAssetPlan(plan, tx)
}

func validateDirectEnd4MigrationAssetPlan(
	home string,
	plan runtimeTransactionPlan,
	assets map[string]directEnd4MigrationAsset,
	tx *runtimeFileTransaction,
) error {
	for _, publication := range plan.publications {
		deferred, err := validateDirectEnd4MigrationAssetState(home, publication, tx.states[publication.path], assets[publication.path])
		if err != nil {
			return err
		}
		if deferred {
			return fmt.Errorf("direct End4 migration asset requires Home Manager activation: %s", publication.path)
		}
	}
	return nil
}

func applyDirectEnd4MigrationAssetPlan(
	home string,
	plan runtimeTransactionPlan,
	assets map[string]directEnd4MigrationAsset,
	tx *runtimeFileTransaction,
) error {
	for _, publication := range plan.publications {
		publication := publication
		state := tx.states[publication.path]
		asset := assets[publication.path]
		if directEnd4MigrationRegularAssetReady(publication, state) ||
			directEnd4MigrationAssetIsHomeManagerSymlink(home, publication, state, asset) {
			continue
		}
		if err := tx.mutate(runtimeMutationWrite, publication.path, func() (runtimePathState, error) {
			return publishRuntimeRegularFile(publication.path, []byte(publication.content), publication.mode, state, tx.parents[filepath.Dir(publication.path)])
		}); err != nil {
			return err
		}
	}
	return nil
}

func verifyDirectEnd4MigrationAssetPlan(plan runtimeTransactionPlan, tx *runtimeFileTransaction) error {
	if err := tx.verifyOwnedResults(); err != nil {
		return err
	}
	for _, publication := range plan.publications {
		if _, mutated := tx.owned[publication.path]; mutated {
			continue
		}
		current, err := tx.currentRuntimeState(publication.path)
		if err != nil || !sameRuntimePathState(tx.states[publication.path], current) {
			return fmt.Errorf("managed direct End4 migration asset changed during publication: %s", publication.path)
		}
	}
	return nil
}

func directEnd4MigrationAssetPlan(hyprDir, dotsSource, namespace string) (runtimeTransactionPlan, map[string]directEnd4MigrationAsset, error) {
	var plan runtimeTransactionPlan
	assets := make(map[string]directEnd4MigrationAsset, len(directEnd4MigrationAssetSpecs))
	if directEnd4MigrationAssetSpecsErr != nil {
		return plan, assets, directEnd4MigrationAssetSpecsErr
	}
	if dotsSource == "" {
		return plan, assets, errors.New("hypr dotfiles source is required for direct End4 migration")
	}
	for _, spec := range directEnd4MigrationAssetSpecs {
		sourcePath := filepath.Join(dotsSource, "hypr", filepath.FromSlash(spec.sourceRel))
		data, _, err := readRegularFileNoFollowResolved(sourcePath)
		if err != nil {
			return plan, assets, fmt.Errorf("direct End4 migration source missing or unreadable: %s: %w", sourcePath, err)
		}
		destRel := spec.destRel
		if destRel == "user/hyprland.lua" {
			destRel = namespace + "/hyprland.lua"
		}
		destination := filepath.Join(hyprDir, filepath.FromSlash(destRel))
		plan.publications = append(plan.publications, runtimePublication{path: destination, content: string(data), mode: spec.mode})
		assets[destination] = spec
	}
	return plan, assets, nil
}

func validateDirectEnd4MigrationAssetState(home string, publication runtimePublication, state runtimePathState, asset directEnd4MigrationAsset) (bool, error) {
	if state.snapshot.kind == runtimeSnapshotAbsent || directEnd4MigrationRegularAssetReady(publication, state) {
		return false, nil
	}
	if state.snapshot.kind == runtimeSnapshotSymlink {
		content, active := directEnd4MigrationHomeManagerSymlinkContent(home, publication, state, asset)
		if !active {
			return false, fmt.Errorf("unowned direct End4 migration asset collision: %s", publication.path)
		}
		if content == publication.content {
			return false, nil
		}
		if isHistoricalDirectEnd4MigrationAsset(asset.sourceRel, []byte(content)) {
			return true, nil
		}
		return false, fmt.Errorf("unowned direct End4 migration asset collision: %s", publication.path)
	}
	if state.snapshot.kind == runtimeSnapshotRegular {
		if string(state.snapshot.content) == publication.content {
			return false, nil
		}
		if isHistoricalDirectEnd4MigrationAsset(asset.sourceRel, state.snapshot.content) {
			return false, nil
		}
	}
	return false, fmt.Errorf("unowned direct End4 migration asset collision: %s", publication.path)
}

func directEnd4MigrationRegularAssetReady(publication runtimePublication, state runtimePathState) bool {
	return state.snapshot.kind == runtimeSnapshotRegular &&
		string(state.snapshot.content) == publication.content &&
		state.snapshot.mode.Perm() == publication.mode.Perm()
}

func isHistoricalDirectEnd4MigrationAsset(sourceRel string, content []byte) bool {
	return migrationv1tov2.IsHistoricalDirectEnd4Asset(sourceRel, content)
}

func directEnd4MigrationAssetIsHomeManagerSymlink(
	home string,
	publication runtimePublication,
	state runtimePathState,
	asset directEnd4MigrationAsset,
) bool {
	content, active := directEnd4MigrationHomeManagerSymlinkContent(home, publication, state, asset)
	return active && content == publication.content
}

func directEnd4MigrationHomeManagerSymlinkContent(
	home string,
	publication runtimePublication,
	state runtimePathState,
	asset directEnd4MigrationAsset,
) (string, bool) {
	if state.snapshot.kind != runtimeSnapshotSymlink {
		return "", false
	}
	hyprRoot := filepath.Join(home, ".config", "hypr")
	rel, err := filepath.Rel(hyprRoot, publication.path)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", false
	}
	target := state.snapshot.linkTarget
	if !filepath.IsAbs(target) {
		return "", false
	}
	candidates := []string{rel}
	if asset.sourceRel == "hyprland.lua" {
		candidates = append(candidates, filepath.Join("user", "hyprland.lua"), filepath.Join("wahrwelt", "hyprland.lua"))
	}
	active := false
	for _, candidate := range candidates {
		if isImmutableNixStoreHomeManagerHyprPath(target, filepath.ToSlash(candidate)) {
			active = true
			break
		}
	}
	if !active {
		return "", false
	}
	data, _, err := readRegularFileNoFollowResolved(target)
	return string(data), err == nil
}

func isManagedUserNamespaceRuntimeContent(content string) bool {
	for _, candidate := range []string{
		shellruntime.CanonicalEntrypoint(),
		shellruntime.HomeManagerInitialCanonicalEntrypoint(),
		migrationv1tov2.UserNamespaceTransitionEntrypoint(),
		migrationv1tov2.LegacyUserEntrypoint(),
		migrationv1tov2.LegacyHomeManagerUserEntrypoint(shellruntime.DefaultProfile),
		migrationv1tov2.HistoricalHomeManagerSeededUserEntrypoint(shellruntime.DefaultProfile, migrationv1tov2.LegacyWahrweltNamespace),
		migrationv1tov2.HistoricalHomeManagerSeededUserEntrypoint(shellruntime.DefaultProfile, migrationv1tov2.CanonicalUserNamespace),
		migrationv1tov2.DirectEnd4Entrypoint(shellruntime.End4),
		migrationv1tov2.DirectEnd4Entrypoint(shellruntime.End4PC),
	} {
		if content == candidate {
			return true
		}
	}
	return false
}

func supportedStableTopLevelRuntime(home, path string, state runtimePathState) bool {
	want := stableRuntimeSourceConfig(shellruntime.RuntimeFile(home, "hyprland.lua"), "Wahrwelt stable Hyprland entrypoint.")
	switch state.snapshot.kind {
	case runtimeSnapshotRegular:
		return string(state.snapshot.content) == want
	case runtimeSnapshotSymlink:
		target, ok := managedHomeManagerTopLevelRuntimeTarget(home, path, state.snapshot.linkTarget)
		if !ok {
			return false
		}
		data, _, err := readRegularFileNoFollowResolved(target)
		return err == nil && string(data) == want
	default:
		return false
	}
}

func readableHyprUserAdapters(hyprDir string) (canonical, legacy bool) {
	return readableRegularRuntimePath(filepath.Join(hyprDir, "user", "hyprland.lua")),
		readableRegularRuntimePath(filepath.Join(hyprDir, "wahrwelt", "hyprland.lua"))
}

func readableRegularRuntimePath(path string) bool {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return false
	}
	fd, err := unix.Open(resolved, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if err != nil {
		return false
	}
	file, err := newFileFromUnixFD(fd, resolved)
	if err != nil {
		_ = unix.Close(fd)
		return false
	}
	defer func() { _ = file.Close() }()
	info, err := file.Stat()
	return err == nil && info.Mode().IsRegular()
}

func writeHyprRuntimeShellState(home, hyprDir string) error {
	return writeHyprRuntimeShellStateWithHook(home, hyprDir, nil)
}

func bootstrapActiveShellForUpgrade(home, hyprDir string) string {
	if profile := shellruntime.ReadActiveShell(shellruntime.ActiveShellStatePath(home)); profile != "" {
		return profile
	}
	legacyState := migrationv1tov2.LegacyActiveShellStatePath(filepath.Join(home, ".local", "state"))
	if profile := shellruntime.ReadActiveShell(legacyState); profile != "" {
		return profile
	}
	variantPath := shellruntime.End4VariantStatePath(home)
	for _, candidate := range []struct {
		entrypoint string
		keybinds   string
	}{
		{shellruntime.RuntimeFile(home, "hyprland.lua"), shellruntime.RuntimeFile(home, "shell-keybinds.lua")},
		{filepath.Join(hyprDir, "hyprland.lua"), filepath.Join(hyprDir, "shell-keybinds.lua")},
	} {
		if profile := shellruntime.DetectShellFromEntrypointWithEnd4Variant(candidate.entrypoint, candidate.keybinds, variantPath); profile != "" {
			return profile
		}
		data, err := os.ReadFile(candidate.entrypoint)
		if err != nil {
			continue
		}
		switch migrationv1tov2.RecognizeEntrypoint(string(data), shellruntime.DefaultProfile) {
		case migrationv1tov2.EntrypointDirectEnd4, migrationv1tov2.EntrypointDirectEnd4PC:
			return shellruntime.ReadEnd4Variant(variantPath)
		case migrationv1tov2.EntrypointLegacyUser,
			migrationv1tov2.EntrypointHomeManagerSeededUser,
			migrationv1tov2.EntrypointNamespaceTransition:
			if profile := shellruntime.DetectShellFromKeybinds(candidate.keybinds); profile != "" {
				return profile
			}
		}
	}
	return shellruntime.DefaultProfile
}

func writeHyprRuntimeShellStateWithHook(home, hyprDir string, hook runtimeMutationHook) (resultErr error) {
	profile := bootstrapActiveShellForUpgrade(home, hyprDir)
	return writeHyprRuntimeShellStateForProfileWithHook(home, hyprDir, profile, hook, false)
}

func writeHyprRuntimeShellStateForDirectMigrationWithHook(home, hyprDir, profile string, hook runtimeMutationHook) error {
	return writeHyprRuntimeShellStateForProfileWithHook(home, hyprDir, profile, hook, true)
}

func writeHyprRuntimeShellStateForProfileWithHook(
	home, hyprDir, profile string,
	hook runtimeMutationHook,
	strictDirectMigration bool,
) (resultErr error) {
	profileSpec, err := validateRuntimeShellAssets(hyprDir, profile)
	if err != nil {
		return err
	}
	plan := buildRuntimeTransactionPlan(home, hyprDir, profileSpec)
	tx, err := beginRuntimeFileTransaction(plan.paths(), hook)
	if err != nil {
		return err
	}
	defer func() {
		resultErr = tx.finish(resultErr)
	}()
	if err := tx.pinPublicationParents(plan); err != nil {
		return err
	}
	preserved, err := validateRuntimeTransactionPlan(plan, home, hyprDir, tx, map[bool]string{true: profile}[strictDirectMigration])
	if err != nil {
		return err
	}
	for _, path := range plan.removals {
		if err := tx.mutate(runtimeMutationRemove, path, func() (runtimePathState, error) {
			return removeLegacyRuntimeFileWithState(path, tx.states[path], tx.parents[filepath.Dir(path)], home, hyprDir)
		}); err != nil {
			return err
		}
	}
	for _, publication := range plan.publications {
		publication := publication
		if _, ok := preserved[publication.path]; ok {
			continue
		}
		if err := tx.mutate(runtimeMutationWrite, publication.path, func() (runtimePathState, error) {
			return publishRuntimeRegularFile(publication.path, []byte(publication.content), publication.mode, tx.states[publication.path], tx.parents[filepath.Dir(publication.path)])
		}); err != nil {
			return err
		}
	}
	if err := tx.verifyOwnedResults(); err != nil {
		return err
	}
	if err := verifyPreservedHomeManagerRuntimePublications(home, plan, preserved, tx); err != nil {
		return err
	}
	return nil
}

func verifyPreservedHomeManagerRuntimePublications(
	home string,
	plan runtimeTransactionPlan,
	preserved map[string]runtimePathState,
	tx *runtimeFileTransaction,
) error {
	for _, publication := range plan.publications {
		expected, ok := preserved[publication.path]
		if !ok {
			continue
		}
		if !runtimePublicationIsPreservedHomeManagerSymlink(home, publication, expected) {
			return fmt.Errorf("preserved Home Manager shell runtime changed before commit: %s", publication.path)
		}
		current, err := tx.currentRuntimeState(publication.path)
		if err != nil || !sameRuntimePathState(expected, current) {
			return fmt.Errorf("preserved Home Manager shell runtime changed before commit: %s", publication.path)
		}
	}
	return nil
}

func buildRuntimeTransactionPlan(home, hyprDir string, profile shellruntime.Profile) runtimeTransactionPlan {
	plan := runtimeTransactionPlan{removals: legacyHyprlandRuntimePaths(home, hyprDir)}
	runtimeDir := shellruntime.RuntimeDir(home)
	var mainPublications []runtimePublication
	for _, name := range shellruntime.RuntimeFiles {
		comment := ""
		if name == "hyprland.lua" {
			comment = "Wahrwelt stable Hyprland entrypoint."
		}
		publication := runtimePublication{
			path:    filepath.Join(hyprDir, name),
			content: stableRuntimeSourceConfig(filepath.Join(runtimeDir, name), comment),
			mode:    0o644,
		}
		if name == "hyprland.lua" {
			mainPublications = append(mainPublications, publication)
			continue
		}
		plan.publications = append(plan.publications, publication)
	}
	plan.publications = append(plan.publications,
		runtimePublication{path: shellruntime.RuntimeFile(home, "shell-profile.lua"), content: shellLauncherConfig(hyprDir), mode: 0o644},
		runtimePublication{path: shellruntime.RuntimeFile(home, "shell-launcher.lua"), content: shellLauncherBindingsConfig(profile), mode: 0o644},
		runtimePublication{path: shellruntime.RuntimeFile(home, "shell-keybinds.lua"), content: shellKeybindsConfig(hyprDir, profile), mode: 0o644},
	)
	plan.publications = append(plan.publications, runtimeLockStackPublications(home, hyprDir, profile.ID)...)
	plan.publications = append(plan.publications,
		runtimePublication{path: shellruntime.RuntimeFile(home, "hyprland.lua"), content: shellruntime.CanonicalEntrypoint(), mode: 0o644},
	)
	plan.publications = append(plan.publications, mainPublications...)
	return plan
}

func (plan runtimeTransactionPlan) paths() []string {
	paths := make([]string, 0, len(plan.removals)+len(plan.publications))
	paths = append(paths, plan.removals...)
	for _, publication := range plan.publications {
		paths = append(paths, publication.path)
	}
	return paths
}

func validateRuntimeShellAssets(hyprDir, profileID string) (shellruntime.Profile, error) {
	return validateRuntimeShellAssetsWithUserEntrypoint(hyprDir, profileID, filepath.Join(hyprDir, "user", "hyprland.lua"))
}

func validateRuntimeShellAssetsWithUserEntrypoint(hyprDir, profileID, userEntrypoint string) (shellruntime.Profile, error) {
	profile, ok := shellruntime.ProfileByID(profileID)
	if !ok {
		return shellruntime.Profile{}, fmt.Errorf("unknown shell runtime profile: %s", profileID)
	}

	type runtimeAsset struct {
		label string
		path  string
	}
	required := []runtimeAsset{
		{label: "canonical Hyprland entrypoint", path: userEntrypoint},
		{label: "shell lifecycle launcher", path: filepath.Join(hyprDir, "scripts", "start-shell.sh")},
		{label: "shell launcher profile", path: filepath.Join(hyprDir, filepath.FromSlash(profile.Launcher))},
	}
	if shellruntime.IsEnd4Profile(profile.ID) {
		required = append(required,
			runtimeAsset{label: "end4 shell adapter", path: filepath.Join(hyprDir, "end4-adapter.lua")},
			runtimeAsset{label: "end4 Hyprland entrypoint", path: filepath.Join(hyprDir, "end4", "hyprland.lua")},
			runtimeAsset{label: "end4 hyprlock entrypoint", path: filepath.Join(hyprDir, "end4", "hyprlock.conf")},
			runtimeAsset{label: "end4 hypridle entrypoint", path: filepath.Join(hyprDir, "end4", "hypridle.conf")},
			runtimeAsset{label: "end4 quickshell config", path: filepath.Join(filepath.Dir(hyprDir), "quickshell", profile.QuickshellConfig, "shell.qml")},
		)
	} else {
		required = append(required, runtimeAsset{label: "shell keybind profile", path: filepath.Join(hyprDir, filepath.FromSlash(profile.Keybinds))})
	}

	for _, asset := range required {
		info, err := os.Stat(asset.path)
		if err != nil {
			if os.IsNotExist(err) {
				return shellruntime.Profile{}, fmt.Errorf("%s missing for profile %s: %s", asset.label, profile.ID, asset.path)
			}
			return shellruntime.Profile{}, fmt.Errorf("%s unreadable for profile %s: %s: %w", asset.label, profile.ID, asset.path, err)
		}
		if !info.Mode().IsRegular() {
			return shellruntime.Profile{}, fmt.Errorf("%s is not a regular file for profile %s: %s", asset.label, profile.ID, asset.path)
		}
	}

	return profile, nil
}

func legacyHyprlandRuntimePaths(home, hyprDir string) []string {
	return []string{
		filepath.Join(hyprDir, "hyprland.conf"),
		filepath.Join(hyprDir, "shell-profile.conf"),
		filepath.Join(hyprDir, "shell-launcher.conf"),
		filepath.Join(hyprDir, "shell-keybinds.conf"),
		filepath.Join(hyprDir, "wahrwelt", "hyprland.conf"),
		shellruntime.RuntimeFile(home, "hyprland.conf"),
		shellruntime.RuntimeFile(home, "shell-profile.conf"),
		shellruntime.RuntimeFile(home, "shell-launcher.conf"),
		shellruntime.RuntimeFile(home, "shell-keybinds.conf"),
	}
}

func removeLegacyRuntimeFile(path string) error {
	state, err := snapshotRuntimePathState(path)
	if err != nil {
		return err
	}
	if state.snapshot.kind == runtimeSnapshotAbsent {
		return nil
	}
	return fmt.Errorf("unowned legacy Hyprland runtime collision: %s", path)
}

func validateRuntimeTransactionPlan(
	plan runtimeTransactionPlan,
	home, hyprDir string,
	tx *runtimeFileTransaction,
	directProfile string,
) (map[string]runtimePathState, error) {
	preserved := make(map[string]runtimePathState)
	for _, path := range plan.removals {
		if err := validateLegacyRuntimeFileState(path, tx.states[path], home, hyprDir); err != nil {
			return nil, err
		}
	}
	for _, publication := range plan.publications {
		state := tx.states[publication.path]
		if runtimePublicationIsPreservedHomeManagerSymlink(home, publication, state) {
			preserved[publication.path] = state
			continue
		}
		if directProfile != "" {
			if err := validateDirectEnd4RuntimePublicationState(home, hyprDir, publication, state); err != nil {
				return nil, err
			}
			continue
		}
		if publication.path == filepath.Join(hyprDir, "hyprland.lua") && !isKnownTopLevelRuntimeEntrypoint(state.snapshot, home) {
			return nil, fmt.Errorf("unowned top-level Hyprland runtime collision: %s", publication.path)
		}
		if state.snapshot.kind == runtimeSnapshotSymlink {
			return nil, fmt.Errorf("refusing symlink shell runtime publication collision: %s", publication.path)
		}
	}
	return preserved, nil
}

func validateDirectEnd4RuntimePublicationState(
	home, hyprDir string,
	publication runtimePublication,
	state runtimePathState,
) error {
	if state.snapshot.kind == runtimeSnapshotAbsent {
		return nil
	}
	if runtimePublicationIsPreservedHomeManagerSymlink(home, publication, state) {
		return nil
	}
	if state.snapshot.kind != runtimeSnapshotRegular {
		return fmt.Errorf("unowned direct End4 ancillary runtime collision: %s", publication.path)
	}
	content := string(state.snapshot.content)
	if content == publication.content {
		return nil
	}
	if publication.path == shellruntime.RuntimeFile(home, "hyprland.lua") || publication.path == filepath.Join(hyprDir, "hyprland.lua") {
		if isManagedUserNamespaceRuntimeContent(content) || content == stableRuntimeSourceConfig(shellruntime.RuntimeFile(home, "hyprland.lua"), "Wahrwelt stable Hyprland entrypoint.") {
			return nil
		}
		return fmt.Errorf("unowned direct End4 main runtime collision: %s", publication.path)
	}
	if publication.path == shellruntime.RuntimeFile(home, "shell-launcher.lua") && isLegacyDirectEnd4LauncherPlaceholder(content) {
		return nil
	}
	if publication.path == shellruntime.RuntimeFile(home, "shell-keybinds.lua") && isLegacyDirectEnd4KeybindsPlaceholder(content) {
		return nil
	}
	if isHistoricalEnd4SharedRuntimePayload(home, hyprDir, publication.path, content) {
		return nil
	}
	return fmt.Errorf("unowned direct End4 ancillary runtime collision: %s", publication.path)
}

func isHistoricalEnd4SharedRuntimePayload(home, hyprDir, path, content string) bool {
	for _, profileID := range []string{shellruntime.End4, shellruntime.End4PC} {
		var candidate string
		switch path {
		case shellruntime.RuntimeFile(home, "shell-launcher.lua"):
			candidate = fmt.Sprintf("-- Active shell launcher profile: %s\nrequire(%q)\n", profileID, "end4.launcher")
		case shellruntime.RuntimeFile(home, "hyprlock.conf"):
			candidate = runtimeSourceConfig(filepath.Join(hyprDir, "end4", "hyprlock.conf"), "Active Hyprlock profile: "+profileID)
		case shellruntime.RuntimeFile(home, "hypridle.conf"):
			candidate = runtimeSourceConfig(filepath.Join(hyprDir, "end4", "hypridle.conf"), "Active Hypridle profile: "+profileID)
		default:
			return false
		}
		if content == candidate {
			return true
		}
	}
	return false
}

func runtimePublicationIsPreservedHomeManagerSymlink(home string, publication runtimePublication, state runtimePathState) bool {
	if state.snapshot.kind != runtimeSnapshotSymlink {
		return false
	}
	target, ok := managedHomeManagerTopLevelRuntimeTarget(home, publication.path, state.snapshot.linkTarget)
	if !ok {
		return false
	}
	data, _, err := readRegularFileNoFollowResolved(target)
	return err == nil && string(data) == publication.content
}

func managedHomeManagerTopLevelRuntimeTarget(home, path, rawTarget string) (string, bool) {
	if filepath.Dir(path) != filepath.Join(home, ".config", "hypr") {
		return "", false
	}
	name := filepath.Base(path)
	managedName := false
	for _, candidate := range shellruntime.RuntimeFiles {
		if name == candidate {
			managedName = true
			break
		}
	}
	if !managedName {
		return "", false
	}
	if filepath.IsAbs(rawTarget) && isImmutableNixStoreHomeManagerHyprTarget(rawTarget, name) {
		return rawTarget, true
	}
	return "", false
}

func validateLegacyRuntimeFileState(path string, state runtimePathState, home, hyprDir string) error {
	if state.snapshot.kind == runtimeSnapshotAbsent {
		return nil
	}
	if isKnownLegacyRuntimeFile(path, state.snapshot, home, hyprDir) {
		return nil
	}
	return fmt.Errorf("unowned legacy Hyprland runtime collision: %s", path)
}

func removeLegacyRuntimeFileWithState(path string, state runtimePathState, directory *pinnedDirectory, home, hyprDir string) (runtimePathState, error) {
	if err := validateLegacyRuntimeFileState(path, state, home, hyprDir); err != nil {
		return runtimePathState{}, err
	}
	if state.snapshot.kind == runtimeSnapshotAbsent {
		return runtimePathState{snapshot: runtimePathSnapshot{path: path, kind: runtimeSnapshotAbsent}}, nil
	}
	quarantine, err := quarantineRuntimePathState(path, state, directory, true)
	owned := runtimePathState{snapshot: runtimePathSnapshot{path: path, kind: runtimeSnapshotAbsent}, quarantine: quarantine}
	return owned, err
}

func isKnownTopLevelRuntimeEntrypoint(snapshot runtimePathSnapshot, home string) bool {
	if snapshot.kind == runtimeSnapshotAbsent {
		return true
	}
	if snapshot.kind != runtimeSnapshotRegular {
		return false
	}
	text := string(snapshot.content)
	for _, candidate := range knownTopLevelRuntimeEntrypoints(home) {
		if text == candidate {
			return true
		}
	}
	return false
}

func knownTopLevelRuntimeEntrypoints(home string) []string {
	runtimeEntrypoint := shellruntime.RuntimeFile(home, "hyprland.lua")
	return []string{
		stableRuntimeSourceConfig(runtimeEntrypoint, "Wahrwelt stable Hyprland entrypoint."),
		shellruntime.CanonicalEntrypoint(),
		shellruntime.HomeManagerInitialCanonicalEntrypoint(),
		migrationv1tov2.UserNamespaceTransitionEntrypoint(),
		migrationv1tov2.HistoricalHomeManagerSeededUserEntrypoint(shellruntime.DefaultProfile, migrationv1tov2.LegacyWahrweltNamespace),
		migrationv1tov2.HistoricalHomeManagerSeededUserEntrypoint(shellruntime.DefaultProfile, migrationv1tov2.CanonicalUserNamespace),
		migrationv1tov2.LegacyUserEntrypoint(),
		migrationv1tov2.LegacyHomeManagerUserEntrypoint(shellruntime.DefaultProfile),
		migrationv1tov2.DirectEnd4Entrypoint(shellruntime.End4),
		migrationv1tov2.DirectEnd4Entrypoint(shellruntime.End4PC),
	}
}

func isLegacyDirectEnd4LauncherPlaceholder(content string) bool {
	return content == migrationv1tov2.DirectEnd4LauncherPlaceholder(shellruntime.End4) ||
		content == migrationv1tov2.DirectEnd4LauncherPlaceholder(shellruntime.End4PC)
}

func isLegacyDirectEnd4KeybindsPlaceholder(content string) bool {
	return content == migrationv1tov2.DirectEnd4KeybindsPlaceholder(shellruntime.End4) ||
		content == migrationv1tov2.DirectEnd4KeybindsPlaceholder(shellruntime.End4PC)
}

func isKnownLegacyRuntimeFile(path string, snapshot runtimePathSnapshot, home, hyprDir string) bool {
	switch snapshot.kind {
	case runtimeSnapshotRegular:
		for _, candidate := range knownLegacyRuntimePayloads(path, home, hyprDir) {
			if bytes.Equal(snapshot.content, []byte(candidate)) {
				return true
			}
		}
		return false
	case runtimeSnapshotSymlink:
		target := snapshot.linkTarget
		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(path), target)
		}
		target = filepath.Clean(target)
		for _, candidate := range legacyHyprlandRuntimePaths(home, hyprDir) {
			if target != candidate || target == path {
				continue
			}
			state, err := snapshotRuntimePathState(target)
			if err == nil && state.snapshot.kind == runtimeSnapshotRegular && isKnownLegacyRuntimeFile(target, state.snapshot, home, hyprDir) {
				return true
			}
		}
		return false
	default:
		return false
	}
}

func knownLegacyRuntimePayloads(path, home, hyprDir string) []string {
	runtimeDir := shellruntime.RuntimeDir(home)
	name := filepath.Base(path)
	profileScript := filepath.Join(hyprDir, "scripts", "start-shell.sh")
	payloads := []string{
		stableRuntimeSourceConfig(filepath.Join(runtimeDir, name), map[bool]string{name == "hyprland.conf": "Wahrwelt stable Hyprland entrypoint."}[name == "hyprland.conf"]),
	}
	if name == "shell-profile.conf" {
		payloads = append(payloads, fmt.Sprintf("# Runtime shell launcher\nexec-once = %s\n", profileScript))
	}
	for _, profile := range shellruntime.ProfileSpecs {
		switch name {
		case "shell-launcher.conf":
			payloads = append(payloads, fmt.Sprintf("# Active shell launcher profile: %s\nsource = %s\n", profile.ID, filepath.Join(hyprDir, profile.ID, "launcher.conf")))
		case "shell-keybinds.conf":
			payloads = append(payloads, fmt.Sprintf("# Active shell keybind profile: %s\nsource = %s\n", profile.ID, filepath.Join(hyprDir, profile.ID, "keybinds.conf")))
			if shellruntime.IsEnd4Profile(profile.ID) {
				payloads = append(payloads, fmt.Sprintf("-- Active shell keybind profile: %s\n-- end4 registers keybinds from its own Hyprland Lua modules.\n", profile.ID))
			}
		case "hyprland.conf":
			for _, namespace := range []string{"mysetup", "wahrwelt"} {
				payloads = append(payloads, fmt.Sprintf("# Active Hyprland profile: %s (%s)\nsource = %s\nsource = %s\n", namespace, profile.ID, filepath.Join(hyprDir, namespace, "hyprland.conf"), filepath.Join(runtimeDir, "shell-profile.conf")))
			}
			if shellruntime.IsEnd4Profile(profile.ID) {
				payloads = append(payloads, fmt.Sprintf("# Active Hyprland profile: %s\nsource = %s\nsource = %s\n", profile.ID, filepath.Join(hyprDir, "end4", "hyprland.conf"), filepath.Join(runtimeDir, "shell-profile.conf")))
			}
		}
	}
	return payloads
}

func runtimeLockStackPublications(home, hyprDir, profile string) []runtimePublication {
	return runtimeLockStackPublicationsForPaths(
		hyprDir,
		shellruntime.RuntimeFile(home, "hyprlock.conf"),
		shellruntime.RuntimeFile(home, "hypridle.conf"),
		profile,
	)
}

func runtimeLockStackPublicationsForPaths(hyprDir, hyprlock, hypridle, profile string) []runtimePublication {
	if !shellruntime.IsEnd4Profile(profile) {
		return []runtimePublication{
			{
				path:    hyprlock,
				content: shellManagedRuntimePlaceholder("Hyprlock", "Caelestia and Noctalia use shell-native lock flows."),
				mode:    0o644,
			},
			{
				path:    hypridle,
				content: shellManagedRuntimePlaceholder("Hypridle", "Caelestia and Noctalia use shell-native idle flows."),
				mode:    0o644,
			},
		}
	}
	return []runtimePublication{
		{
			path:    hyprlock,
			content: runtimeSourceConfig(filepath.Join(hyprDir, "end4", "hyprlock.conf"), "Active Hyprlock profile: end4"),
			mode:    0o644,
		},
		{
			path:    hypridle,
			content: runtimeSourceConfig(filepath.Join(hyprDir, "end4", "hypridle.conf"), "Active Hypridle profile: end4"),
			mode:    0o644,
		},
	}
}

func stableRuntimeSourceConfig(target, comment string) string {
	if strings.HasSuffix(target, ".lua") {
		return fmt.Sprintf(`-- Generated by Wahrwelt: stable Hyprland Lua runtime entrypoint
local home = os.getenv("HOME")
if home == nil then
    error("HOME is not set; cannot locate Wahrwelt Hyprland runtime")
end

local config_home = os.getenv("XDG_CONFIG_HOME") or (home .. "/.config")
local hypr_root = config_home .. "/hypr"
package.path = hypr_root .. "/?.lua;" .. hypr_root .. "/?/init.lua;" .. package.path
dofile(%q)
`, target)
	}
	if comment == "" {
		return fmt.Sprintf("source = %s\n", target)
	}
	return fmt.Sprintf("# %s\nsource = %s\n", comment, target)
}

func writeShellLauncherConfig(path, hyprDir string) error {
	return writeRuntimeConfigFile(path, shellLauncherConfig(hyprDir))
}

func shellLauncherConfig(hyprDir string) string {
	script := filepath.Join(hyprDir, "scripts", "start-shell.sh")
	return fmt.Sprintf(`-- Runtime shell launcher
hl.on("hyprland.start", function()
    hl.exec_cmd(%q)
end)
`, script)
}

func shellKeybindsConfig(hyprDir string, profile shellruntime.Profile) string {
	content := shellruntime.AdapterMarker(profile.ID) + "\n"
	if shellruntime.IsEnd4Profile(profile.ID) {
		quickshellConfig := filepath.Join(filepath.Dir(hyprDir), "quickshell", profile.QuickshellConfig)
		content += fmt.Sprintf("require(%q).load({ profile = %q, quickshell_config = %q })\n", profile.Adapter, profile.ID, quickshellConfig)
	} else {
		content += fmt.Sprintf("require(%q)\n", profile.Adapter)
	}
	return content
}

func shellLauncherBindingsConfig(profile shellruntime.Profile) string {
	module := strings.TrimSuffix(filepath.ToSlash(profile.Launcher), ".lua")
	module = strings.ReplaceAll(module, "/", ".")
	launcherProfile := profile.ID
	if shellruntime.IsEnd4Profile(profile.ID) {
		launcherProfile = shellruntime.End4
	}
	return fmt.Sprintf("-- Active shell launcher profile: %s\nrequire(%q)\n", launcherProfile, module)
}

func runtimeSourceConfig(target, label string) string {
	return fmt.Sprintf("# %s\nsource = %s\n", label, target)
}

func shellManagedRuntimePlaceholder(component, detail string) string {
	return fmt.Sprintf("# Active %s profile: shell-managed\n# %s\n", component, detail)
}

type runtimeSnapshotKind uint8

const (
	runtimeSnapshotAbsent runtimeSnapshotKind = iota
	runtimeSnapshotRegular
	runtimeSnapshotSymlink
)

type runtimePathSnapshot struct {
	path       string
	kind       runtimeSnapshotKind
	mode       os.FileMode
	content    []byte
	linkTarget string
}

type runtimePathState struct {
	snapshot   runtimePathSnapshot
	info       os.FileInfo
	quarantine *runtimePathQuarantine
}

type runtimePathQuarantine struct {
	directory *pinnedDirectory
	name      string
	path      string
	state     runtimePathState
	retained  bool
}

type runtimePathRecovery struct {
	directory *pinnedDirectory
	file      *os.File
	path      string
	retained  string
}

type runtimeCreatedDirectory struct {
	parent    *pinnedDirectory
	directory *pinnedDirectory
	name      string
	path      string
	info      os.FileInfo
}

type runtimeFileTransaction struct {
	snapshots      map[string]runtimePathSnapshot
	states         map[string]runtimePathState
	owned          map[string]runtimePathState
	recovery       map[string]*runtimePathRecovery
	parents        map[string]*pinnedDirectory
	anchors        map[string]*pinnedDirectory
	created        map[string]*runtimeCreatedDirectory
	createdInOrder []*runtimeCreatedDirectory
	applied        []string
	hook           runtimeMutationHook
	journalHook    runtimeJournalHook
	beginHook      runtimeBeginHook
	rolledBack     bool
}

func beginRuntimeFileTransaction(paths []string, hook runtimeMutationHook) (*runtimeFileTransaction, error) {
	return beginRuntimeFileTransactionWithJournalHook(paths, hook, nil)
}

func beginRuntimeFileTransactionWithJournalHook(paths []string, hook runtimeMutationHook, journalHook runtimeJournalHook) (*runtimeFileTransaction, error) {
	return beginRuntimeFileTransactionWithHooks(paths, hook, journalHook, nil)
}

func beginRuntimeFileTransactionWithHooks(paths []string, hook runtimeMutationHook, journalHook runtimeJournalHook, beginHook runtimeBeginHook) (*runtimeFileTransaction, error) {
	tx := &runtimeFileTransaction{
		snapshots:   make(map[string]runtimePathSnapshot, len(paths)),
		states:      make(map[string]runtimePathState, len(paths)),
		owned:       make(map[string]runtimePathState, len(paths)),
		recovery:    make(map[string]*runtimePathRecovery, len(paths)),
		parents:     make(map[string]*pinnedDirectory, len(paths)),
		anchors:     make(map[string]*pinnedDirectory, len(paths)),
		created:     make(map[string]*runtimeCreatedDirectory, len(paths)),
		hook:        hook,
		journalHook: journalHook,
		beginHook:   beginHook,
	}
	for _, path := range paths {
		if _, seen := tx.snapshots[path]; seen {
			continue
		}
		if err := tx.pinRuntimeParentAtBegin(path); err != nil {
			tx.close()
			return nil, err
		}
		if tx.beginHook != nil {
			if err := tx.beginHook(path); err != nil {
				tx.close()
				return nil, err
			}
		}
		state := runtimePathState{snapshot: runtimePathSnapshot{path: path, kind: runtimeSnapshotAbsent}}
		if directory := tx.parents[filepath.Dir(path)]; directory != nil {
			var err error
			state, err = runtimeStateAt(directory, filepath.Base(path), path)
			if err != nil {
				tx.close()
				return nil, err
			}
		}
		if err := tx.anchors[filepath.Dir(path)].checkCanonical(); err != nil {
			tx.close()
			return nil, err
		}
		tx.snapshots[path] = state.snapshot
		tx.states[path] = state
		recovery, err := createRuntimePathRecovery(state, tx.parents[filepath.Dir(path)])
		if err != nil {
			tx.close()
			return nil, err
		}
		if recovery != nil {
			tx.recovery[path] = recovery
		}
	}
	return tx, nil
}

func (tx *runtimeFileTransaction) pinRuntimeParentAtBegin(path string) error {
	dir := filepath.Dir(path)
	if _, ok := tx.anchors[dir]; ok {
		return nil
	}
	anchor, exact, err := openNearestPinnedRuntimeDirectory(dir)
	if err != nil {
		return err
	}
	tx.anchors[dir] = anchor
	if exact {
		tx.parents[dir] = anchor
	}
	return nil
}

func (tx *runtimeFileTransaction) pinPublicationParents(plan runtimeTransactionPlan) error {
	for _, publication := range plan.publications {
		path := publication.path
		dir := filepath.Dir(path)
		if _, ok := tx.parents[dir]; !ok {
			anchor := tx.anchors[dir]
			if anchor == nil {
				return fmt.Errorf("shell runtime parent was not anchored at begin: %s", dir)
			}
			directory, err := tx.createPinnedRuntimeDescendant(anchor, dir)
			if err != nil {
				return err
			}
			tx.parents[dir] = directory
		}
	}
	for _, directory := range tx.parents {
		if directory != nil {
			if err := directory.checkCanonical(); err != nil {
				return err
			}
		}
	}
	for path, expected := range tx.states {
		current, err := tx.currentRuntimeState(path)
		if err != nil || !sameRuntimePathState(expected, current) {
			return fmt.Errorf("shell runtime path changed while pinning parent: %s", path)
		}
	}
	return nil
}

func (tx *runtimeFileTransaction) currentRuntimeState(path string) (runtimePathState, error) {
	if directory := tx.parents[filepath.Dir(path)]; directory != nil {
		return runtimeStateAt(directory, filepath.Base(path), path)
	}
	anchor := tx.anchors[filepath.Dir(path)]
	if anchor == nil {
		return runtimePathState{}, fmt.Errorf("shell runtime path has no begin-time anchor: %s", path)
	}
	return runtimeStateBeneathAnchor(anchor, path)
}

func (tx *runtimeFileTransaction) verifyOwnedResults() error {
	for path, expected := range tx.owned {
		current, err := tx.currentRuntimeState(path)
		if err != nil || !sameRuntimePathState(expected, current) {
			return fmt.Errorf("transaction-owned shell runtime result changed before commit: %s", path)
		}
	}
	return nil
}

func snapshotRuntimePath(path string) (runtimePathSnapshot, error) {
	state, err := snapshotRuntimePathState(path)
	if err != nil {
		return runtimePathSnapshot{}, err
	}
	return state.snapshot, nil
}

func snapshotRuntimePathState(path string) (runtimePathState, error) {
	anchor, exact, err := openNearestPinnedRuntimeDirectory(filepath.Dir(path))
	if err != nil {
		return runtimePathState{}, err
	}
	defer anchor.close()
	if !exact {
		return runtimePathState{snapshot: runtimePathSnapshot{path: path, kind: runtimeSnapshotAbsent}}, nil
	}
	return runtimeStateAt(anchor, filepath.Base(path), path)
}

func (tx *runtimeFileTransaction) mutate(operation, path string, mutation func() (runtimePathState, error)) error {
	owned, err := mutation()
	bound := owned.snapshot.path == path
	if bound {
		tx.applied = append(tx.applied, path)
		tx.owned[path] = owned
	}
	if err != nil {
		return tx.rollbackAfter(fmt.Errorf("%s shell runtime path %s: %w", operation, path, err))
	}
	if !bound {
		return tx.rollbackAfter(fmt.Errorf("mutation returned an unbound shell runtime result for %s", path))
	}
	if tx.journalHook != nil {
		if err := tx.journalHook(runtimeJournalMutation, path); err != nil {
			return tx.rollbackAfter(fmt.Errorf("journal shell runtime mutation %s: %w", path, err))
		}
	}
	if tx.hook != nil {
		if err := tx.hook(operation, path); err != nil {
			return tx.rollbackAfter(fmt.Errorf("after %s shell runtime path %s: %w", operation, path, err))
		}
	}
	return nil
}

func (tx *runtimeFileTransaction) rollbackAfter(cause error) error {
	if tx.rolledBack {
		return cause
	}
	tx.rolledBack = true
	var rollbackErrs []error
	restored := make(map[string]bool, len(tx.applied))
	for index := len(tx.applied) - 1; index >= 0; index-- {
		path := tx.applied[index]
		if restored[path] {
			continue
		}
		restored[path] = true
		owned, ok := tx.owned[path]
		if !ok {
			rollbackErrs = append(rollbackErrs, fmt.Errorf("transaction ownership is unavailable; recovery retained at %s", path))
			continue
		}
		if err := tx.restoreRuntimePath(tx.snapshots[path], owned); err != nil {
			rollbackErrs = append(rollbackErrs, fmt.Errorf("restore shell runtime path %s: %w", path, err))
		}
	}
	if len(rollbackErrs) == 0 {
		return cause
	}
	return errors.Join(append([]error{cause}, rollbackErrs...)...)
}

func (tx *runtimeFileTransaction) finish(cause error) error {
	if cause != nil && !tx.rolledBack && len(tx.applied) != 0 {
		cause = tx.rollbackAfter(cause)
	}
	var finishErrs []error
	if cause == nil {
		if err := tx.pruneCommittedQuarantines(); err != nil {
			finishErrs = append(finishErrs, err)
		}
	} else {
		if err := tx.cleanupCreatedDirectories(); err != nil {
			finishErrs = append(finishErrs, err)
		}
	}
	tx.close()
	if cause == nil && len(finishErrs) == 0 {
		return nil
	}
	if cause == nil {
		return errors.Join(finishErrs...)
	}
	return errors.Join(append([]error{cause}, finishErrs...)...)
}

func (tx *runtimeFileTransaction) close() {
	for _, recovery := range tx.recovery {
		if recovery == nil {
			continue
		}
		if recovery.file != nil {
			_ = recovery.file.Close()
		}
	}
	closed := make(map[*pinnedDirectory]bool, len(tx.parents)+len(tx.anchors)+len(tx.createdInOrder))
	for _, directories := range []map[string]*pinnedDirectory{tx.parents, tx.anchors} {
		for _, directory := range directories {
			if directory != nil && !closed[directory] {
				directory.close()
				closed[directory] = true
			}
		}
	}
	for _, created := range tx.createdInOrder {
		for _, directory := range []*pinnedDirectory{created.parent, created.directory} {
			if directory != nil && !closed[directory] {
				directory.close()
				closed[directory] = true
			}
		}
	}
}

func (tx *runtimeFileTransaction) restoreRuntimePath(snapshot runtimePathSnapshot, owned runtimePathState) error {
	directory := tx.parents[filepath.Dir(snapshot.path)]
	if directory == nil {
		return tx.restoreRuntimePathWithoutPinnedParent(snapshot, owned)
	}
	current, err := runtimeStateAt(directory, filepath.Base(snapshot.path), snapshot.path)
	if err != nil {
		return err
	}
	if !sameRuntimePathState(owned, current) {
		return tx.retainChangedRuntimeResult(snapshot.path, owned)
	}
	if owned.quarantine != nil {
		return restoreRuntimePathFromQuarantine(directory, snapshot, owned)
	}
	return restoreRuntimeSnapshot(directory, snapshot, owned)
}

func (tx *runtimeFileTransaction) restoreRuntimePathWithoutPinnedParent(snapshot runtimePathSnapshot, owned runtimePathState) error {
	if snapshot.kind != runtimeSnapshotAbsent || owned.snapshot.kind != runtimeSnapshotAbsent {
		return fmt.Errorf("shell runtime parent is not pinned for rollback: %s", filepath.Dir(snapshot.path))
	}
	current, err := tx.currentRuntimeState(snapshot.path)
	if err != nil {
		return err
	}
	if sameRuntimePathState(owned, current) {
		return nil
	}
	return fmt.Errorf("transaction-owned runtime absence changed; preserving concurrent winner: %s", snapshot.path)
}

func (tx *runtimeFileTransaction) retainChangedRuntimeResult(path string, owned runtimePathState) error {
	if owned.quarantine != nil {
		owned.quarantine.retained = true
		return fmt.Errorf("transaction-owned runtime result changed; preserving concurrent winner; recovery retained at %s", owned.quarantine.path)
	}
	retained, err := tx.retainRuntimeRecovery(path)
	if err != nil {
		return fmt.Errorf("transaction-owned runtime result changed; preserving concurrent winner; retain original recovery: %w", err)
	}
	if retained == "" {
		return fmt.Errorf("transaction-owned runtime result changed; preserving concurrent winner")
	}
	return fmt.Errorf("transaction-owned runtime result changed; preserving concurrent winner; recovery retained at %s", retained)
}

func restoreRuntimeSnapshot(directory *pinnedDirectory, snapshot runtimePathSnapshot, owned runtimePathState) error {
	switch snapshot.kind {
	case runtimeSnapshotAbsent:
		return removeRuntimePathState(snapshot.path, owned, directory, false)
	case runtimeSnapshotRegular:
		if owned.snapshot.kind == runtimeSnapshotAbsent {
			return publishRuntimeRegularIfAbsentAt(directory, filepath.Base(snapshot.path), snapshot.path, snapshot.content, snapshot.mode, false)
		}
		if err := removeRuntimePathState(snapshot.path, owned, directory, false); err != nil {
			return err
		}
		return publishRuntimeRegularIfAbsentAt(directory, filepath.Base(snapshot.path), snapshot.path, snapshot.content, snapshot.mode, false)
	case runtimeSnapshotSymlink:
		if owned.snapshot.kind != runtimeSnapshotAbsent {
			if err := removeRuntimePathState(snapshot.path, owned, directory, false); err != nil {
				return err
			}
		}
		return publishRuntimeSymlinkIfAbsentAt(directory, filepath.Base(snapshot.path), snapshot.path, snapshot.linkTarget, false)
	default:
		return fmt.Errorf("unknown shell runtime snapshot kind for %s", snapshot.path)
	}
}

func restoreRuntimePathFromQuarantine(directory *pinnedDirectory, snapshot runtimePathSnapshot, owned runtimePathState) error {
	quarantine := owned.quarantine
	if quarantine == nil || quarantine.directory != directory {
		return fmt.Errorf("shell runtime rollback quarantine is unavailable: %s", snapshot.path)
	}
	quarantined, err := runtimeStateAt(directory, quarantine.name, snapshot.path)
	if err != nil || !sameRuntimePathState(quarantine.state, quarantined) {
		quarantine.retained = true
		return fmt.Errorf("shell runtime rollback recovery changed; retained at %s", quarantine.path)
	}
	name := filepath.Base(snapshot.path)
	switch owned.snapshot.kind {
	case runtimeSnapshotAbsent:
		return restoreAbsentRuntimePathFromQuarantine(directory, name, snapshot, quarantine)
	case runtimeSnapshotRegular:
		return restoreRegularRuntimePathFromQuarantine(directory, name, snapshot, owned, quarantine)
	default:
		quarantine.retained = true
		return fmt.Errorf("unsupported quarantined shell runtime rollback for %s; recovery retained at %s", snapshot.path, quarantine.path)
	}
}

func restoreAbsentRuntimePathFromQuarantine(
	directory *pinnedDirectory,
	name string,
	snapshot runtimePathSnapshot,
	quarantine *runtimePathQuarantine,
) error {
	if err := unix.Renameat2(directory.fd(), quarantine.name, directory.fd(), name, unix.RENAME_NOREPLACE); err != nil {
		quarantine.retained = true
		return fmt.Errorf("restore quarantined shell runtime path without replacement; recovery retained at %s: %w", quarantine.path, err)
	}
	restored, err := runtimeStateAt(directory, name, snapshot.path)
	wantRestored := runtimePathState{snapshot: snapshot, info: quarantine.state.info}
	if err != nil || !sameRuntimePathState(wantRestored, restored) {
		return fmt.Errorf("shell runtime rollback postcondition failed after restoring %s", snapshot.path)
	}
	quarantine.name = ""
	return nil
}

func restoreRegularRuntimePathFromQuarantine(
	directory *pinnedDirectory,
	name string,
	snapshot runtimePathSnapshot,
	owned runtimePathState,
	quarantine *runtimePathQuarantine,
) error {
	if err := unix.Renameat2(directory.fd(), name, directory.fd(), quarantine.name, unix.RENAME_EXCHANGE); err != nil {
		quarantine.retained = true
		return fmt.Errorf("exchange shell runtime rollback recovery; retained at %s: %w", quarantine.path, err)
	}
	restored, restoredErr := runtimeStateAt(directory, name, snapshot.path)
	displaced, displacedErr := runtimeStateAt(directory, quarantine.name, snapshot.path)
	wantRestored := runtimePathState{snapshot: snapshot, info: quarantine.state.info}
	if restoredErr != nil || displacedErr != nil || !sameRuntimePathState(wantRestored, restored) || !sameRuntimePathState(owned, displaced) {
		quarantine.retained = true
		return fmt.Errorf("shell runtime rollback exchange had an uncertain postcondition; inspect %s and %s", snapshot.path, quarantine.path)
	}
	quarantine.state = displaced
	return pruneRuntimeQuarantine(quarantine)
}

func createRuntimePathRecovery(state runtimePathState, directory *pinnedDirectory) (*runtimePathRecovery, error) {
	if state.snapshot.kind == runtimeSnapshotAbsent {
		return nil, nil
	}
	if directory == nil {
		return nil, fmt.Errorf("shell runtime parent was not pinned for recovery: %s", state.snapshot.path)
	}
	file, err := createAnonymousConfigFile(directory, "shell runtime rollback recovery")
	if err != nil {
		return nil, err
	}
	data := state.snapshot.content
	mode := state.snapshot.mode
	if state.snapshot.kind == runtimeSnapshotSymlink {
		data = []byte("Wahrwelt shell runtime symlink recovery\n" + state.snapshot.linkTarget + "\n")
		mode = 0o600
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return nil, err
	}
	if err := file.Chmod(mode); err != nil {
		_ = file.Close()
		return nil, err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return nil, err
	}
	return &runtimePathRecovery{directory: directory, file: file, path: state.snapshot.path}, nil
}

func (tx *runtimeFileTransaction) retainRuntimeRecovery(path string) (string, error) {
	recovery := tx.recovery[path]
	if recovery == nil || recovery.file == nil || recovery.directory == nil {
		return "", nil
	}
	if recovery.retained != "" {
		return recovery.retained, nil
	}
	for attempt := 0; attempt < 32; attempt++ {
		var random [16]byte
		if _, err := rand.Read(random[:]); err != nil {
			return "", err
		}
		name := ".wahrwelt-runtime-recovery-" + hex.EncodeToString(random[:])
		if err := linkAnonymousConfigFile(recovery.file, recovery.directory, name); err != nil {
			if errors.Is(err, unix.EEXIST) {
				continue
			}
			return "", err
		}
		parent := recovery.directory.path
		if resolved, readlinkErr := os.Readlink(fmt.Sprintf("/proc/self/fd/%d", recovery.directory.fd())); readlinkErr == nil && filepath.IsAbs(resolved) {
			parent = resolved
		}
		recovery.retained = filepath.Join(parent, name)
		return recovery.retained, nil
	}
	return "", fmt.Errorf("no collision-free recovery name for %s", path)
}

func sameRuntimePathState(left, right runtimePathState) bool {
	if !sameRuntimePathSnapshot(left.snapshot, right.snapshot) {
		return false
	}
	if left.snapshot.kind == runtimeSnapshotAbsent {
		return true
	}
	return left.info != nil && right.info != nil && os.SameFile(left.info, right.info)
}

func sameRuntimePathSnapshot(left, right runtimePathSnapshot) bool {
	if left.path != right.path || left.kind != right.kind || left.mode != right.mode || left.linkTarget != right.linkTarget {
		return false
	}
	return bytes.Equal(left.content, right.content)
}

func runtimeStateAt(directory *pinnedDirectory, name, displayPath string) (runtimePathState, error) {
	pathFD, err := unix.Openat2(directory.fd(), name, &unix.OpenHow{
		Flags:   unix.O_PATH | unix.O_CLOEXEC | unix.O_NOFOLLOW,
		Resolve: unix.RESOLVE_BENEATH | unix.RESOLVE_NO_MAGICLINKS,
	})
	if err != nil {
		if errors.Is(err, unix.ENOENT) {
			return runtimePathState{snapshot: runtimePathSnapshot{path: displayPath, kind: runtimeSnapshotAbsent}}, nil
		}
		return runtimePathState{}, err
	}
	pathFile, err := newFileFromUnixFD(pathFD, displayPath)
	if err != nil {
		_ = unix.Close(pathFD)
		return runtimePathState{}, fmt.Errorf("wrap shell runtime path descriptor: %w", err)
	}
	defer func() { _ = pathFile.Close() }()
	before, err := pathFile.Stat()
	if err != nil {
		return runtimePathState{}, err
	}
	switch {
	case before.Mode().IsRegular():
		return regularRuntimeStateAt(directory, name, displayPath, before)
	case before.Mode()&os.ModeSymlink != 0:
		return symlinkRuntimeStateAt(directory, name, displayPath, before)
	default:
		return runtimePathState{}, fmt.Errorf("refusing non-regular shell runtime collision: %s", displayPath)
	}
}

func regularRuntimeStateAt(directory *pinnedDirectory, name, displayPath string, before os.FileInfo) (runtimePathState, error) {
	fd, err := unix.Openat2(directory.fd(), name, &unix.OpenHow{
		Flags:   unix.O_RDONLY | unix.O_CLOEXEC | unix.O_NOFOLLOW | unix.O_NONBLOCK,
		Resolve: unix.RESOLVE_BENEATH | unix.RESOLVE_NO_SYMLINKS | unix.RESOLVE_NO_MAGICLINKS,
	})
	if err != nil {
		return runtimePathState{}, fmt.Errorf("open shell runtime for snapshot: %s: %w", displayPath, err)
	}
	file, err := newFileFromUnixFD(fd, displayPath)
	if err != nil {
		_ = unix.Close(fd)
		return runtimePathState{}, fmt.Errorf("wrap shell runtime snapshot descriptor: %w", err)
	}
	data, readErr := readOpenRegularFile(file)
	opened, statErr := file.Stat()
	_ = file.Close()
	if readErr != nil {
		return runtimePathState{}, readErr
	}
	if statErr != nil || !opened.Mode().IsRegular() || !os.SameFile(before, opened) {
		return runtimePathState{}, fmt.Errorf("shell runtime path changed while reading: %s", displayPath)
	}
	after, err := runtimePathInfoAt(directory, name, displayPath)
	if err != nil || !os.SameFile(opened, after) {
		return runtimePathState{}, fmt.Errorf("shell runtime path changed while reading: %s", displayPath)
	}
	snapshot := runtimePathSnapshot{path: displayPath, kind: runtimeSnapshotRegular, mode: opened.Mode(), content: data}
	return runtimePathState{snapshot: snapshot, info: opened}, nil
}

func symlinkRuntimeStateAt(directory *pinnedDirectory, name, displayPath string, before os.FileInfo) (runtimePathState, error) {
	buffer := make([]byte, 4096)
	length, err := unix.Readlinkat(directory.fd(), name, buffer)
	if err != nil {
		return runtimePathState{}, err
	}
	after, err := runtimePathInfoAt(directory, name, displayPath)
	if err != nil || after.Mode()&os.ModeSymlink == 0 || !os.SameFile(before, after) {
		return runtimePathState{}, fmt.Errorf("shell runtime symlink changed while reading: %s", displayPath)
	}
	snapshot := runtimePathSnapshot{path: displayPath, kind: runtimeSnapshotSymlink, linkTarget: string(buffer[:length])}
	return runtimePathState{snapshot: snapshot, info: after}, nil
}

func runtimePathInfoAt(directory *pinnedDirectory, name, displayPath string) (os.FileInfo, error) {
	fd, err := unix.Openat2(directory.fd(), name, &unix.OpenHow{
		Flags:   unix.O_PATH | unix.O_CLOEXEC | unix.O_NOFOLLOW,
		Resolve: unix.RESOLVE_BENEATH | unix.RESOLVE_NO_MAGICLINKS,
	})
	if err != nil {
		return nil, err
	}
	file, err := newFileFromUnixFD(fd, displayPath)
	if err != nil {
		_ = unix.Close(fd)
		return nil, fmt.Errorf("wrap shell runtime identity descriptor: %w", err)
	}
	defer func() { _ = file.Close() }()
	return file.Stat()
}

func openNearestPinnedRuntimeDirectory(path string) (*pinnedDirectory, bool, error) {
	clean := filepath.Clean(path)
	if !filepath.IsAbs(clean) {
		return nil, false, fmt.Errorf("shell runtime parent must be absolute: %s", path)
	}
	fd, err := unix.Open("/", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return nil, false, err
	}
	file, err := newFileFromUnixFD(fd, "/")
	if err != nil {
		_ = unix.Close(fd)
		return nil, false, fmt.Errorf("wrap pinned shell runtime root directory: %w", err)
	}
	info, err := file.Stat()
	if err != nil || !info.IsDir() {
		_ = file.Close()
		if err != nil {
			return nil, false, err
		}
		return nil, false, fmt.Errorf("shell runtime root is not a directory")
	}
	directory := &pinnedDirectory{path: "/", file: file, info: info, descriptor: fd}
	for _, name := range strings.Split(strings.TrimPrefix(clean, "/"), "/") {
		if name == "" || name == "." {
			continue
		}
		nextPath := filepath.Join(directory.path, name)
		next, err := openPinnedRuntimeChild(directory, name, nextPath)
		if err != nil {
			if errors.Is(err, unix.ENOENT) {
				if canonicalErr := directory.checkCanonical(); canonicalErr != nil {
					directory.close()
					return nil, false, canonicalErr
				}
				return directory, false, nil
			}
			directory.close()
			return nil, false, fmt.Errorf("refusing non-directory shell runtime parent component %s: %w", nextPath, err)
		}
		directory.close()
		directory = next
	}
	if err := directory.checkCanonical(); err != nil {
		directory.close()
		return nil, false, err
	}
	return directory, true, nil
}

func openPinnedRuntimeDirectory(path string) (*pinnedDirectory, error) {
	directory, exact, err := openNearestPinnedRuntimeDirectory(path)
	if err != nil {
		return nil, err
	}
	if !exact {
		directory.close()
		return nil, unix.ENOENT
	}
	return directory, nil
}

func openPinnedRuntimeChild(parent *pinnedDirectory, name, path string) (*pinnedDirectory, error) {
	fd, err := unix.Openat2(parent.fd(), name, &unix.OpenHow{
		Flags:   unix.O_RDONLY | unix.O_DIRECTORY | unix.O_CLOEXEC | unix.O_NOFOLLOW,
		Resolve: unix.RESOLVE_BENEATH | unix.RESOLVE_NO_SYMLINKS | unix.RESOLVE_NO_MAGICLINKS,
	})
	if err != nil {
		return nil, err
	}
	file, err := newFileFromUnixFD(fd, path)
	if err != nil {
		_ = unix.Close(fd)
		return nil, fmt.Errorf("wrap pinned shell runtime parent: %w", err)
	}
	info, err := file.Stat()
	if err != nil || !info.IsDir() {
		_ = file.Close()
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("shell runtime path is not a directory: %s", path)
	}
	return &pinnedDirectory{path: path, file: file, info: info, descriptor: fd}, nil
}

func runtimeStateBeneathAnchor(anchor *pinnedDirectory, path string) (runtimePathState, error) {
	if anchor == nil {
		return runtimePathState{}, fmt.Errorf("shell runtime path has no pinned ancestor: %s", path)
	}
	parentPath := filepath.Dir(path)
	relative, err := filepath.Rel(anchor.path, parentPath)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
		if err != nil {
			return runtimePathState{}, err
		}
		return runtimePathState{}, fmt.Errorf("shell runtime path escapes begin-time anchor: %s", path)
	}
	if err := anchor.checkCanonical(); err != nil {
		return runtimePathState{}, err
	}
	current, opened, missing, err := openRuntimeParentBeneathAnchor(anchor, relative, path)
	defer func() {
		for _, directory := range opened {
			directory.close()
		}
	}()
	if err != nil {
		return runtimePathState{}, err
	}
	if missing {
		return runtimePathState{snapshot: runtimePathSnapshot{path: path, kind: runtimeSnapshotAbsent}}, nil
	}
	state, err := runtimeStateAt(current, filepath.Base(path), path)
	if err != nil {
		return runtimePathState{}, err
	}
	if err := current.checkCanonical(); err != nil {
		return runtimePathState{}, err
	}
	if err := anchor.checkCanonical(); err != nil {
		return runtimePathState{}, err
	}
	return state, nil
}

func openRuntimeParentBeneathAnchor(
	anchor *pinnedDirectory,
	relative, path string,
) (*pinnedDirectory, []*pinnedDirectory, bool, error) {
	current := anchor
	var opened []*pinnedDirectory
	if relative == "." {
		return current, opened, false, nil
	}
	for _, name := range strings.Split(relative, string(os.PathSeparator)) {
		childPath := filepath.Join(current.path, name)
		next, err := openPinnedRuntimeChild(current, name, childPath)
		if err == nil {
			opened = append(opened, next)
			current = next
			continue
		}
		if errors.Is(err, unix.ENOENT) {
			if canonicalErr := current.checkCanonical(); canonicalErr != nil {
				return current, opened, false, canonicalErr
			}
			return current, opened, true, nil
		}
		return current, opened, false, fmt.Errorf("refusing changed shell runtime parent component %s for %s: %w", childPath, path, err)
	}
	return current, opened, false, nil
}

func (tx *runtimeFileTransaction) createPinnedRuntimeDescendant(anchor *pinnedDirectory, path string) (*pinnedDirectory, error) {
	if anchor == nil {
		return nil, fmt.Errorf("shell runtime parent has no begin-time anchor: %s", path)
	}
	if err := anchor.checkCanonical(); err != nil {
		return nil, err
	}
	relative, err := filepath.Rel(anchor.path, path)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("shell runtime parent escapes begin-time anchor: %s", path)
	}
	current := anchor
	for _, name := range strings.Split(relative, string(os.PathSeparator)) {
		childPath := filepath.Join(current.path, name)
		if created := tx.created[childPath]; created != nil {
			current = created.directory
			continue
		}
		if _, err := runtimePathInfoAt(current, name, childPath); err == nil {
			return nil, fmt.Errorf("shell runtime parent appeared after transaction begin; preserving concurrent winner: %s", childPath)
		} else if !errors.Is(err, unix.ENOENT) {
			return nil, err
		}
		next, err := tx.createAndJournalRuntimeDirectory(current, name, childPath)
		if err != nil {
			return nil, err
		}
		current = next
	}
	if err := current.checkCanonical(); err != nil {
		return nil, err
	}
	return current, nil
}

func (tx *runtimeFileTransaction) createAndJournalRuntimeDirectory(parent *pinnedDirectory, name, path string) (*pinnedDirectory, error) {
	return tx.createAndJournalRuntimeDirectoryWithHooks(parent, name, path, nil, nil)
}

func (tx *runtimeFileTransaction) createAndJournalRuntimeDirectoryWithHook(
	parent *pinnedDirectory,
	name,
	path string,
	afterCandidatePinned func(candidatePath string) error,
) (*pinnedDirectory, error) {
	return tx.createAndJournalRuntimeDirectoryWithHooks(parent, name, path, nil, afterCandidatePinned)
}

func (tx *runtimeFileTransaction) createAndJournalRuntimeDirectoryWithHooks(
	parent *pinnedDirectory,
	name,
	path string,
	beforeCandidatePinned,
	afterCandidatePinned func(candidatePath string) error,
) (*pinnedDirectory, error) {
	temporaryName, directory, err := createRuntimeDirectoryCandidate(parent, beforeCandidatePinned)
	if err != nil {
		return nil, err
	}
	temporaryPath := filepath.Join(parent.path, temporaryName)
	cleanupCandidate := true
	fail := func(cause error) (*pinnedDirectory, error) {
		if cleanupCandidate {
			cleanupErr := removePinnedRuntimeDirectoryCandidate(parent, temporaryName, directory.info, directory)
			directory.close()
			return nil, errors.Join(cause, cleanupErr)
		}
		return nil, cause
	}
	if err := validateRuntimeDirectoryCandidate(parent, directory, temporaryName, temporaryPath, name, path, afterCandidatePinned); err != nil {
		return fail(err)
	}
	if err := unix.Renameat2(parent.fd(), temporaryName, parent.fd(), name, unix.RENAME_NOREPLACE); err != nil {
		if errors.Is(err, unix.EEXIST) || errors.Is(err, unix.ENOTEMPTY) {
			return fail(fmt.Errorf("shell runtime parent appeared after transaction begin; preserving concurrent winner: %s", path))
		}
		return fail(err)
	}
	directory.path = path
	cleanupCandidate = false
	if err := tx.journalCreatedRuntimeDirectory(parent, directory, name, path); err != nil {
		return nil, err
	}
	return directory, nil
}

func validateRuntimeDirectoryCandidate(
	parent, directory *pinnedDirectory,
	temporaryName, temporaryPath, name, path string,
	afterCandidatePinned func(candidatePath string) error,
) error {
	if afterCandidatePinned != nil {
		if err := afterCandidatePinned(temporaryPath); err != nil {
			return err
		}
	}
	if err := unix.Fchmod(directory.fd(), 0o755); err != nil {
		return err
	}
	candidateInfo, err := runtimePathInfoAt(parent, temporaryName, temporaryPath)
	if err != nil || !os.SameFile(directory.info, candidateInfo) {
		return fmt.Errorf("shell runtime directory candidate changed before publication: %s", temporaryPath)
	}
	if _, err := runtimePathInfoAt(parent, name, path); err == nil {
		return fmt.Errorf("shell runtime parent appeared after transaction begin; preserving concurrent winner: %s", path)
	} else if !errors.Is(err, unix.ENOENT) {
		return err
	}
	if err := parent.checkCanonical(); err != nil {
		return err
	}
	candidateInfo, err = runtimePathInfoAt(parent, temporaryName, temporaryPath)
	if err != nil || !os.SameFile(directory.info, candidateInfo) {
		return fmt.Errorf("shell runtime directory candidate changed before rename: %s", temporaryPath)
	}
	return nil
}

func (tx *runtimeFileTransaction) journalCreatedRuntimeDirectory(parent, directory *pinnedDirectory, name, path string) error {
	created := &runtimeCreatedDirectory{parent: parent, directory: directory, name: name, path: path, info: directory.info}
	tx.created[path] = created
	tx.createdInOrder = append(tx.createdInOrder, created)
	if tx.journalHook != nil {
		if err := tx.journalHook(runtimeJournalDirectory, path); err != nil {
			return fmt.Errorf("journal created shell runtime directory %s: %w", path, err)
		}
	}
	return nil
}

func createRuntimeDirectoryCandidate(
	parent *pinnedDirectory,
	beforeOpen func(candidatePath string) error,
) (string, *pinnedDirectory, error) {
	for attempt := 0; attempt < 32; attempt++ {
		name, err := randomRuntimeEntryName(".wahrwelt-runtime-dir-")
		if err != nil {
			return "", nil, err
		}
		if err := unix.Mkdirat(parent.fd(), name, 0o700); err != nil {
			if errors.Is(err, unix.EEXIST) {
				continue
			}
			return "", nil, err
		}
		path := filepath.Join(parent.path, name)
		createdInfo, err := runtimePathInfoAt(parent, name, path)
		if err != nil {
			return "", nil, fmt.Errorf("capture newly created shell runtime directory candidate identity; recovery retained at %s: %w", path, err)
		}
		if beforeOpen != nil {
			if err := beforeOpen(path); err != nil {
				return "", nil, fmt.Errorf("before pinning newly created shell runtime directory candidate; recovery retained at %s: %w", path, err)
			}
		}
		directory, err := openPinnedRuntimeChild(parent, name, path)
		if err != nil {
			return "", nil, fmt.Errorf("pin newly created shell runtime directory candidate; recovery retained at %s: %w", path, err)
		}
		if !os.SameFile(createdInfo, directory.info) {
			directory.close()
			return "", nil, fmt.Errorf("newly created shell runtime directory candidate changed before pin; unknown entry preserved at %s", path)
		}
		return name, directory, nil
	}
	return "", nil, fmt.Errorf("no collision-free shell runtime directory candidate")
}

func removePinnedRuntimeDirectoryCandidate(parent *pinnedDirectory, name string, expected os.FileInfo, pinned *pinnedDirectory) error {
	current, err := runtimePathInfoAt(parent, name, filepath.Join(parent.path, name))
	if err != nil {
		if errors.Is(err, unix.ENOENT) {
			return nil
		}
		return err
	}
	if !os.SameFile(expected, current) {
		return fmt.Errorf(
			"shell runtime directory candidate changed; preserving unknown entry at %s; transaction recovery retained at %s",
			filepath.Join(parent.path, name),
			currentPinnedDirectoryPath(pinned),
		)
	}
	recoveryName, err := randomRuntimeEntryName(".wahrwelt-runtime-recovery-dir-")
	if err != nil {
		return err
	}
	if err := unix.Renameat2(parent.fd(), name, parent.fd(), recoveryName, unix.RENAME_NOREPLACE); err != nil {
		return fmt.Errorf("retain shell runtime directory candidate at %s: %w", filepath.Join(parent.path, name), err)
	}
	return nil
}

func publishRuntimeRegularFile(path string, data []byte, mode os.FileMode, expected runtimePathState, directory *pinnedDirectory) (runtimePathState, error) {
	if directory == nil {
		return runtimePathState{}, fmt.Errorf("shell runtime parent is not pinned: %s", filepath.Dir(path))
	}
	name := filepath.Base(path)
	current, err := runtimeStateAt(directory, name, path)
	if err != nil {
		return runtimePathState{}, err
	}
	if !sameRuntimePathState(expected, current) {
		return runtimePathState{}, fmt.Errorf("shell runtime publication target changed before write: %s", path)
	}
	switch expected.snapshot.kind {
	case runtimeSnapshotAbsent:
		return publishRuntimeRegularAt(directory, name, path, data, mode, true)
	case runtimeSnapshotRegular:
		return replaceRuntimeRegularAt(directory, name, path, expected, data, mode, true)
	case runtimeSnapshotSymlink:
		return runtimePathState{}, fmt.Errorf("refusing symlink shell runtime publication collision: %s", path)
	default:
		return runtimePathState{}, fmt.Errorf("unknown shell runtime publication state: %s", path)
	}
}

func publishRuntimeRegularIfAbsentAt(directory *pinnedDirectory, name, path string, data []byte, mode os.FileMode, requireCanonical bool) error {
	if directory == nil {
		return fmt.Errorf("shell runtime parent is not pinned: %s", filepath.Dir(path))
	}
	current, err := runtimeStateAt(directory, name, path)
	if err != nil {
		return err
	}
	if current.snapshot.kind != runtimeSnapshotAbsent {
		return fmt.Errorf("shell runtime rollback target appeared; preserving concurrent winner: %s", path)
	}
	_, err = publishRuntimeRegularAt(directory, name, path, data, mode, requireCanonical)
	return err
}

func publishRuntimeRegularAt(directory *pinnedDirectory, name, displayPath string, data []byte, mode os.FileMode, requireCanonical bool) (runtimePathState, error) {
	file, err := createAnonymousConfigFile(directory, "shell runtime")
	if err != nil {
		return runtimePathState{}, err
	}
	defer func() { _ = file.Close() }()
	if _, err := file.Write(data); err != nil {
		return runtimePathState{}, err
	}
	if err := file.Chmod(mode); err != nil {
		return runtimePathState{}, err
	}
	if err := file.Sync(); err != nil {
		return runtimePathState{}, err
	}
	if requireCanonical {
		if err := directory.checkCanonical(); err != nil {
			return runtimePathState{}, err
		}
	}
	if err := linkAnonymousConfigFile(file, directory, name); err != nil {
		if errors.Is(err, unix.EEXIST) {
			return runtimePathState{}, fmt.Errorf("shell runtime publication target appeared; preserving concurrent winner: %s", displayPath)
		}
		return runtimePathState{}, fmt.Errorf("publish shell runtime without replacement: %w", err)
	}
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return runtimePathState{}, fmt.Errorf("shell runtime publication postcondition failed: %s", displayPath)
	}
	owned := runtimePathState{snapshot: runtimePathSnapshot{path: displayPath, kind: runtimeSnapshotRegular, mode: info.Mode(), content: bytes.Clone(data)}, info: info}
	if !samePathFile(directory.anchoredPath(name), info) {
		return owned, fmt.Errorf("shell runtime publication target changed during write; preserving concurrent winner: %s", displayPath)
	}
	if requireCanonical {
		if err := directory.checkCanonical(); err != nil {
			return owned, err
		}
	}
	return owned, nil
}

func replaceRuntimeRegularAt(directory *pinnedDirectory, name, displayPath string, expected runtimePathState, data []byte, mode os.FileMode, requireCanonical bool) (runtimePathState, error) {
	return replaceRuntimeRegularAtWithExchangeHook(directory, name, displayPath, expected, data, mode, requireCanonical, nil)
}

func replaceRuntimeRegularAtWithExchangeHook(
	directory *pinnedDirectory,
	name,
	displayPath string,
	expected runtimePathState,
	data []byte,
	mode os.FileMode,
	requireCanonical bool,
	beforeExchange func(candidatePath string) error,
) (runtimePathState, error) {
	candidate, candidateState, err := prepareRuntimeReplacementCandidate(directory, displayPath, data, mode)
	if err != nil {
		return runtimePathState{}, err
	}
	defer func() { _ = candidate.Close() }()
	candidateQuarantine, err := linkRuntimeReplacementCandidate(candidate, directory, candidateState)
	if err != nil {
		return runtimePathState{}, err
	}
	cleanupCandidate := true
	defer func() {
		if cleanupCandidate {
			_ = pruneRuntimeQuarantine(candidateQuarantine)
		}
	}()
	if err := prepareRuntimeReplacementExchange(directory, name, displayPath, expected, candidateQuarantine.path, requireCanonical, beforeExchange); err != nil {
		return runtimePathState{}, err
	}
	if err := unix.Renameat2(directory.fd(), candidateQuarantine.name, directory.fd(), name, unix.RENAME_EXCHANGE); err != nil {
		return runtimePathState{}, fmt.Errorf("exchange shell runtime replacement: %w", err)
	}
	cleanupCandidate = false
	displaced, displacedErr := runtimeStateAt(directory, candidateQuarantine.name, displayPath)
	published, publishedErr := runtimeStateAt(directory, name, displayPath)
	if displacedErr != nil || publishedErr != nil || !sameRuntimePathState(expected, displaced) || !sameRuntimePathState(candidateState, published) {
		return handleFailedRuntimeReplacement(
			directory,
			name,
			displayPath,
			candidateState,
			candidateQuarantine,
			displaced,
			published,
			displacedErr,
			publishedErr,
		)
	}
	owned := candidateState
	owned.quarantine = &runtimePathQuarantine{
		directory: directory,
		name:      candidateQuarantine.name,
		path:      candidateQuarantine.path,
		state:     displaced,
	}
	if requireCanonical {
		if err := directory.checkCanonical(); err != nil {
			return owned, err
		}
	}
	return owned, nil
}

func prepareRuntimeReplacementCandidate(
	directory *pinnedDirectory,
	displayPath string,
	data []byte,
	mode os.FileMode,
) (*os.File, runtimePathState, error) {
	candidate, err := createAnonymousConfigFile(directory, "shell runtime replacement")
	if err != nil {
		return nil, runtimePathState{}, err
	}
	fail := func(err error) (*os.File, runtimePathState, error) {
		_ = candidate.Close()
		return nil, runtimePathState{}, err
	}
	if _, err := candidate.Write(data); err != nil {
		return fail(err)
	}
	if err := candidate.Chmod(mode); err != nil {
		return fail(err)
	}
	if err := candidate.Sync(); err != nil {
		return fail(err)
	}
	info, err := candidate.Stat()
	if err != nil {
		return fail(err)
	}
	state := runtimePathState{
		snapshot: runtimePathSnapshot{path: displayPath, kind: runtimeSnapshotRegular, mode: info.Mode(), content: bytes.Clone(data)},
		info:     info,
	}
	return candidate, state, nil
}

func linkRuntimeReplacementCandidate(
	candidate *os.File,
	directory *pinnedDirectory,
	state runtimePathState,
) (*runtimePathQuarantine, error) {
	name, err := linkRuntimeAnonymousAtRandom(candidate, directory, ".wahrwelt-runtime-candidate-")
	if err != nil {
		return nil, err
	}
	return &runtimePathQuarantine{
		directory: directory,
		name:      name,
		path:      filepath.Join(directory.path, name),
		state:     state,
	}, nil
}

func prepareRuntimeReplacementExchange(
	directory *pinnedDirectory,
	name, displayPath string,
	expected runtimePathState,
	candidatePath string,
	requireCanonical bool,
	beforeExchange func(candidatePath string) error,
) error {
	current, err := runtimeStateAt(directory, name, displayPath)
	if err != nil || !sameRuntimePathState(expected, current) {
		return fmt.Errorf("shell runtime publication target changed before write: %s", displayPath)
	}
	if requireCanonical {
		if err := directory.checkCanonical(); err != nil {
			return err
		}
	}
	if beforeExchange == nil {
		return nil
	}
	return beforeExchange(candidatePath)
}

func handleFailedRuntimeReplacement(
	directory *pinnedDirectory,
	name, displayPath string,
	candidateState runtimePathState,
	candidateQuarantine *runtimePathQuarantine,
	displaced, published runtimePathState,
	displacedErr, publishedErr error,
) (runtimePathState, error) {
	owned := candidateState
	owned.quarantine = &runtimePathQuarantine{
		directory: directory,
		name:      candidateQuarantine.name,
		path:      candidateQuarantine.path,
		state:     displaced,
	}
	if displacedErr == nil && publishedErr == nil && restoreExactRuntimeReplacementPair(
		directory,
		name,
		displayPath,
		candidateQuarantine,
		displaced,
		published,
	) {
		return runtimePathState{}, fmt.Errorf("shell runtime publication target changed during replacement; exact pair restored and recovery retained at %s", candidateQuarantine.path)
	}
	owned.quarantine.retained = true
	return owned, fmt.Errorf("shell runtime replacement postcondition failed; recoveries retained at %s and %s", displayPath, owned.quarantine.path)
}

func restoreExactRuntimeReplacementPair(
	directory *pinnedDirectory,
	name, displayPath string,
	candidateQuarantine *runtimePathQuarantine,
	displaced, published runtimePathState,
) bool {
	currentPublished, publishedErr := runtimeStateAt(directory, name, displayPath)
	currentDisplaced, displacedErr := runtimeStateAt(directory, candidateQuarantine.name, displayPath)
	if publishedErr != nil || displacedErr != nil ||
		!sameRuntimePathState(published, currentPublished) || !sameRuntimePathState(displaced, currentDisplaced) {
		return false
	}
	if err := unix.Renameat2(directory.fd(), name, directory.fd(), candidateQuarantine.name, unix.RENAME_EXCHANGE); err != nil {
		return false
	}
	restored, restoredErr := runtimeStateAt(directory, name, displayPath)
	retained, retainedErr := runtimeStateAt(directory, candidateQuarantine.name, displayPath)
	if restoredErr != nil || retainedErr != nil ||
		!sameRuntimePathState(displaced, restored) || !sameRuntimePathState(published, retained) {
		return false
	}
	candidateQuarantine.state = retained
	candidateQuarantine.retained = true
	return true
}

func quarantineRuntimePathState(path string, expected runtimePathState, directory *pinnedDirectory, requireCanonical bool) (*runtimePathQuarantine, error) {
	return quarantineRuntimePathStateWithHook(path, expected, directory, requireCanonical, nil)
}

func quarantineRuntimePathStateWithHook(path string, expected runtimePathState, directory *pinnedDirectory, requireCanonical bool, beforeRename func() error) (*runtimePathQuarantine, error) {
	name, err := validateRuntimePathQuarantineTarget(path, expected, directory, requireCanonical)
	if err != nil {
		return nil, err
	}
	quarantineName, err := randomRuntimeEntryName(".wahrwelt-runtime-quarantine-")
	if err != nil {
		return nil, err
	}
	if beforeRename != nil {
		if err := beforeRename(); err != nil {
			return nil, err
		}
	}
	if err := unix.Renameat2(directory.fd(), name, directory.fd(), quarantineName, unix.RENAME_NOREPLACE); err != nil {
		if errors.Is(err, unix.EEXIST) {
			return nil, fmt.Errorf("shell runtime quarantine collision: %s", filepath.Join(directory.path, quarantineName))
		}
		return nil, err
	}
	moved, inspectErr := runtimeStateAt(directory, quarantineName, path)
	if inspectErr != nil || !sameRuntimePathState(expected, moved) {
		restoreErr := unix.Renameat2(directory.fd(), quarantineName, directory.fd(), name, unix.RENAME_NOREPLACE)
		if restoreErr != nil {
			return nil, fmt.Errorf("shell runtime removal raced; unknown entry retained at %s: %w", filepath.Join(directory.path, quarantineName), restoreErr)
		}
		return nil, fmt.Errorf("shell runtime removal target changed during quarantine; restored unknown entry: %s", path)
	}
	quarantine := &runtimePathQuarantine{
		directory: directory,
		name:      quarantineName,
		path:      filepath.Join(directory.path, quarantineName),
		state:     moved,
	}
	if requireCanonical {
		if err := directory.checkCanonical(); err != nil {
			return quarantine, err
		}
	}
	return quarantine, nil
}

func validateRuntimePathQuarantineTarget(
	path string,
	expected runtimePathState,
	directory *pinnedDirectory,
	requireCanonical bool,
) (string, error) {
	if directory == nil {
		return "", fmt.Errorf("shell runtime parent is not pinned: %s", filepath.Dir(path))
	}
	name := filepath.Base(path)
	current, err := runtimeStateAt(directory, name, path)
	if err != nil {
		return "", err
	}
	if !sameRuntimePathState(expected, current) {
		return "", fmt.Errorf("shell runtime removal target changed; preserving concurrent winner: %s", path)
	}
	if requireCanonical {
		if err := directory.checkCanonical(); err != nil {
			return "", err
		}
	}
	return name, nil
}

func removeRuntimePathState(path string, expected runtimePathState, directory *pinnedDirectory, requireCanonical bool) error {
	quarantine, err := quarantineRuntimePathState(path, expected, directory, requireCanonical)
	if err != nil {
		return err
	}
	return pruneRuntimeQuarantine(quarantine)
}

func randomRuntimeEntryName(prefix string) (string, error) {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(random[:]), nil
}

func linkRuntimeAnonymousAtRandom(file *os.File, directory *pinnedDirectory, prefix string) (string, error) {
	for attempt := 0; attempt < 32; attempt++ {
		name, err := randomRuntimeEntryName(prefix)
		if err != nil {
			return "", err
		}
		if err := linkAnonymousConfigFile(file, directory, name); err != nil {
			if errors.Is(err, unix.EEXIST) {
				continue
			}
			return "", err
		}
		return name, nil
	}
	return "", fmt.Errorf("no collision-free shell runtime entry name")
}

func pruneRuntimeQuarantine(quarantine *runtimePathQuarantine) error {
	if quarantine == nil || quarantine.name == "" || quarantine.retained {
		return nil
	}
	directory := quarantine.directory
	movedName, err := randomRuntimeEntryName(".wahrwelt-runtime-recovery-")
	if err != nil {
		return err
	}
	if err := unix.Renameat2(directory.fd(), quarantine.name, directory.fd(), movedName, unix.RENAME_NOREPLACE); err != nil {
		if errors.Is(err, unix.ENOENT) {
			return nil
		}
		quarantine.retained = true
		return fmt.Errorf("move shell runtime entry to retained recovery; retained at %s: %w", quarantine.path, err)
	}
	movedPath := filepath.Join(directory.path, movedName)
	moved, inspectErr := runtimeStateAt(directory, movedName, quarantine.state.snapshot.path)
	if inspectErr != nil || !sameRuntimePathState(quarantine.state, moved) {
		restoreErr := unix.Renameat2(directory.fd(), movedName, directory.fd(), quarantine.name, unix.RENAME_NOREPLACE)
		if restoreErr != nil {
			quarantine.name = movedName
			quarantine.path = movedPath
			quarantine.retained = true
			return fmt.Errorf("unknown shell runtime recovery moved while retaining; retained at %s: %w", movedPath, restoreErr)
		}
		quarantine.retained = true
		return fmt.Errorf("shell runtime recovery changed while retaining; restored unknown entry at %s", quarantine.path)
	}
	quarantine.name = movedName
	quarantine.path = movedPath
	quarantine.retained = true
	return nil
}

func (tx *runtimeFileTransaction) pruneCommittedQuarantines() error {
	seen := make(map[*runtimePathQuarantine]bool, len(tx.owned))
	var errs []error
	for _, owned := range tx.owned {
		quarantine := owned.quarantine
		if quarantine == nil || seen[quarantine] {
			continue
		}
		seen[quarantine] = true
		if err := pruneRuntimeQuarantine(quarantine); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (tx *runtimeFileTransaction) cleanupCreatedDirectories() error {
	var errs []error
	for index := len(tx.createdInOrder) - 1; index >= 0; index-- {
		if err := cleanupCreatedRuntimeDirectory(tx.createdInOrder[index]); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func cleanupCreatedRuntimeDirectory(created *runtimeCreatedDirectory) error {
	if created == nil || created.parent == nil || created.directory == nil {
		return nil
	}
	quarantineName, err := randomRuntimeEntryName(".wahrwelt-runtime-recovery-dir-")
	if err != nil {
		return err
	}
	if err := unix.Renameat2(created.parent.fd(), created.name, created.parent.fd(), quarantineName, unix.RENAME_NOREPLACE); err != nil {
		if errors.Is(err, unix.ENOENT) {
			return nil
		}
		return fmt.Errorf("quarantine created shell runtime directory %s: %w", created.path, err)
	}
	quarantinePath := filepath.Join(created.parent.path, quarantineName)
	moved, err := openPinnedRuntimeChild(created.parent, quarantineName, quarantinePath)
	if err != nil || !os.SameFile(created.info, moved.info) {
		if moved != nil {
			moved.close()
		}
		restoreErr := unix.Renameat2(created.parent.fd(), quarantineName, created.parent.fd(), created.name, unix.RENAME_NOREPLACE)
		if restoreErr != nil {
			return fmt.Errorf("created shell runtime directory was replaced; unknown recovery retained at %s: %w", quarantinePath, restoreErr)
		}
		return fmt.Errorf("created shell runtime directory was replaced; restored unknown entry at %s", created.path)
	}
	defer moved.close()
	entries, readErr := moved.file.ReadDir(1)
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		_ = unix.Renameat2(created.parent.fd(), quarantineName, created.parent.fd(), created.name, unix.RENAME_NOREPLACE)
		return fmt.Errorf("inspect created shell runtime directory before cleanup: %s: %w", created.path, readErr)
	}
	if len(entries) != 0 {
		if restoreErr := unix.Renameat2(created.parent.fd(), quarantineName, created.parent.fd(), created.name, unix.RENAME_NOREPLACE); restoreErr != nil {
			return fmt.Errorf("created shell runtime directory gained concurrent content; recovery retained at %s: %w", quarantinePath, restoreErr)
		}
		return fmt.Errorf("created shell runtime directory gained concurrent content; preserved at %s", created.path)
	}
	return nil
}

func publishRuntimeSymlinkIfAbsentAt(directory *pinnedDirectory, name, path, target string, requireCanonical bool) error {
	if directory == nil {
		return fmt.Errorf("shell runtime parent is not pinned: %s", filepath.Dir(path))
	}
	current, err := runtimeStateAt(directory, name, path)
	if err != nil {
		return err
	}
	if current.snapshot.kind != runtimeSnapshotAbsent {
		return fmt.Errorf("shell runtime rollback target appeared; preserving concurrent winner: %s", path)
	}
	if requireCanonical {
		if err := directory.checkCanonical(); err != nil {
			return err
		}
	}
	if err := unix.Symlinkat(target, directory.fd(), name); err != nil {
		if errors.Is(err, unix.EEXIST) {
			return fmt.Errorf("shell runtime rollback target appeared; preserving concurrent winner: %s", path)
		}
		return err
	}
	if requireCanonical {
		return directory.checkCanonical()
	}
	return nil
}
