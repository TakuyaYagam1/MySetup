#!/usr/bin/env bash
set -euo pipefail

exec python3 -I -S - "$@" <<'PY'
import ctypes
import errno
import fcntl
import hashlib
import os
import re
import secrets
import signal
import stat
import subprocess
import sys


AT_EMPTY_PATH = 0x1000
RENAME_EXCHANGE = 0x2
HISTORICAL_ADAPTER_DIGESTS = {
    "cecf44b96c7afd4886d498abe0de382b2574c66281a5cf78bbac06586c1b071c",
    "e28d16bde1d68fa2fa43c755630284f00b3c6a14f75656e89cfb5514f8633263",
    "18c3eb7f48101e0bd0b57918a683778784c74c833a215af7f7b0f1d416a0a5df",
    "24229642cd871aa3eb3d27c44b0d72357395951aec076a09d173b45ca17231a0",
    "1d8e001faf0c6078a7d9a34e4c592fcb523afd817d2ff56099c7b2fe16407506",
    "a547d710e9fd13ca8829e17caa378a14ee9d6a0d114426731e0ab363e9328118",
    "3666c398dbba460e9b3dac54f396a7f53ad2093f49967c05e4588e66c41f08eb",
}
NIX_STORE_OBJECT_RE = re.compile(
    r"^(/nix/store/[0-9abcdfghijklmnpqrsvwxyz]{32}-[^/]+)(/.*)?$"
)
NIXOS_HOME_MANAGER_ADAPTER_RE = re.compile(
    r"^/nix/store/[0-9abcdfghijklmnpqrsvwxyz]{32}-home-manager-files/"
    r"\.config/hypr/(?:user|wahrwelt|mysetup)/hyprland\.lua$"
)
NIXOS_HOME_MANAGER_TOP_LEVEL_RUNTIME_RE = re.compile(
    r"^/nix/store/[0-9abcdfghijklmnpqrsvwxyz]{32}-home-manager-files/"
    r"\.config/hypr/hyprland\.lua$"
)

libc = ctypes.CDLL(None, use_errno=True)
linkat = libc.linkat
linkat.argtypes = [ctypes.c_int, ctypes.c_char_p, ctypes.c_int, ctypes.c_char_p, ctypes.c_int]
linkat.restype = ctypes.c_int
renameat2 = libc.renameat2
renameat2.argtypes = [ctypes.c_int, ctypes.c_char_p, ctypes.c_int, ctypes.c_char_p, ctypes.c_uint]
renameat2.restype = ctypes.c_int


class ActivationError(RuntimeError):
    pass


def ownership_collision(label, path, detail=None):
    message = f"Wahrwelt {label} ownership collision: {path}"
    if detail:
        message += f" ({detail})"
    raise ActivationError(message)


def identity(info):
    return info.st_dev, info.st_ino


def validate_chain(pinned, label, target):
    for path, expected in pinned:
        try:
            current = os.lstat(path)
        except FileNotFoundError:
            ownership_collision(label, target, f"pinned directory disappeared: {path}")
        if not stat.S_ISDIR(current.st_mode) or identity(current) != expected:
            ownership_collision(label, target, f"pinned directory changed: {path}")


def pin_directory(path, label, create):
    normalized = os.path.normpath(os.path.abspath(path))
    parts = [] if normalized == "/" else [part for part in normalized.split(os.sep) if part]
    flags = os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW | os.O_CLOEXEC
    current_fd = os.open("/", flags)
    pinned = [("/", identity(os.fstat(current_fd)))]
    prefix = ""
    try:
        for part in parts:
            prefix += "/" + part
            try:
                next_fd = os.open(part, flags, dir_fd=current_fd)
            except FileNotFoundError:
                if not create:
                    ownership_collision(label, normalized, f"missing directory {prefix}")
                try:
                    os.mkdir(part, 0o755, dir_fd=current_fd)
                except FileExistsError:
                    pass
                try:
                    created = os.stat(part, dir_fd=current_fd, follow_symlinks=False)
                except FileNotFoundError:
                    ownership_collision(label, normalized, f"created directory disappeared: {prefix}")
                if not stat.S_ISDIR(created.st_mode):
                    ownership_collision(label, normalized, f"created path is not a directory: {prefix}")
                if prefix == normalized:
                    test_barrier("DIRECTORY")
                try:
                    next_fd = os.open(part, flags, dir_fd=current_fd)
                except OSError as error:
                    ownership_collision(label, normalized, f"cannot pin created directory {prefix}: {error}")
                if identity(os.fstat(next_fd)) != identity(created):
                    os.close(next_fd)
                    ownership_collision(label, normalized, f"created directory changed before pin: {prefix}")
                os.fsync(current_fd)
            except OSError as error:
                ownership_collision(label, normalized, f"cannot pin directory {prefix}: {error}")
            os.close(current_fd)
            current_fd = next_fd
            pinned.append((prefix, identity(os.fstat(current_fd))))
        validate_chain(pinned, label, normalized)
        return normalized, current_fd, pinned
    except BaseException:
        os.close(current_fd)
        raise


def classify_leaf(parent_fd, name, label, display_path):
    try:
        current = os.stat(name, dir_fd=parent_fd, follow_symlinks=False)
    except FileNotFoundError:
        return "absent", None
    if stat.S_ISLNK(current.st_mode):
        return "symlink", current
    if stat.S_ISREG(current.st_mode):
        return "regular", current
    raise ActivationError(f"Refusing non-regular Wahrwelt {label} collision: {display_path}")


def read_fd(fd):
    os.lseek(fd, 0, os.SEEK_SET)
    chunks = []
    while True:
        chunk = os.read(fd, 64 * 1024)
        if not chunk:
            return b"".join(chunks)
        chunks.append(chunk)


def read_regular(path, label):
    try:
        fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW | os.O_CLOEXEC)
    except OSError as error:
        ownership_collision(label, path, f"cannot open managed source: {error}")
    try:
        info = os.fstat(fd)
        if not stat.S_ISREG(info.st_mode):
            ownership_collision(label, path, "managed source is not regular")
        return read_fd(fd)
    finally:
        os.close(fd)


def write_anonymous(parent_fd, payload, mode, label, display_path):
    try:
        fd = os.open(".", os.O_TMPFILE | os.O_WRONLY | os.O_CLOEXEC, 0o600, dir_fd=parent_fd)
    except OSError as error:
        ownership_collision(label, display_path, f"cannot create anonymous candidate: {error}")
    try:
        view = memoryview(payload)
        while view:
            written = os.write(fd, view)
            if written == 0:
                raise ActivationError(f"short write while preparing {display_path}")
            view = view[written:]
        os.fchmod(fd, mode)
        os.fsync(fd)
        return fd, os.fstat(fd)
    except BaseException:
        os.close(fd)
        raise


def materialize_anonymous(parent_fd, payload_fd, prefix, label, display_path):
    for _ in range(128):
        name = f".{prefix}.wahrwelt-recovery.{os.getpid()}.{secrets.token_hex(8)}"
        if linkat(payload_fd, b"", parent_fd, os.fsencode(name), AT_EMPTY_PATH) == 0:
            return name
        link_error = ctypes.get_errno()
        if link_error == errno.EEXIST:
            continue
        ownership_collision(label, display_path, f"cannot materialize anonymous candidate: {os.strerror(link_error)}")
    ownership_collision(label, display_path, "cannot allocate unique recovery name")


def test_barrier(prefix):
    ready = os.environ.get(f"WAHRWELT_TEST_{prefix}_READY_FD")
    proceed = os.environ.get(f"WAHRWELT_TEST_{prefix}_CONTINUE_FD")
    if ready is None and proceed is None:
        return
    if ready is None or proceed is None:
        raise ActivationError(f"incomplete {prefix.lower()} test barrier")
    try:
        ready_fd = int(ready)
        proceed_fd = int(proceed)
    except ValueError:
        raise ActivationError(f"invalid {prefix.lower()} test barrier") from None
    os.write(ready_fd, b"ready\n")
    if os.read(proceed_fd, 1) != b"1":
        raise ActivationError(f"closed {prefix.lower()} test barrier")


