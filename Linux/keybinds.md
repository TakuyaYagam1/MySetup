# Keybinds

Hyprland-based NixOS configuration with three swappable QuickShell-based
shells. Use `Super+Shift+W` to switch between them at runtime.

The configuration is layered: a **common** set is sourced by every shell,
then each shell adds its own bindings on top.

## How to read the tables

| Notation | Meaning |
| --- | --- |
| `Super` | Windows/Meta key |
| `Super+Shift, X` | hold Super+Shift, press X |
| `bind` | normal single-press binding |
| `bindl` | also fires when the session is locked |
| `binde` | repeats while held (e.g. resize, workspace cycle) |
| `bindr` | fires on release (used for `qs ... kill`) |
| `bindm` | mouse binding (movewindow / resizewindow) |
| `$kbX` | named variable defined in `dots/hypr/variables.conf` |

Source files in the repository:

- `Linux/dots/hypr/variables.conf` - keybind variable definitions
- `Linux/dots/hypr/hyprland/keybinds.conf` - base Hyprland keybinds
- `Linux/dots/hypr/shell-common-keybinds.conf` - shared shell entrypoints
- `Linux/dots/hypr/shell-workspace-keybinds.conf` - workspace group navigation
- `Linux/dots/hypr/{caelestia,noctalia,end4}/keybinds.conf` - per-shell
- `Linux/dots/hypr/{caelestia,noctalia}/launcher.conf` - Super long-press launcher

---

## Common (all shells)

These keybinds are sourced by **all three shells** (caelestia, noctalia,
end4) via `source = $hypr/shell-common-keybinds.conf` and
`shell-workspace-keybinds.conf`, plus the base Hyprland config in
`hyprland/keybinds.conf`.

### Shell selector

| Keys | Action |
| --- | --- |
| `Super+Shift+W` | Toggle QuickShell-based shell selector (switch between caelestia / noctalia / end4) |

### Application launchers

| Keys | Action |
| --- | --- |
| `Super+Return` | Terminal |
| `Super+Shift+F` | Zen Browser |
| `Super+Shift+C` | Cursor IDE |
| `Super+Shift+V` | VS Code |
| `Super+Shift+Z` | Zed |
| `Super+Shift+D` | DataGrip |
| `Super+Shift+A` | Antigravity (Gemini CLI) |
| `Super+Shift+Q` | AmneziaVPN |
| `Super+Shift, v2rayN` | v2rayN |
| `Super+Shift+B` | Vesktop (Discord) |
| `Super+Shift+I` | Spotify (toggle) |
| `Super+Shift+T` | Telegram |
| `Ctrl+Shift+Escape` | btop |
| `Super+E` | File explorer |
| `Super+Alt+E` | File explorer (duplicate keybind) |
| `Ctrl+Alt+V` | pavucontrol (audio mixer) |

### Screenshots & screen recording

| Keys | Action |
| --- | --- |
| `Super+S` | Screenshot whole screen |
| `Super+Shift+S` | Screenshot region |
| `Super+Shift+Alt+S` | Screenshot + edit |
| `Super+C` | Color picker (`hyprpicker -a`) |
| `Super+R` | Toggle screen recording |

### Workspaces

| Keys | Action |
| --- | --- |
| `Super+1` … `Super+0` | Jump to workspace 1–10 |
| `Super+Shift+1` … `Super+Shift+0` | Move active window to workspace 1–10 |
| `Ctrl+Super+1` … `Ctrl+Super+0` | Jump to workspace group 1–10 |
| `Ctrl+Super+Alt+1` … `Ctrl+Super+Alt+0` | Move active window to workspace group 1–10 |
| `Ctrl+Super+Right` | Next workspace |
| `Ctrl+Super+Left` | Previous workspace |
| `Super+Tab` | Toggle special workspace |
| `Super+Page_Up` / `Super+Page_Down` | Previous / next workspace (repeats) |
| `Super+ScrollUp/Down` | Previous / next workspace |
| `Ctrl+Super+ScrollUp/Down` | Move ±10 workspaces |
| `Super+Alt+Page_Up/Down` | Move window ±1 workspace |
| `Super+Shift+ScrollUp/Down` | Move window ±1 workspace |
| `Ctrl+Super+Shift+Right/Left` | Move window ±1 workspace |
| `Ctrl+Super+Shift+Up` | Move window to special:special |
| `Ctrl+Super+Shift+Down` | Move window to current ws |
| `Super+Alt+S` | Move window to special:special |

