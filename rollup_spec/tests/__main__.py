"""Entry point so the suite can be run with `python -m tests` from rollup_spec/.

Equivalent to `python -m pytest`; both pick up pyproject.toml's
`[tool.pytest.ini_options]` (pythonpath = src, testpaths = tests).
"""

import sys

import pytest

if __name__ == "__main__":
    raise SystemExit(pytest.main(sys.argv[1:]))