def seed_exclusive_at(parent_fd, name, source, label, display_path, pinned):
    validate_chain(pinned, label, display_path)
    classification, _ = classify_leaf(parent_fd, name, label, display_path)
    if classification in {"regular", "symlink"}:
        return False
    payload = read_regular(source, label)
    payload_fd, payload_info = write_anonymous(parent_fd, payload, 0o644, label, display_path)
    try:
        test_barrier("SEED")
        validate_chain(pinned, label, display_path)
        classification, _ = classify_leaf(parent_fd, name, label, display_path)
        if classification in {"regular", "symlink"}:
            return False
        if linkat(payload_fd, b"", parent_fd, os.fsencode(name), AT_EMPTY_PATH) != 0:
            link_error = ctypes.get_errno()
            if link_error == errno.EEXIST:
                classification, _ = classify_leaf(parent_fd, name, label, display_path)
                if classification in {"regular", "symlink"}:
                    return False
            ownership_collision(label, display_path, f"exclusive publication failed: {os.strerror(link_error)}")
        published = os.stat(name, dir_fd=parent_fd, follow_symlinks=False)
        if not stat.S_ISREG(published.st_mode) or identity(published) != identity(payload_info):
            ownership_collision(label, display_path, "published inode changed")
        validate_chain(pinned, label, display_path)
        os.fsync(parent_fd)
        return True
    finally:
        os.close(payload_fd)


def exchange(parent_fd, first, second, label, display_path):
    if renameat2(parent_fd, os.fsencode(first), parent_fd, os.fsencode(second), RENAME_EXCHANGE) != 0:
        exchange_error = ctypes.get_errno()
        ownership_collision(label, display_path, f"atomic exchange failed: {os.strerror(exchange_error)}")


def uncertain_exchange(label, display_path, recovery_name, detail):
    recovery_path = os.path.join(os.path.dirname(display_path), recovery_name)
    raise ActivationError(
        f"Wahrwelt {label} ownership collision: {display_path} ({detail}); "
        f"retained both {display_path} and {recovery_path}"
    )


def read_leaf(parent_fd, name, label, display_path):
    try:
        fd = os.open(name, os.O_RDONLY | os.O_NOFOLLOW | os.O_CLOEXEC, dir_fd=parent_fd)
    except OSError as error:
        ownership_collision(label, display_path, f"cannot pin regular leaf: {error}")
    try:
        info = os.fstat(fd)
        if not stat.S_ISREG(info.st_mode):
            ownership_collision(label, display_path, "leaf is no longer regular")
        return info, read_fd(fd)
    finally:
        os.close(fd)


def exact_regular_leaf(parent_fd, name, expected_info, expected_content, label, display_path):
    try:
        current_info, current_content = read_leaf(parent_fd, name, label, display_path)
    except ActivationError:
        return False
    return identity(current_info) == identity(expected_info) and current_content == expected_content


def adapter_token(parent_fd, name, label, display_path):
    classification, info = classify_leaf(parent_fd, name, label, display_path)
    if classification == "absent":
        return ("absent",)
    if classification == "symlink":
        return ("symlink", info.st_dev, info.st_ino, os.readlink(name, dir_fd=parent_fd))
    regular_info, content = read_leaf(parent_fd, name, label, display_path)
    return (
        "regular",
        regular_info.st_dev,
        regular_info.st_ino,
        hashlib.sha256(content).digest(),
    )


def validate_adapter_token(parent_fd, name, expected, label, display_path):
    current = adapter_token(parent_fd, name, label, display_path)
    if current != expected:
        ownership_collision(label, display_path, "adapter changed after guarded preparation")


def classify_top_level_runtime_at(
    parent_fd,
    name,
    old_generation,
    expected_relative,
    stable_source,
    direct_sources,
    pinned,
    display_path,
):
    label = "top-level Hyprland runtime"
    validate_chain(pinned, label, display_path)
    classification, initial = classify_leaf(parent_fd, name, label, display_path)
    if classification == "absent":
        return "absent"

    if classification == "regular":
        opened, content = read_leaf(parent_fd, name, label, display_path)
        stable_content = read_regular(stable_source, label)
        if content == stable_content:
            if opened.st_nlink != 1:
                ownership_collision(label, display_path, "managed stable entrypoint has external hardlinks")
            validate_chain(pinned, label, display_path)
            current = os.stat(name, dir_fd=parent_fd, follow_symlinks=False)
            if identity(current) != identity(opened):
                ownership_collision(label, display_path, "managed stable entrypoint changed during classification")
            return f"stable-regular|{opened.st_dev}|{opened.st_ino}|{hashlib.sha256(content).hexdigest()}"
        known_contents = [read_regular(source, label) for source in direct_sources]
        if content not in known_contents:
            ownership_collision(label, display_path, "unknown regular entrypoint")
        if opened.st_nlink != 1:
            ownership_collision(label, display_path, "managed direct entrypoint has external hardlinks")
        validate_chain(pinned, label, display_path)
        current = os.stat(name, dir_fd=parent_fd, follow_symlinks=False)
        if identity(current) != identity(opened):
            ownership_collision(label, display_path, "managed direct entrypoint changed during classification")
        source_index = known_contents.index(content) + 1
        return (
            f"direct-regular|{source_index}|{opened.st_dev}|{opened.st_ino}|"
            f"{hashlib.sha256(content).hexdigest()}"
        )

    normalized_relative = os.path.normpath(expected_relative)
    if os.path.isabs(expected_relative) or normalized_relative in {".", ".."} or normalized_relative.startswith(".." + os.sep):
        ownership_collision(label, display_path, "invalid managed relative path")
    raw_target = os.readlink(name, dir_fd=parent_fd)
    expected = None
    if old_generation:
        home_files = os.path.realpath(os.path.join(os.path.abspath(old_generation), "home-files"))
        old_generation_target = os.path.join(home_files, normalized_relative)
        if raw_target == old_generation_target:
            expected = old_generation_target
    if expected is None and (
        normalized_relative == ".config/hypr/hyprland.lua"
        and NIXOS_HOME_MANAGER_TOP_LEVEL_RUNTIME_RE.fullmatch(raw_target) is not None
        and immutable_nix_store_leaf(raw_target, True)
        and immutable_nix_store_leaf(os.path.realpath(raw_target), False)
    ):
        expected = raw_target
    if expected is None:
        ownership_collision(label, display_path, "symlink is not owned by Home Manager")
    try:
        expected_info = os.stat(expected, follow_symlinks=True)
        linked_info = os.stat(name, dir_fd=parent_fd, follow_symlinks=True)
    except OSError as error:
        ownership_collision(label, display_path, f"managed symlink target is unavailable: {error}")
    if (
        not stat.S_ISREG(expected_info.st_mode)
        or not stat.S_ISREG(linked_info.st_mode)
        or identity(expected_info) != identity(linked_info)
    ):
        ownership_collision(label, display_path, "managed symlink does not resolve to the expected regular entrypoint")
    try:
        linked_fd = os.open(name, os.O_RDONLY | os.O_CLOEXEC, dir_fd=parent_fd)
    except OSError as error:
        ownership_collision(label, display_path, f"cannot pin managed symlink target: {error}")
    try:
        opened = os.fstat(linked_fd)
        content = read_fd(linked_fd)
    finally:
        os.close(linked_fd)
    expected_content = read_regular(stable_source, label)
    if identity(opened) != identity(expected_info) or content != expected_content:
        ownership_collision(label, display_path, "managed symlink target is not the exact stable delegator")
    validate_chain(pinned, label, display_path)
    current = os.stat(name, dir_fd=parent_fd, follow_symlinks=False)
    if identity(current) != identity(initial) or os.readlink(name, dir_fd=parent_fd) != raw_target:
        ownership_collision(label, display_path, "managed symlink changed during classification")
    return (
        f"stable-link|{initial.st_dev}|{initial.st_ino}|"
        f"{opened.st_dev}|{opened.st_ino}|{hashlib.sha256(content).hexdigest()}"
    )


def classify_direct_runtime_at(parent_fd, name, direct_sources, pinned, display_path):
    label = "Hyprland runtime"
    validate_chain(pinned, label, display_path)
    classification, initial = classify_leaf(parent_fd, name, label, display_path)
    if classification == "absent":
        return "absent"
    if classification == "symlink":
        ownership_collision(label, display_path, "runtime entrypoint is a symlink")
    opened, content = read_leaf(parent_fd, name, label, display_path)
    known_contents = [read_regular(source, label) for source in direct_sources]
    if content not in known_contents:
        ownership_collision(label, display_path, "unknown regular runtime entrypoint")
    if opened.st_nlink != 1:
        ownership_collision(label, display_path, "managed direct entrypoint has external hardlinks")
    validate_chain(pinned, label, display_path)
    current, current_content = read_leaf(parent_fd, name, label, display_path)
    if identity(current) != identity(opened) or current_content != content or current.st_nlink != 1:
        ownership_collision(label, display_path, "managed direct entrypoint changed during classification")
    source_index = known_contents.index(content) + 1
    return (
        f"direct-regular|{source_index}|{opened.st_dev}|{opened.st_ino}|"
        f"{hashlib.sha256(content).hexdigest()}"
    )


