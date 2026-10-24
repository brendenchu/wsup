"""WSUP CLI; Python 3.11+ standard library only."""
from pathlib import Path
import sys

# Support direct execution and consumers loading this compatibility entry point.
_tools = str(Path(__file__).resolve().parent)
if _tools not in sys.path:
    sys.path.insert(0, _tools)
from wsup_core.cli import main
from wsup_core.registry import load

if __name__ == "__main__":
    sys.exit(main())
