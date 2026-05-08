{ pkgs }:

pkgs.python3.withPackages (
  ps:
  builtins.filter (pkg: pkg != null) [
    (ps.build or null)
    (ps.cffi or null)
    (ps.click or null)
    (ps."dbus-python" or null)
    (ps."kde-material-you-colors" or null)
    (ps.libsass or null)
    (ps.loguru or null)
    (ps."material-color-utilities" or null)
    (ps.materialyoucolor or null)
    (ps.numpy or null)
    (ps.pillow or null)
    (ps.psutil or null)
    (ps.pycairo or null)
    (ps.pygobject3 or null)
    (ps.pywayland or null)
    (ps.setproctitle or null)
    (ps."setuptools-scm" or null)
    (ps.tqdm or null)
    (ps.wheel or null)
    (ps."pyproject-hooks" or null)
    (ps.opencv4 or null)
  ]
)