def exact_runtime_token_at(parent_fd, name, source, pinned, display_path):
    label = "Hyprland runtime"
    validate_chain(pinned, label, display_path)
    classification, _ = classify_leaf(parent_fd, name, label, display_path)
    if classification == "absent":
        return "absent"
    if classification == "symlink":
        ownership_collision(label, display_path, "prepared runtime is a symlink")
    opened, content = read_leaf(parent_fd, name, label, display_path)
    expected = read_regular(source, label)
    if content != expected:
        ownership_collision(label, display_path, "prepared runtime content is not exact")
    if opened.st_nlink != 1:
        ownership_collision(label, display_path, "prepared runtime has external hardlinks")
    validate_chain(pinned, label, display_path)
    current, current_content = read_leaf(parent_fd, name, label, display_path)
    if identity(current) != identity(opened) or current_content != content or current.st_nlink != 1:
        ownership_collision(label, display_path, "prepared runtime changed during token capture")
    return f"regular|{opened.st_dev}|{opened.st_ino}|{hashlib.sha256(content).hexdigest()}"


def managed_regular_token(parent_fd, name, source, label, display_path):
    classification, _ = classify_leaf(parent_fd, name, label, display_path)
    if classification != "regular":
        ownership_collision(label, display_path, "managed entrypoint is not regular")
    info, content = read_leaf(parent_fd, name, label, display_path)
    expected = read_regular(source, label)
    if content != expected:
        ownership_collision(label, display_path, "managed entrypoint content is not canonical")
    return info.st_dev, info.st_ino, hashlib.sha256(content).digest()


def immutable_nix_store_leaf(path, allow_symlink):
    matched = NIX_STORE_OBJECT_RE.fullmatch(path)
    if matched is None:
        return False
    object_path = matched.group(1)
    suffix = matched.group(2)
    leaf = object_path
    if suffix:
        try:
            root_info = os.lstat(object_path)
        except OSError:
            return False
        if (
            not stat.S_ISDIR(root_info.st_mode)
            or root_info.st_uid != 0
            or root_info.st_mode & 0o022
        ):
            return False
        current = object_path
        components = [component for component in suffix.split(os.sep) if component]
        if not components:
            return False
        for component in components[:-1]:
            current = os.path.join(current, component)
            try:
                current_info = os.lstat(current)
            except OSError:
                return False
            if (
                not stat.S_ISDIR(current_info.st_mode)
                or current_info.st_uid != 0
                or current_info.st_mode & 0o022
            ):
                return False
        leaf = os.path.join(current, components[-1])
    try:
        leaf_info = os.lstat(leaf)
    except OSError:
        return False
    if leaf_info.st_uid != 0:
        return False
    if stat.S_ISLNK(leaf_info.st_mode):
        return allow_symlink
    return stat.S_ISREG(leaf_info.st_mode) and not leaf_info.st_mode & 0o222


def validate_nixos_home_manager_adapter_link(
    parent_fd,
    name,
    current_source,
    pinned,
    label,
    display_path,
):
    raw_target = os.readlink(name, dir_fd=parent_fd)
    if NIXOS_HOME_MANAGER_ADAPTER_RE.fullmatch(raw_target) is None:
        return False
    resolved_target = os.path.realpath(raw_target)
    if not immutable_nix_store_leaf(raw_target, True) or not immutable_nix_store_leaf(
        resolved_target, False
    ):
        return False
    try:
        linked_fd = os.open(name, os.O_RDONLY | os.O_CLOEXEC, dir_fd=parent_fd)
    except OSError:
        return False
    try:
        opened = os.fstat(linked_fd)
        content = read_fd(linked_fd)
    finally:
        os.close(linked_fd)
    if not stat.S_ISREG(opened.st_mode):
        return False
    content_digest = hashlib.sha256(content).hexdigest()
    current_content = read_regular(current_source, label)
    if content != current_content and content_digest not in HISTORICAL_ADAPTER_DIGESTS:
        return False
    try:
        raw_info = os.stat(raw_target, follow_symlinks=True)
        linked_info = os.stat(name, dir_fd=parent_fd, follow_symlinks=True)
    except OSError:
        return False
    validate_chain(pinned, label, display_path)
    if identity(raw_info) != identity(opened) or identity(linked_info) != identity(opened):
        return False
    return True


def validate_home_manager_adapter_link(
    parent_fd,
    name,
    old_generation,
    current_source,
    pinned,
    label,
    display_path,
):
    if validate_nixos_home_manager_adapter_link(
        parent_fd,
        name,
        current_source,
        pinned,
        label,
        display_path,
    ):
        return
    if not old_generation:
        ownership_collision(label, display_path, "unowned adapter symlink")
    home_files = os.path.realpath(os.path.join(old_generation, "home-files"))
    if not os.path.isdir(home_files):
        ownership_collision(label, display_path, "old Home Manager generation is unavailable")
    raw_target = os.readlink(name, dir_fd=parent_fd)
    linked_info = os.stat(name, dir_fd=parent_fd, follow_symlinks=True)
    if not stat.S_ISREG(linked_info.st_mode):
        ownership_collision(label, display_path, "adapter symlink target is not regular")
    for namespace in ("user", "wahrwelt", "mysetup"):
        expected = os.path.join(home_files, ".config", "hypr", namespace, "hyprland.lua")
        if raw_target != expected:
            continue
        try:
            expected_info = os.stat(expected, follow_symlinks=True)
        except FileNotFoundError:
            continue
        if stat.S_ISREG(expected_info.st_mode) and identity(expected_info) == identity(linked_info):
            return
    ownership_collision(label, display_path, "unowned adapter symlink")


def prepare_user_adapter(parent_fd, current_source, old_generation, pinned, display_path):
    label = "Hypr user adapter"
    name = "hyprland.lua"
    validate_chain(pinned, label, display_path)
    classification, initial = classify_leaf(parent_fd, name, label, display_path)
    if classification == "absent":
        return adapter_token(parent_fd, name, label, display_path)
    if classification == "symlink":
        validate_home_manager_adapter_link(
            parent_fd,
            name,
            old_generation,
            current_source,
            pinned,
            label,
            display_path,
        )
        token = adapter_token(parent_fd, name, label, display_path)
        if token[0] != "symlink" or token[1:3] != identity(initial):
            ownership_collision(label, display_path, "adapter symlink changed during provenance check")
        return token

    opened, content = read_leaf(parent_fd, name, label, display_path)
    current_content = read_regular(current_source, label)
    if content == current_content:
        token = adapter_token(parent_fd, name, label, display_path)
        if token[0] != "regular" or token[3] != hashlib.sha256(current_content).digest():
            ownership_collision(label, display_path, "current adapter changed while capturing its token")
        return token
    digest = hashlib.sha256(content).hexdigest()
    if digest not in HISTORICAL_ADAPTER_DIGESTS:
        ownership_collision(label, display_path, "unknown regular adapter")
    if opened.st_nlink != 1:
        ownership_collision(label, display_path, "historical adapter has external hardlinks")

    candidate_fd, candidate_info = write_anonymous(parent_fd, current_content, 0o644, label, display_path)
    try:
        test_barrier("ADAPTER")
        validate_chain(pinned, label, display_path)
        before_exchange, before_content = read_leaf(parent_fd, name, label, display_path)
        if (
            identity(before_exchange) != identity(opened)
            or before_exchange.st_nlink != 1
            or before_content != content
        ):
            ownership_collision(label, display_path, "historical adapter changed at publication barrier")
        recovery_name = materialize_anonymous(parent_fd, candidate_fd, name, label, display_path)
        exchange(parent_fd, recovery_name, name, label, display_path)
        test_barrier("ADAPTER_EXCHANGE")
        published, published_content = read_leaf(parent_fd, name, label, display_path)
        recovery, recovery_content = read_leaf(parent_fd, recovery_name, label, display_path)
        if (
            identity(published) != identity(candidate_info)
            or published_content != current_content
            or identity(recovery) != identity(opened)
            or recovery_content != content
        ):
            try:
                current_published = os.stat(name, dir_fd=parent_fd, follow_symlinks=False)
                current_recovery = os.stat(recovery_name, dir_fd=parent_fd, follow_symlinks=False)
            except FileNotFoundError:
                uncertain_exchange(label, display_path, recovery_name, "uncertain adapter exchange outcome")
            if (
                identity(current_published) == identity(candidate_info)
                and exact_regular_leaf(parent_fd, recovery_name, opened, content, label, display_path)
            ):
                exchange(parent_fd, recovery_name, name, label, display_path)
                ownership_collision(label, display_path, "adapter changed during atomic exchange and was rolled back")
            uncertain_exchange(label, display_path, recovery_name, "adapter changed during atomic exchange")
        validate_chain(pinned, label, display_path)
        os.fsync(parent_fd)
        print(
            f"Wahrwelt retained the historical Hypr user adapter at {os.path.dirname(display_path)}/{recovery_name}",
            file=sys.stderr,
        )
        token = adapter_token(parent_fd, name, label, display_path)
        if token[0] != "regular" or token[3] != hashlib.sha256(current_content).digest():
            ownership_collision(label, display_path, "published adapter content changed before commit")
        return token
    finally:
        os.close(candidate_fd)