### Special workspaces (toggled overlays)

| Keys | Action |
| --- | --- |
| `Ctrl+Shift+Alt+Escape` | System monitor overlay |
| `Super+M` | Music overlay |
| `Super+D` | Communication overlay |
| `Super+T` | Todo overlay |

### Window operations

| Keys | Action |
| --- | --- |
| `Super+Q` | Close active window (delegates to `close-active.sh`) |
| `Super+Space` | Toggle floating |
| `Super+P` | Pin window |
| `Ctrl+Shift+Return` | Fullscreen |
| `Super+Shift+Return` | Bordered fullscreen |
| `Super+Z` (held) | Move window with mouse |
| `Super+X` (held) | Resize window with mouse |
| `Super+LMB` (held) | Move window with mouse (mouse:272) |
| `Super+RMB` (held) | Resize window with mouse (mouse:273) |
| `Super+Alt+Backslash` | Picture-in-Picture |
| `Ctrl+Super+Backslash` | Center window |
| `Ctrl+Super+Alt+Backslash` | Resize to 55%×70% and center |
| `Super+Left/Right/Up/Down` | Focus window in direction |
| `Super+Shift+Left/Right/Up/Down` | Move window in direction |
| `Super+Minus` / `Super+Equal` | Resize horizontally ∓10% |
| `Super+Shift+Minus` / `Super+Shift+Equal` | Resize vertically ∓10% |
| `Super+Alt+Left/Right/Up/Down` | Resize active window |

### Window groups (tabbed)

| Keys | Action |
| --- | --- |
| `Super+Comma` | Toggle group |
| `Super+U` | Ungroup |
| `Alt+Tab` (held) | Cycle next in group |
| `Shift+Alt+Tab` (held) | Cycle previous in group |
| `Ctrl+Alt+Tab` (held) | Switch to next group member |
| `Ctrl+Shift+Alt+Tab` (held) | Switch to previous group member |
| `Super+Shift+Comma` | Lock active group |

### Audio & media (hardware keys)

| Keys | Action |
| --- | --- |
| `XF86AudioMute` | Toggle sink mute |
| `XF86AudioMicMute` | Toggle source mute |
| `Super+Shift+M` | Toggle sink mute |
| `XF86AudioRaiseVolume` (held) | Raise volume |
| `XF86AudioLowerVolume` (held) | Lower volume |

> Media play/pause/next/prev keybinds are **shell-specific** - see the
> per-shell sections below, because each shell wires its own IPC layer.

### System

| Keys | Action |
| --- | --- |
| `Super+Shift+L` | Suspend then hibernate |
| `Ctrl+Shift+Alt+V` | Paste last clipboard entry via ydotool (workaround) |

---

## Caelestia

Bindings specific to **caelestia-shell**, layered on top of Common.
Wired through caelestia's `global, caelestia:*` dispatchers and CLI helpers.

> Source: `Linux/dots/hypr/caelestia/keybinds.conf`,
> `Linux/dots/hypr/caelestia/launcher.conf`.

### Launcher (Super long-press)

Caelestia opens its launcher when you **press and hold `Super`** alone.
A short tap is interpreted as a modifier (cancelled by any other key).

| Trigger | Action |
| --- | --- |
| `Super` (immediate) | Open launcher (`caelestia:launcher`) |
| `Super` + any other key / mouse | Cancel launcher (`caelestia:launcherInterrupt`) |

Interrupt is wired for: catchall keys, all mouse buttons
(`mouse:272`–`mouse:277`), and scroll up/down.

