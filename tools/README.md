# WSUP development tools

This source checkout uses this directory for the product implementation:

- [wsup.py](wsup.py) is the development CLI entry point.
- [wsup_core](wsup_core/) contains the standard-library Python services.

Run `python3 -B tools/wsup.py ...` from the repository root. Generated
workspaces install this runtime under `.wsup/`; their visible `tools/` is a
home for developer-tool projects, not a second WSUP implementation.

Keep tests in [tests](../tests/) and distributable starters in
[templates](../templates/). Do not copy an installed runtime into this directory.