def migrate_known_runtime_at(parent_fd, name, canonical_source, known_sources, pinned, display_path):
    label = "Hyprland runtime"
    validate_chain(pinned, label, display_path)
    classification, initial = classify_leaf(parent_fd, name, label, display_path)
    if classification == "absent":
        return False
    if classification == "symlink":
        ownership_collision(label, display_path, "managed runtime entrypoint is a symlink")
    try:
        runtime_fd = os.open(name, os.O_RDWR | os.O_NOFOLLOW | os.O_CLOEXEC, dir_fd=parent_fd)
    except OSError as error:
        ownership_collision(label, display_path, f"cannot pin runtime: {error}")
    lease_acquired = False
    previous_sigio_handler = None
    previous_signal_mask = None
    lease_break_requested = False
    try:
        opened = os.fstat(runtime_fd)
        if not stat.S_ISREG(opened.st_mode) or identity(opened) != identity(initial):
            ownership_collision(label, display_path, "runtime changed while pinning")
        legacy_content = read_fd(runtime_fd)
        canonical_content = read_regular(canonical_source, label)
        if legacy_content == canonical_content:
            return False
        known_contents = [read_regular(source, label) for source in known_sources]
        if legacy_content not in known_contents:
            ownership_collision(label, display_path, "unknown regular runtime entrypoint")
        if opened.st_nlink != 1:
            ownership_collision(label, display_path, "recognized legacy runtime has external hardlinks")

        def handle_lease_break(_signum, _frame):
            nonlocal lease_break_requested
            lease_break_requested = True

        previous_sigio_handler = signal.getsignal(signal.SIGIO)
        signal.signal(signal.SIGIO, handle_lease_break)
        fcntl.fcntl(runtime_fd, fcntl.F_SETOWN, os.getpid())
        try:
            fcntl.fcntl(runtime_fd, fcntl.F_SETLEASE, fcntl.F_WRLCK)
        except OSError as error:
            ownership_collision(label, display_path, f"recognized legacy runtime is busy: {error}")
        lease_acquired = True
        opened = os.fstat(runtime_fd)
        current = os.stat(name, dir_fd=parent_fd, follow_symlinks=False)
        if opened.st_nlink != 1 or identity(current) != identity(opened) or read_fd(runtime_fd) != legacy_content:
            ownership_collision(label, display_path, "recognized legacy runtime changed before migration")
        previous_signal_mask = signal.pthread_sigmask(signal.SIG_BLOCK, {signal.SIGIO})
        if lease_break_requested:
            ownership_collision(label, display_path, "recognized legacy runtime lease was broken")

        candidate_fd, candidate_info = write_anonymous(parent_fd, canonical_content, 0o644, label, display_path)
        try:
            test_barrier("MIGRATION")
            validate_chain(pinned, label, display_path)
            current = os.stat(name, dir_fd=parent_fd, follow_symlinks=False)
            opened = os.fstat(runtime_fd)
            if (
                opened.st_nlink != 1
                or identity(current) != identity(opened)
                or read_fd(runtime_fd) != legacy_content
                or lease_break_requested
                or signal.SIGIO in signal.sigpending()
            ):
                ownership_collision(label, display_path, "recognized legacy runtime changed at publication barrier")
            recovery_name = materialize_anonymous(parent_fd, candidate_fd, name, label, display_path)
            exchange(parent_fd, recovery_name, name, label, display_path)
            test_barrier("MIGRATION_EXCHANGE")
            published, published_content = read_leaf(parent_fd, name, label, display_path)
            recovery = os.stat(recovery_name, dir_fd=parent_fd, follow_symlinks=False)
            recovery_content = read_fd(runtime_fd)
            if (
                identity(published) != identity(candidate_info)
                or published_content != canonical_content
                or identity(recovery) != identity(opened)
                or recovery_content != legacy_content
                or recovery.st_nlink != 1
            ):
                try:
                    current_published = os.stat(name, dir_fd=parent_fd, follow_symlinks=False)
                    current_recovery = os.stat(recovery_name, dir_fd=parent_fd, follow_symlinks=False)
                except FileNotFoundError:
                    uncertain_exchange(label, display_path, recovery_name, "uncertain atomic exchange outcome")
                if (
                    identity(current_published) == identity(candidate_info)
                    and identity(current_recovery) == identity(opened)
                    and read_fd(runtime_fd) == legacy_content
                ):
                    exchange(parent_fd, recovery_name, name, label, display_path)
                    ownership_collision(label, display_path, "runtime changed during atomic exchange and was rolled back")
                uncertain_exchange(label, display_path, recovery_name, "runtime changed during atomic exchange")
            if lease_break_requested or signal.SIGIO in signal.sigpending():
                current_published = os.stat(name, dir_fd=parent_fd, follow_symlinks=False)
                current_recovery = os.stat(recovery_name, dir_fd=parent_fd, follow_symlinks=False)
                if (
                    identity(current_published) != identity(candidate_info)
                    or identity(current_recovery) != identity(opened)
                    or read_fd(runtime_fd) != legacy_content
                ):
                    uncertain_exchange(label, display_path, recovery_name, "late writer found a changed exchange pair")
                exchange(parent_fd, recovery_name, name, label, display_path)
                restored = os.stat(name, dir_fd=parent_fd, follow_symlinks=False)
                if identity(restored) != identity(opened):
                    ownership_collision(label, display_path, "uncertain rollback after a late writer")
                ownership_collision(label, display_path, "recognized legacy runtime received a late writer")
            validate_chain(pinned, label, display_path)
            os.fsync(parent_fd)
            print(f"Wahrwelt retained the legacy Hyprland runtime at {os.path.dirname(display_path)}/{recovery_name}", file=sys.stderr)
            return True
        finally:
            os.close(candidate_fd)
    finally:
        if lease_acquired:
            try:
                fcntl.fcntl(runtime_fd, fcntl.F_SETLEASE, fcntl.F_UNLCK)
            except OSError:
                pass
        if previous_signal_mask is not None:
            signal.pthread_sigmask(signal.SIG_SETMASK, previous_signal_mask)
        if previous_sigio_handler is not None:
            signal.signal(signal.SIGIO, previous_sigio_handler)
        os.close(runtime_fd)


def simple_seed(source, target, label):
    target = os.path.abspath(target)
    _, parent_fd, pinned = pin_directory(os.path.dirname(target), label, create=False)
    try:
        seed_exclusive_at(parent_fd, os.path.basename(target), source, label, target, pinned)
    finally:
        os.close(parent_fd)


def simple_migrate(target, canonical_source, known_sources):
    target = os.path.abspath(target)
    _, parent_fd, pinned = pin_directory(os.path.dirname(target), "Hyprland runtime", create=False)
    try:
        migrate_known_runtime_at(parent_fd, os.path.basename(target), canonical_source, known_sources, pinned, target)
    finally:
        os.close(parent_fd)


def stage_known_runtime(target, canonical_source, known_sources):
    target = os.path.abspath(target)
    _, parent_fd, pinned = pin_directory(os.path.dirname(target), "Hyprland runtime", create=True)
    try:
        name = os.path.basename(target)
        migrate_known_runtime_at(parent_fd, name, canonical_source, known_sources, pinned, target)
        seed_exclusive_at(parent_fd, name, canonical_source, "Hyprland runtime", target, pinned)
        managed_regular_token(parent_fd, name, canonical_source, "Hyprland runtime", target)
        validate_chain(pinned, "Hyprland runtime", target)
    finally:
        os.close(parent_fd)


