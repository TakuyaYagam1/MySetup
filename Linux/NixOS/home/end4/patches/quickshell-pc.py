#!/usr/bin/env python3
"""Patch end4-pC for immutable Wahrwelt-managed installation."""

from pathlib import Path
import sys


def replace_function(text: str, start: str, end: str, replacement: str) -> str:
    start_index = text.find(start)
    if start_index < 0:
        raise SystemExit(f"start marker not found: {start.strip()}")

    end_index = text.find(end, start_index)
    if end_index < 0:
        raise SystemExit(f"end marker not found: {end.strip()}")

    return text[:start_index] + replacement + text[end_index:]


def patch_about(root: Path, updater_notice: str) -> None:
    path = root / "modules/ii/settings/pages/About.qml"
    text = path.read_text()

    system_update = """    function runSystemUpdate() {
        Quickshell.execDetached(["foot", "nixos-update"])
        Qt.callLater(() => GlobalStates.settingsOpen = false)
    }

"""
    text = replace_function(
        text,
        "    function runSystemUpdate() {",
        "    function runUpdateDots() {",
        system_update,
    )

    dots_update = f"""    function runUpdateDots() {{
        Quickshell.execDetached([
            "notify-send",
            "Wahrwelt",
            {updater_notice!r}
        ])
        Qt.callLater(() => GlobalStates.settingsOpen = false)
    }}

"""
    text = replace_function(
        text,
        "    function runUpdateDots() {",
        "    Rectangle {",
        dots_update,
    )

    path.write_text(text)


def patch_updates_count(root: Path) -> None:
    path = root / "modules/ii/bar/UpdatesCount.qml"
    text = path.read_text()
    old = """        command: [
            "kitty", "--hold",
            "fish", "-i", "-l", "-c",
            "yay -Syu --combinedupgrade=false"
        ]"""
    new = '        command: ["foot", "nixos-update"]'
    if old not in text:
        raise SystemExit("UpdatesCount Arch update command not found")
    path.write_text(text.replace(old, new, 1))


def main() -> None:
    if len(sys.argv) != 3:
        raise SystemExit(
            "usage: quickshell-pc.py <patched-quickshell-root> <updater-notice>"
        )

    root = Path(sys.argv[1])
    patch_about(root, sys.argv[2])
    patch_updates_count(root)


if __name__ == "__main__":
    main()