### Shell panels

| Keys | Action |
| --- | --- |
| `Ctrl+Alt+Delete` | Session menu (`caelestia:session`) |
| `Super+N` | Show / hide sidebar (`caelestia:sidebar`) |
| `Ctrl+Alt+C` | Clear notifications (`caelestia:clearNotifs`) |
| `Super+K` | Show all panels (`caelestia:showall`) |
| `Super+L` | Lock screen (`caelestia:lock`) |
| `Super+Alt+L` | Restore lock state from last session |

### Brightness

| Keys | Action |
| --- | --- |
| `XF86MonBrightnessUp` | Brightness up |
| `XF86MonBrightnessDown` | Brightness down |

### Media

| Keys | Action |
| --- | --- |
| `Ctrl+Super+Space` | Play / pause (`caelestia:mediaToggle`) |
| `XF86AudioPlay` / `XF86AudioPause` | Play / pause |
| `Ctrl+Super+Equal` | Next track (`caelestia:mediaNext`) |
| `XF86AudioNext` | Next track |
| `Ctrl+Super+Minus` | Previous track (`caelestia:mediaPrev`) |
| `XF86AudioPrev` | Previous track |
| `XF86AudioStop` | Stop |

### Caelestia helpers

| Keys | Action |
| --- | --- |
| `Ctrl+Super+Shift+R` (release) | Kill caelestia shell |
| `Ctrl+Super+Alt+R` (release) | Restart caelestia shell |
| `Super+Alt+R` | Start screen recording (`caelestia record -s`) |
| `Ctrl+Alt+R` | Start screen recording (default) |
| `Super+Shift+Alt+R` | Caelestia record helper (`-r`) |

### Window-specific

| Keys | Action |
| --- | --- |
| `Super+Alt+Backslash` (from $kbWindowPip) | Resize active window to PiP |

### Clipboard & emoji

| Keys | Action |
| --- | --- |
| `Super+V` | Toggle caelestia clipboard (`pkill fuzzel` first) |
| `Super+Alt+V` | Caelestia clipboard with `-d` (delete entries) |
| `Super+Period` | Caelestia emoji picker (`-p`) |

---

## Noctalia

Bindings specific to **noctalia**, layered on top of Common.
Wired through `noctalia msg <command>`.

> Source: `Linux/dots/hypr/noctalia/keybinds.conf`,
> `Linux/dots/hypr/noctalia/launcher.conf`,
> `Linux/dots/hypr/scripts/noctalia-launcher.sh`.

### Launcher (Super long-press)

Noctalia opens its launcher through a press/release dance on **Super alone**,
wired via `noctalia-launcher.sh`. The script tracks press, release, and any
interrupting input.

| Trigger | Action |
| --- | --- |
| `Super` press | `noctalia-launcher.sh press` (arms launcher) |
| `Super` release | `noctalia-launcher.sh release` (opens launcher if no interrupt) |
| `Super` + any other key / mouse | `noctalia-launcher.sh interrupt` (cancels) |

Interrupt is wired for: catchall keys, all mouse buttons
(`mouse:272`–`mouse:277`), and scroll up/down.

### Shell panels

| Keys | Action |
| --- | --- |
| `Ctrl+Alt+Delete` | Session menu toggle |
| `Super+N` | Control center toggle |
| `Ctrl+Alt+C` | Clear notifications |
| `Super+K` | Settings toggle |
| `Super+L` | Lock screen |
| `Super+Alt+L` | Restore lock state from last session |

### Brightness

| Keys | Action |
| --- | --- |
| `XF86MonBrightnessUp` | Brightness increase |
| `XF86MonBrightnessDown` | Brightness decrease |

### Media

| Keys | Action |
| --- | --- |
| `Ctrl+Super+Space` | Play / pause |
| `XF86AudioPlay` / `XF86AudioPause` | Play / pause |
| `Ctrl+Super+Equal` | Next track |
| `XF86AudioNext` | Next track |
| `Ctrl+Super+Minus` | Previous track |
| `XF86AudioPrev` | Previous track |
| `XF86AudioStop` | Stop |