def simple_classify_top_level_runtime(target, old_generation, expected_relative, stable_source, direct_sources):
    target = os.path.abspath(target)
    _, parent_fd, pinned = pin_directory(os.path.dirname(target), "top-level Hyprland runtime", create=False)
    try:
        print(
            classify_top_level_runtime_at(
                parent_fd,
                os.path.basename(target),
                old_generation,
                expected_relative,
                stable_source,
                direct_sources,
                pinned,
                target,
            )
        )
    finally:
        os.close(parent_fd)


def simple_classify_direct_runtime(target, direct_sources):
    target = os.path.abspath(target)
    _, parent_fd, pinned = pin_directory(os.path.dirname(target), "Hyprland runtime", create=False)
    try:
        print(
            classify_direct_runtime_at(
                parent_fd,
                os.path.basename(target),
                direct_sources,
                pinned,
                target,
            )
        )
    finally:
        os.close(parent_fd)


def simple_exact_runtime_token(target, source):
    target = os.path.abspath(target)
    _, parent_fd, pinned = pin_directory(os.path.dirname(target), "Hyprland runtime", create=False)
    try:
        print(exact_runtime_token_at(parent_fd, os.path.basename(target), source, pinned, target))
    finally:
        os.close(parent_fd)


def activate_user_dir(args):
    if len(args) != 4:
        raise ActivationError("usage: activate-user-dir USER_DIR ADAPTER_SOURCE OLD_GENERATION DEFAULT_SOURCE")
    user_dir, current_source, old_generation, default_source = args
    user_dir, user_fd, pinned = pin_directory(user_dir, "Hypr user directory", create=True)
    try:
        test_barrier("ACTIVATION")
        validate_chain(pinned, "Hypr user directory", user_dir)
        target = os.path.join(user_dir, "hyprland.lua")
        prepared_adapter = prepare_user_adapter(user_fd, current_source, old_generation, pinned, target)
        seed_exclusive_at(user_fd, "default.lua", default_source, "user config", os.path.join(user_dir, "default.lua"), pinned)
        test_barrier("ADAPTER_FINAL")
        validate_adapter_token(user_fd, "hyprland.lua", prepared_adapter, "Hypr user adapter", target)
        validate_chain(pinned, "Hypr user directory", user_dir)
    finally:
        os.close(user_fd)


def direct_end4_provenance_token(path, end4_content, end4_pc_content, label):
    try:
        initial = os.lstat(path)
    except FileNotFoundError:
        return None
    if not stat.S_ISREG(initial.st_mode):
        return None
    try:
        fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW | os.O_CLOEXEC)
    except OSError:
        return None
    try:
        opened = os.fstat(fd)
        content = read_fd(fd)
    finally:
        os.close(fd)
    if content not in {end4_content, end4_pc_content}:
        return None
    if opened.st_nlink != 1 or identity(opened) != identity(initial):
        ownership_collision(label, path, "direct provenance has external hardlinks or changed")
    return opened.st_dev, opened.st_ino, hashlib.sha256(content).digest()


def resolved_regular_asset(path, label):
    try:
        initial = os.lstat(path)
    except OSError as error:
        ownership_collision(label, path, f"required runtime asset is unavailable: {error}")
    initial_link = os.readlink(path) if stat.S_ISLNK(initial.st_mode) else None
    try:
        fd = os.open(path, os.O_RDONLY | os.O_CLOEXEC)
    except OSError as error:
        ownership_collision(label, path, f"required runtime asset is unavailable: {error}")
    try:
        info = os.fstat(fd)
        if not stat.S_ISREG(info.st_mode):
            ownership_collision(label, path, "required runtime asset does not resolve to a regular file")
        content = read_fd(fd)
    finally:
        os.close(fd)
    try:
        current = os.lstat(path)
    except OSError as error:
        ownership_collision(label, path, f"required runtime asset changed: {error}")
    current_link = os.readlink(path) if stat.S_ISLNK(current.st_mode) else None
    if (
        stat.S_IFMT(current.st_mode) != stat.S_IFMT(initial.st_mode)
        or identity(current) != identity(initial)
        or current_link != initial_link
        or (not stat.S_ISLNK(initial.st_mode) and identity(info) != identity(initial))
    ):
        ownership_collision(label, path, "required runtime asset changed during validation")
    token = (
        stat.S_IFMT(initial.st_mode),
        initial.st_dev,
        initial.st_ino,
        initial_link,
        info.st_dev,
        info.st_ino,
        stat.S_IMODE(info.st_mode),
        hashlib.sha256(content).digest(),
    )
    return token, content


def validated_process_runtime_directory(path, label):
    if (
        not path
        or not os.path.isabs(path)
        or path.startswith("//")
        or os.path.normpath(path) != path
    ):
        raise ActivationError(
            "recognized direct End4 process has no canonical XDG_RUNTIME_DIR"
        )
    try:
        fd = os.open(path, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW | os.O_CLOEXEC)
    except OSError as error:
        ownership_collision(label, path, f"cannot pin XDG runtime directory: {error}")
    try:
        info = os.fstat(fd)
        visible = os.lstat(path)
        if (
            not stat.S_ISDIR(info.st_mode)
            or not stat.S_ISDIR(visible.st_mode)
            or identity(info) != identity(visible)
            or info.st_uid != os.getuid()
            or stat.S_IMODE(info.st_mode) != 0o700
        ):
            ownership_collision(label, path, "XDG runtime directory is not exact owner mode 0700")
        return path, (
            info.st_dev,
            info.st_ino,
            info.st_uid,
            stat.S_IMODE(info.st_mode),
        )
    finally:
        os.close(fd)


def direct_end4_process_records(label):
    executables = {
        b"qs-end4",
        b"qs",
        b"quickshell",
        b"quickshell-wrapped",
        b".quickshell-wrapped",
    }
    tokens = []
    try:
        process_names = os.listdir("/proc")
    except OSError:
        return tokens
    for process_name in process_names:
        if not process_name.isdecimal() or process_name.startswith("0"):
            continue
        proc = "/proc/" + process_name
        try:
            info = os.stat(proc, follow_symlinks=False)
            if info.st_uid != os.getuid():
                continue
            with open(proc + "/stat", "rb", buffering=0) as stream:
                stat_value = stream.read(65537)
            if len(stat_value) > 65536 or b") " not in stat_value:
                continue
            fields = stat_value.rsplit(b") ", 1)[1].split()
            if len(fields) <= 19 or not fields[19].isdigit() or fields[19] == b"0":
                continue
            start_time = fields[19].decode("ascii")
            with open(proc + "/cmdline", "rb", buffering=0) as stream:
                command_line = stream.read(65537)
            if len(command_line) > 65536 or not command_line.endswith(b"\0"):
                continue
            argv = command_line[:-1].split(b"\0")
        except (OSError, IndexError, ValueError):
            continue
        if (
            len(argv) != 5
            or os.path.basename(argv[0]) not in executables
            or argv[1:4] != [b"-n", b"-d", b"-c"]
            or argv[4] not in {b"ii", b"end4-pC"}
        ):
            continue
        try:
            with open(proc + "/environ", "rb", buffering=0) as stream:
                environment_value = stream.read(4194305)
        except OSError as error:
            raise ActivationError(
                f"cannot inspect exact direct End4 process {process_name} environment: {error}"
            ) from error
        if len(environment_value) > 4194304:
            raise ActivationError(
                f"exact direct End4 process {process_name} environment is too large"
            )
        environment_items = [item for item in environment_value.split(b"\0") if item]
        environment = set(environment_items)
        if (
            b"WAHRWELT_END4_PROFILE=end4" in environment
            or b"WAHRWELT_END4_PROFILE=end4-pc" in environment
        ):
            continue
        runtime_values = [
            item[len(b"XDG_RUNTIME_DIR="):]
            for item in environment_items
            if item.startswith(b"XDG_RUNTIME_DIR=")
        ]
        if len(runtime_values) != 1 or not runtime_values[0]:
            raise ActivationError(
                f"recognized direct End4 process {process_name} has no exact XDG_RUNTIME_DIR"
            )
        runtime_path = os.fsdecode(runtime_values[0])
        runtime_path, runtime_token = validated_process_runtime_directory(runtime_path, label)
        tokens.append(
            (
                int(process_name),
                f"{process_name}:{start_time}:{argv[4].decode('ascii')}",
                runtime_path,
                runtime_token,
            )
        )
    return sorted(tokens)


