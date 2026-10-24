"""Validated defaults, shared settings, local overrides, environment and CLI."""
import os
from .registry import read_json, relative_target

DEFAULTS = {"timeout_seconds": 300, "output_limit_bytes": 65536, "report_dir": ".work/quality"}
ENV = {"WSUP_TIMEOUT_SECONDS": "timeout_seconds", "WSUP_OUTPUT_LIMIT_BYTES": "output_limit_bytes",
       "WSUP_REPORT_DIR": "report_dir"}

def load_config(root, overrides=None, environ=None):
    values = dict(DEFAULTS)
    sources = {key: "default" for key in values}
    def merge(settings, label):
        if not isinstance(settings, dict) or set(settings) - set(DEFAULTS):
            raise ValueError(f"{label}: unsupported configuration settings")
        values.update(settings)
        sources.update({key: label for key in settings})
    for filename in ("wsup.json", "wsup.local.json"):
        path = root / filename
        if path.exists():
            data = read_json(path)
            if type(data.get("version")) is not int or data["version"] != 1 or set(data) - {"version", "settings"}:
                raise ValueError(f"{filename}: configuration requires version 1")
            merge(data.get("settings", {}), filename)
    env = os.environ if environ is None else environ
    for name, key in ENV.items():
        if name in env:
            value = env[name]
            if key != "report_dir":
                try:
                    value = int(value)
                except ValueError:
                    raise ValueError(f"{name} must be an integer") from None
            merge({key: value}, name)
    merge({k: v for k, v in (overrides or {}).items() if v is not None}, "CLI")
    for key, maximum in (("timeout_seconds", 86400), ("output_limit_bytes", 10485760)):
        if type(values[key]) is not int or not 1 <= values[key] <= maximum:
            raise ValueError(f"{key} must be 1..{maximum}")
    report = relative_target(root, values["report_dir"])
    if not report.resolve().is_relative_to((root / ".work").resolve()) or (root / ".work").is_symlink():
        raise ValueError("report_dir must be inside the local .work directory")
    return values, sources