### Recording

| Keys | Action |
| --- | --- |
| `Super+R` | Toggle screen recording (`record-toggle.sh`) |

### Clipboard & emoji

| Keys | Action |
| --- | --- |
| `Super+V` | Noctalia clipboard launcher |
| `Super+Alt+V` | Noctalia clipboard launcher (same; placeholder for delete) |
| `Super+Period` | Noctalia emoji launcher |

---

## End4 (Illogical Impulse)

Bindings specific to **end-4 / Illogical Impulse**, layered on top of Common.
The end4 profile uses upstream end-4 dotfiles patched by
`Linux/NixOS/home/end4/patches/hypr.nix` and overlays Wahrwelt-specific
keybinds from `Linux/dots/hypr/end4/keybinds.conf`.

> Sources: `Linux/dots/hypr/end4/keybinds.conf`,
> upstream `end-4/dots-hyprland`.

### Launcher

End4 ships its own QuickShell launcher tied to **Super long-press** via
upstream `end-4/dots-hyprland`. Wahrwelt leaves the upstream wiring alone -
no per-shell launcher.conf is needed (`Linux/dots/hypr/end4/launcher.conf`
is intentionally empty; the active `$hypr/shell-launcher.conf` symlink
points to the upstream config while end4 is active).

| Trigger | Action |
| --- | --- |
| `Super` long-press | Open end4 launcher (handled by upstream IPC) |

### Wahrwelt overrides

| Keys | Action |
| --- | --- |
| `Ctrl+Super+Alt+Slash` | Open this keybinds source in editor (`xdg-open ../wahrwelt/keybinds.conf`) |

### Unbound upstream defaults

These upstream end-4 bindings are explicitly cleared before sourcing the
shared Wahrwelt set, to avoid double-binds and conflicts:

| Cleared |
| --- |
| `Super+Return` |
| `Super+E` |
| `Super+Shift+C` |
| `Super+Shift+A` |
| `Super+Shift+N` |
| `Super+Shift+T` |
| `Super+Shift+B` |
| `Ctrl+Shift+Escape` |
| `Super+S` |
| `Super+Shift+S` |
| `Super+C` |
| `Super+F` |
| `Super+X` |
| `Super+Ctrl+Space` |

After clearing, end4 sources `shell-common-keybinds.conf` and
`shell-workspace-keybinds.conf` - see the [Common](#common-all-shells)
section above for the resulting set.

### Workspaces (end4 extras)

| Keys | Action |
| --- | --- |
| `$kbNextWs` (`Ctrl+Super+Right`) | Next workspace |
| `$kbPrevWs` (`Ctrl+Super+Left`) | Previous workspace |

### Window operations (end4 extras)

| Keys | Action |
| --- | --- |
| `$kbMoveWindow` (`Super+Z`, held) | Move window |
| `$kbResizeWindow` (`Super+X`, held) | Resize window |
| `$kbCloseWindow` (`Super+Q`) | Close active window |
| `$kbToggleWindowFloating` (`Super+Space`) | Toggle floating |
| `$kbPinWindow` (`Super+P`) | Pin window |
| `$kbWindowFullscreen` (`Ctrl+Shift+Return`) | Fullscreen |
| `Super+Ctrl+Return` | Fullscreen (extra alias) |
| `$kbWindowBorderedFullscreen` (`Super+Shift+Return`) | Bordered fullscreen |

### Notes

- All other end-4 panels, AI sidebar, anime corner, and shell-internal
  shortcuts come from the upstream `end-4/dots-hyprland` config. They are
  not duplicated here - refer to the upstream wiki for shell-internal
  bindings: <https://end-4.github.io/dots-hyprland-wiki/>.
- Wahrwelt keeps end4 mutable runtime settings under
  `~/.config/illogical-impulse`, so shell-side JSON changes can persist
  without a rebuild.