def legacy_direct_end4_process_tokens(label):
    ordered = direct_end4_process_records(label)
    if not ordered:
        return [], None, None
    runtime_path = ordered[0][2]
    runtime_token = ordered[0][3]
    if any(record[2:] != (runtime_path, runtime_token) for record in ordered[1:]):
        raise ActivationError("recognized direct End4 processes use different XDG runtime directories")
    return [record[1] for record in ordered], runtime_path, runtime_token


upgrade_token_pattern = re.compile(r"([1-9][0-9]*):([1-9][0-9]*):(ii|end4-pC)")


def durable_upgrade_state_has_token(runtime_path, process_token, label):
    runtime_fd = os.open(
        runtime_path,
        os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW | os.O_CLOEXEC,
    )
    state_fd = None
    marker_fd = None
    try:
        try:
            state_fd = os.open(
                "wahrwelt-end4-upgrade",
                os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW | os.O_CLOEXEC,
                dir_fd=runtime_fd,
            )
        except FileNotFoundError:
            return False
        state_info = os.fstat(state_fd)
        state_visible = os.stat(
            "wahrwelt-end4-upgrade",
            dir_fd=runtime_fd,
            follow_symlinks=False,
        )
        if (
            not stat.S_ISDIR(state_info.st_mode)
            or not stat.S_ISDIR(state_visible.st_mode)
            or identity(state_info) != identity(state_visible)
            or state_info.st_uid != os.getuid()
            or stat.S_IMODE(state_info.st_mode) != 0o700
        ):
            ownership_collision(label, runtime_path, "durable upgrade state directory is unsafe")
        marker_fd = os.open(
            ".wahrwelt-state-owner",
            os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK | os.O_CLOEXEC,
            dir_fd=state_fd,
        )
        marker_info = os.fstat(marker_fd)
        marker_visible = os.stat(
            ".wahrwelt-state-owner",
            dir_fd=state_fd,
            follow_symlinks=False,
        )
        marker_value = (
            "Wahrwelt private state v1\nkind=end4-upgrade-state\n"
            f"inode={state_info.st_dev}:{state_info.st_ino}\n"
        ).encode()
        if (
            not stat.S_ISREG(marker_info.st_mode)
            or identity(marker_info) != identity(marker_visible)
            or marker_info.st_uid != os.getuid()
            or marker_info.st_nlink != 1
            or stat.S_IMODE(marker_info.st_mode) != 0o600
            or read_fd(marker_fd) != marker_value
        ):
            ownership_collision(label, runtime_path, "durable upgrade state marker is unsafe")
        entries = os.listdir(state_fd)
        if len(entries) > 513:
            ownership_collision(label, runtime_path, "durable upgrade state has too many entries")
        active = set()
        for entry in entries:
            if entry == ".wahrwelt-state-owner":
                continue
            token = entry
            if upgrade_token_pattern.fullmatch(token) is None and entry.startswith(".consumed."):
                body = entry[len(".consumed."):]
                token, separator, nonce = body.rpartition(".")
                if not separator or re.fullmatch(r"[0-9a-f]{24}", nonce) is None:
                    token = ""
            if upgrade_token_pattern.fullmatch(token) is None:
                ownership_collision(label, runtime_path, "durable upgrade state has an unknown entry")
            fd = os.open(
                entry,
                os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK | os.O_CLOEXEC,
                dir_fd=state_fd,
            )
            try:
                info = os.fstat(fd)
                visible = os.stat(entry, dir_fd=state_fd, follow_symlinks=False)
                value = f"Wahrwelt End4 upgrade process v1\n{token}\n".encode()
                if (
                    not stat.S_ISREG(info.st_mode)
                    or identity(info) != identity(visible)
                    or info.st_uid != os.getuid()
                    or info.st_nlink != 1
                    or stat.S_IMODE(info.st_mode) != 0o600
                    or read_fd(fd) != value
                ):
                    ownership_collision(label, runtime_path, "durable upgrade token is unsafe")
            finally:
                os.close(fd)
            if entry == token:
                active.add(token)
        return process_token in active
    finally:
        if marker_fd is not None:
            os.close(marker_fd)
        if state_fd is not None:
            os.close(state_fd)
        os.close(runtime_fd)


def durable_upgrade_resume_runtime(label):
    matches = []
    for record in direct_end4_process_records(label):
        _, process_token, runtime_path, runtime_token = record
        if durable_upgrade_state_has_token(runtime_path, process_token, label):
            matches.append((runtime_path, runtime_token))
    if not matches:
        return None, None
    runtime_path, runtime_token = matches[0]
    if any(match != (runtime_path, runtime_token) for match in matches[1:]):
        raise ActivationError("durable End4 upgrade state exists in different XDG runtime directories")
    return runtime_path, runtime_token


def direct_end4_bundle(args, commit):
    expected_lengths = {32} if commit else {24}
    if len(args) not in expected_lengths:
        raise ActivationError(
            "usage: migrate-direct-end4-bundle RUNTIME_DIR PROVENANCE_ENTRYPOINT END4_VARIANT "
            "CANONICAL END4_MAIN END4_PC_MAIN "
            "OFFICIAL_PROFILE OFFICIAL_LOCK OFFICIAL_IDLE OFFICIAL_LAUNCHER "
            "OFFICIAL_KEYBINDS OFFICIAL_LEGACY_LAUNCHER OFFICIAL_LEGACY_KEYBINDS "
            "PC_PROFILE PC_LOCK PC_IDLE PC_LAUNCHER PC_KEYBINDS PC_LEGACY_LAUNCHER "
            "PC_LEGACY_KEYBINDS USER_ADAPTER_SOURCE END4_ADAPTER_SOURCE END4_LAUNCHER_SOURCE "
            "END4_CONTRACT_SOURCE [USER_ADAPTER_TARGET END4_ADAPTER_TARGET END4_LAUNCHER_TARGET "
            "END4_MAIN_TARGET END4_CONTRACT_TARGET OFFICIAL_QUICKSHELL_MAIN PC_QUICKSHELL_MAIN "
            "PERSIST_UPGRADE_HELPER]"
        )
    runtime_dir, provenance_entrypoint, end4_variant, canonical, end4_main, end4_pc_main = args[:6]
    official = args[6:13]
    pc = args[13:20]
    user_adapter_source, end4_adapter_source, end4_launcher_source, end4_contract_source = args[20:24]
    asset_targets = args[24:31] if commit else []
    persist_upgrade_helper = args[31] if commit else None
    label = "End4 runtime bundle"
    read_regular(canonical, label)
    end4_content = read_regular(end4_main, label)
    end4_pc_content = read_regular(end4_pc_main, label)
    official_contents = [read_regular(source, label) for source in official]
    pc_contents = [read_regular(source, label) for source in pc]
    user_adapter_content = read_regular(user_adapter_source, label)
    end4_adapter_content = read_regular(end4_adapter_source, label)
    end4_launcher_content = read_regular(end4_launcher_source, label)
    end4_contract_content = read_regular(end4_contract_source, label)
    provenance_token = direct_end4_provenance_token(
        provenance_entrypoint,
        end4_content,
        end4_pc_content,
        label,
    )

    try:
        runtime_info = os.lstat(runtime_dir)
    except FileNotFoundError:
        if provenance_token is None or not commit:
            return False, [], None, None
        runtime_info = None
    if runtime_info is not None and not stat.S_ISDIR(runtime_info.st_mode):
        ownership_collision(label, runtime_dir, "runtime path is not a directory")

    runtime_dir, runtime_fd, pinned = pin_directory(runtime_dir, label, create=runtime_info is None)
    try:
        validate_chain(pinned, label, runtime_dir)
        runtime_target = os.path.join(runtime_dir, "hyprland.lua")
        classification, initial = classify_leaf(runtime_fd, "hyprland.lua", label, runtime_target)
        if classification == "symlink":
            ownership_collision(label, runtime_target, "direct entrypoint is a symlink")
        state_direct = False
        if classification == "regular":
            opened, main_content = read_leaf(runtime_fd, "hyprland.lua", label, runtime_target)
            state_direct = main_content in {end4_content, end4_pc_content}
        else:
            opened, main_content = None, None
        direct_main_identity = identity(opened) if state_direct else None
        if not state_direct and provenance_token is None:
            if commit:
                resume_runtime, resume_token = durable_upgrade_resume_runtime(label)
                return False, [], resume_runtime, resume_token
            return False, [], None, None
        if state_direct and (opened.st_nlink != 1 or identity(opened) != identity(initial)):
            ownership_collision(label, runtime_target, "direct entrypoint has external hardlinks or changed")
        if commit:
            (
                upgrade_process_tokens,
                callback_runtime,
                callback_runtime_token,
            ) = legacy_direct_end4_process_tokens(label)
        else:
            upgrade_process_tokens, callback_runtime, callback_runtime_token = [], None, None

        # The direct entrypoint proves legacy ownership only. The remembered
        # variant is authoritative, and only its exact persisted pC value opts
        # into pC. Missing, malformed, linked, and non-regular state all fall
        # back to Official, matching the canonical runtime contract.
        selected = official
        try:
            variant_fd = os.open(
                end4_variant,
                os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK | os.O_CLOEXEC,
            )
        except OSError:
            variant_fd = None
        if variant_fd is not None:
            try:
                variant_info = os.fstat(variant_fd)
                if stat.S_ISREG(variant_info.st_mode) and read_fd(variant_fd) == b"end4-pc\n":
                    selected = pc
            finally:
                os.close(variant_fd)

        if commit:
            (
                user_adapter_target,
                end4_adapter_target,
                end4_launcher_target,
                end4_main_target,
                end4_contract_target,
                official_quickshell_main,
                pc_quickshell_main,
            ) = asset_targets
            asset_tokens = {}
            for target, expected in [
                (user_adapter_target, user_adapter_content),
                (end4_adapter_target, end4_adapter_content),
                (end4_launcher_target, end4_launcher_content),
                (end4_contract_target, end4_contract_content),
            ]:
                token, content = resolved_regular_asset(target, label)
                if content != expected:
                    ownership_collision(label, target, "required runtime asset content is not exact")
                asset_tokens[target] = token
            if asset_tokens[end4_contract_target][6] != 0o444:
                ownership_collision(label, end4_contract_target, "runtime contract mode is not 0444")
            asset_tokens[end4_main_target], _ = resolved_regular_asset(end4_main_target, label)
            quickshell_main = pc_quickshell_main if selected is pc else official_quickshell_main
            asset_tokens[quickshell_main], _ = resolved_regular_asset(quickshell_main, label)
            persist_upgrade_token, _ = resolved_regular_asset(persist_upgrade_helper, label)
            if persist_upgrade_token[6] & 0o111 == 0:
                ownership_collision(label, persist_upgrade_helper, "upgrade persistence helper is not executable")
        else:
            asset_tokens = {}
            persist_upgrade_token = None

        names = [
            "shell-profile.lua",
            "hyprlock.conf",
            "hypridle.conf",
            "shell-launcher.lua",
            "shell-keybinds.lua",
        ]
        desired_sources = selected[:5]
        desired_contents = official_contents[:5] if selected is official else pc_contents[:5]
        legacy_launchers = [official[5], pc[5]]
        legacy_keybinds = [official[6], pc[6]]
        legacy_launcher_contents = [official_contents[5], pc_contents[5]]
        legacy_keybinds_contents = [official_contents[6], pc_contents[6]]

        # Prove the complete ancillary bundle before changing any leaf. A
        # recognized direct End4 entrypoint remains loadable if this preflight
        # rejects an unknown or user-owned runtime file.
        for index, name in enumerate(names):
            display_path = os.path.join(runtime_dir, name)
            classification, info = classify_leaf(runtime_fd, name, label, display_path)
            if classification == "absent":
                continue
            if classification == "symlink":
                ownership_collision(label, display_path, "ancillary entrypoint is a symlink")
            opened, content = read_leaf(runtime_fd, name, label, display_path)
            allowed = [official_contents[index], pc_contents[index]]
            if name == "shell-launcher.lua":
                allowed.extend(legacy_launcher_contents)
            elif name == "shell-keybinds.lua":
                allowed.extend(legacy_keybinds_contents)
            if content not in allowed:
                ownership_collision(label, display_path, "unknown ancillary entrypoint")
            if opened.st_nlink != 1 or identity(opened) != identity(info):
                ownership_collision(label, display_path, "ancillary entrypoint has external hardlinks or changed")
        validate_chain(pinned, label, runtime_dir)
        if not commit:
            return True, [], None, None

        # The exact process tokens must survive any interruption after this
        # point, including a completed canonical main write with no protocol
        # response. Persist them before the first ancillary or main mutation.
        if upgrade_process_tokens:
            if validated_process_runtime_directory(callback_runtime, label) != (
                callback_runtime,
                callback_runtime_token,
            ):
                ownership_collision(label, callback_runtime, "process XDG runtime directory changed")
            current_persist_token, _ = resolved_regular_asset(persist_upgrade_helper, label)
            if current_persist_token != persist_upgrade_token:
                ownership_collision(label, persist_upgrade_helper, "upgrade persistence helper changed")
            try:
                persisted = subprocess.run(
                    [
                        persist_upgrade_helper,
                        "--persist-end4-upgrade-processes",
                        ",".join(upgrade_process_tokens),
                    ],
                    stdin=subprocess.DEVNULL,
                    stdout=subprocess.PIPE,
                    stderr=subprocess.PIPE,
                    check=False,
                    timeout=30,
                    env={**os.environ, "XDG_RUNTIME_DIR": callback_runtime},
                )
            except (OSError, subprocess.SubprocessError) as error:
                raise ActivationError(f"failed to persist direct End4 process provenance: {error}") from error
            if persisted.returncode != 0:
                raise ActivationError(
                    "failed to persist direct End4 process provenance before runtime migration"
                )
            if validated_process_runtime_directory(callback_runtime, label) != (
                callback_runtime,
                callback_runtime_token,
            ):
                ownership_collision(label, callback_runtime, "XDG runtime directory changed during persistence")
            current_persist_token, _ = resolved_regular_asset(persist_upgrade_helper, label)
            if current_persist_token != persist_upgrade_token:
                ownership_collision(label, persist_upgrade_helper, "upgrade persistence helper changed after use")
            if provenance_token is not None:
                current_provenance = direct_end4_provenance_token(
                    provenance_entrypoint,
                    end4_content,
                    end4_pc_content,
                    label,
                )
                if current_provenance != provenance_token:
                    ownership_collision(label, provenance_entrypoint, "direct provenance changed during persistence")
            if state_direct:
                current_info, current_content = read_leaf(
                    runtime_fd,
                    "hyprland.lua",
                    label,
                    runtime_target,
                )
                if (
                    current_content != main_content
                    or identity(current_info) != direct_main_identity
                    or current_info.st_nlink != 1
                ):
                    ownership_collision(label, runtime_target, "direct runtime changed during persistence")
            validate_chain(pinned, label, runtime_dir)

        ancillary_tokens = {}
        for index, name in enumerate(names):
            source = desired_sources[index]
            known_sources = [official[index], pc[index]]
            if name == "shell-launcher.lua":
                known_sources.extend(legacy_launchers)
            elif name == "shell-keybinds.lua":
                known_sources.extend(legacy_keybinds)
            display_path = os.path.join(runtime_dir, name)
            migrate_known_runtime_at(runtime_fd, name, source, known_sources, pinned, display_path)
            seed_exclusive_at(runtime_fd, name, source, label, display_path, pinned)
            ancillary_tokens[name] = managed_regular_token(runtime_fd, name, source, label, display_path)

        # Canonical main is the commit marker. It must remain the last write so
        # every interruption before this point leaves the direct End4 runtime
        # usable without depending on the new adapter bundle.
        test_barrier("BUNDLE_MAIN")
        validate_chain(pinned, label, runtime_dir)
        for target, expected_token in asset_tokens.items():
            current_token, _ = resolved_regular_asset(target, label)
            if current_token != expected_token:
                ownership_collision(label, target, "required runtime asset changed before main commit")
        if provenance_token is not None:
            current_provenance = direct_end4_provenance_token(
                provenance_entrypoint,
                end4_content,
                end4_pc_content,
                label,
            )
            if current_provenance != provenance_token:
                ownership_collision(label, provenance_entrypoint, "direct provenance changed before main commit")
        for index, name in enumerate(names):
            display_path = os.path.join(runtime_dir, name)
            current = managed_regular_token(
                runtime_fd,
                name,
                desired_sources[index],
                label,
                display_path,
            )
            if current != ancillary_tokens[name]:
                ownership_collision(label, display_path, "ancillary entrypoint changed before main commit")
        if state_direct:
            migrate_known_runtime_at(
                runtime_fd,
                "hyprland.lua",
                canonical,
                [end4_main, end4_pc_main],
                pinned,
                runtime_target,
            )
            managed_regular_token(runtime_fd, "hyprland.lua", canonical, label, runtime_target)
        elif classification == "absent":
            seed_exclusive_at(runtime_fd, "hyprland.lua", canonical, label, runtime_target, pinned)
            current_classification, _ = classify_leaf(runtime_fd, "hyprland.lua", label, runtime_target)
            if current_classification == "regular":
                current_info, current_content = read_leaf(
                    runtime_fd,
                    "hyprland.lua",
                    label,
                    runtime_target,
                )
                canonical_content = read_regular(canonical, label)
                if current_content == canonical_content and current_info.st_nlink == 1:
                    managed_regular_token(runtime_fd, "hyprland.lua", canonical, label, runtime_target)
        validate_chain(pinned, label, runtime_dir)
        test_barrier("BUNDLE_COMMIT")
        return True, upgrade_process_tokens, callback_runtime, callback_runtime_token
    finally:
        os.close(runtime_fd)


def activate_runtime_dir(args):
    if len(args) < 8:
        raise ActivationError("usage: activate-runtime-dir RUNTIME_DIR CANONICAL KNOWN... PROFILE LOCK IDLE LAUNCHER KEYBINDS")
    runtime_dir, canonical = args[:2]
    known_sources = args[2:-5]
    profile, lock, idle, launcher, keybinds = args[-5:]
    if not known_sources:
        raise ActivationError("activate-runtime-dir requires at least one known runtime source")
    runtime_dir, runtime_fd, pinned = pin_directory(runtime_dir, "Hypr runtime directory", create=True)
    try:
        test_barrier("ACTIVATION")
        validate_chain(pinned, "Hypr runtime directory", runtime_dir)
        runtime_target = os.path.join(runtime_dir, "hyprland.lua")
        migrate_known_runtime_at(runtime_fd, "hyprland.lua", canonical, known_sources, pinned, runtime_target)
        seed_exclusive_at(runtime_fd, "hyprland.lua", canonical, "Hyprland runtime", runtime_target, pinned)
        runtime_token = managed_regular_token(runtime_fd, "hyprland.lua", canonical, "Hyprland runtime", runtime_target)
        for name, source, label in [
            ("shell-profile.lua", profile, "shell profile runtime"),
            ("hyprlock.conf", lock, "hyprlock runtime"),
            ("hypridle.conf", idle, "hypridle runtime"),
            ("shell-launcher.lua", launcher, "shell launcher runtime"),
            ("shell-keybinds.lua", keybinds, "shell keybind runtime"),
        ]:
            seed_exclusive_at(runtime_fd, name, source, label, os.path.join(runtime_dir, name), pinned)
        test_barrier("RUNTIME_FINAL")
        if managed_regular_token(runtime_fd, "hyprland.lua", canonical, "Hyprland runtime", runtime_target) != runtime_token:
            ownership_collision("Hyprland runtime", runtime_target, "managed entrypoint identity changed before commit")
        validate_chain(pinned, "Hypr runtime directory", runtime_dir)
    finally:
        os.close(runtime_fd)


def run_with_runtime_hex(args):
    if len(args) < 3:
        raise ActivationError(
            "usage: run-with-runtime-hex RUNTIME_HEX RUNTIME_ID COMMAND [ARG...]"
        )
    runtime_hex, runtime_identity, command = args[:3]
    if (
        not runtime_hex
        or len(runtime_hex) % 2 != 0
        or re.fullmatch(r"[0-9a-f]+", runtime_hex) is None
    ):
        raise ActivationError("invalid XDG runtime hex transport")
    runtime_bytes = bytes.fromhex(runtime_hex)
    if not runtime_bytes or b"\0" in runtime_bytes:
        raise ActivationError("invalid XDG runtime bytes")
    identity_match = re.fullmatch(
        r"([0-9]+):([1-9][0-9]*):([0-9]+):700",
        runtime_identity,
    )
    if identity_match is None:
        raise ActivationError("invalid XDG runtime identity transport")
    expected_runtime_token = tuple(int(value) for value in identity_match.groups()[:3]) + (0o700,)
    runtime_path = os.fsdecode(runtime_bytes)
    runtime_path, runtime_token = validated_process_runtime_directory(
        runtime_path,
        "End4 runtime resume",
    )
    if runtime_token != expected_runtime_token:
        ownership_collision(
            "End4 runtime resume",
            runtime_path,
            "XDG runtime identity does not match migration proof",
        )
    if not os.path.isabs(command) or os.path.normpath(command) != command:
        raise ActivationError("runtime resume command must use one canonical absolute path")
    test_barrier("RUNTIME_EXEC")
    if validated_process_runtime_directory(runtime_path, "End4 runtime resume") != (
        runtime_path,
        runtime_token,
    ):
        ownership_collision("End4 runtime resume", runtime_path, "XDG runtime changed before exec")
    environment = dict(os.environ)
    environment["XDG_RUNTIME_DIR"] = runtime_path
    try:
        os.execve(command, args[2:], environment)
    except OSError as error:
        raise ActivationError(f"cannot exec runtime resume command: {error}") from error


def runtime_identity_transport(runtime_token):
    device, inode, user, mode = runtime_token
    if mode != 0o700:
        raise ActivationError("cannot transport a non-private XDG runtime identity")
    return f"{device}:{inode}:{user}:{mode:o}"


def main(argv):
    if not argv:
        raise ActivationError("missing Wahrwelt activation operation")
    operation, args = argv[0], argv[1:]
    if operation == "seed-exclusive":
        if len(args) != 3:
            raise ActivationError("usage: seed-exclusive SOURCE TARGET LABEL")
        simple_seed(*args)
    elif operation == "migrate-known-runtime":
        if len(args) < 3:
            raise ActivationError("usage: migrate-known-runtime TARGET CANONICAL KNOWN...")
        simple_migrate(args[0], args[1], args[2:])
    elif operation == "stage-known-runtime":
        if len(args) < 3:
            raise ActivationError("usage: stage-known-runtime TARGET CANONICAL KNOWN...")
        stage_known_runtime(args[0], args[1], args[2:])
    elif operation == "classify-top-level-runtime":
        if len(args) < 5:
            raise ActivationError(
                "usage: classify-top-level-runtime TARGET OLD_GENERATION EXPECTED_RELATIVE STABLE_SOURCE DIRECT_SOURCE..."
            )
        simple_classify_top_level_runtime(args[0], args[1], args[2], args[3], args[4:])
    elif operation == "classify-direct-runtime":
        if len(args) < 2:
            raise ActivationError("usage: classify-direct-runtime TARGET DIRECT_SOURCE...")
        simple_classify_direct_runtime(args[0], args[1:])
    elif operation == "token-exact-runtime":
        if len(args) != 2:
            raise ActivationError("usage: token-exact-runtime TARGET SOURCE")
        simple_exact_runtime_token(args[0], args[1])
    elif operation == "activate-user-dir":
        activate_user_dir(args)
    elif operation == "preflight-direct-end4-bundle":
        direct_end4_bundle(args, False)
    elif operation == "migrate-direct-end4-bundle":
        migrated, process_tokens, process_runtime, process_runtime_token = direct_end4_bundle(args, True)
        if migrated:
            protocol = "legacy-upgrade=" + ",".join(process_tokens)
            if process_tokens:
                protocol += ";runtime-hex=" + os.fsencode(process_runtime).hex()
                protocol += ";runtime-id=" + runtime_identity_transport(process_runtime_token)
            print(protocol)
        elif process_runtime is not None:
            print(
                "resume-upgrade-runtime-hex="
                + os.fsencode(process_runtime).hex()
                + ";runtime-id="
                + runtime_identity_transport(process_runtime_token)
            )
        else:
            print("current")
    elif operation == "run-with-runtime-hex":
        run_with_runtime_hex(args)
    elif operation == "activate-runtime-dir":
        activate_runtime_dir(args)
    else:
        raise ActivationError(f"unknown Wahrwelt activation operation: {operation}")


try:
    main(sys.argv[1:])
except ActivationError as error:
    print(error, file=sys.stderr)
    raise SystemExit(1) from None
PY
