#!/usr/bin/env sh
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
VENV="$ROOT/.gwen-demo-venv"
PROJECT="$ROOT/examples/ledger_app/main.gw"

if command -v python3 >/dev/null 2>&1; then
  PYTHON=python3
elif command -v python >/dev/null 2>&1; then
  PYTHON=python
else
  echo "Python 3 is required but was not found in PATH." >&2
  exit 1
fi

echo "== Gwen trial setup =="
echo "Repository: $ROOT"
echo "Virtual environment: $VENV"

"$PYTHON" -m venv "$VENV"

VENV_PYTHON="$VENV/bin/python"
if [ ! -x "$VENV_PYTHON" ]; then
  echo "Virtual environment python executable not found: $VENV_PYTHON" >&2
  exit 1
fi

echo "== Installing Gwen =="
"$VENV_PYTHON" -m pip install -e "$ROOT"

echo "== Checking ledger_app =="
"$VENV_PYTHON" -m gwen check "$PROJECT"

echo "== Running ledger_app =="
"$VENV_PYTHON" -m gwen "$PROJECT"

cat <<EOF

== Next ==
Read the project here:
  $ROOT/examples/ledger_app/main.gw
  $ROOT/examples/ledger_app/ledger_seed.gw
  $ROOT/examples/ledger_app/ledger_queries.gw
  $ROOT/examples/ledger_app/ledger_report.gw
  $ROOT/examples/ledger_app/ledger_store.gw
  $ROOT/examples/ledger_app/ledger_view.gw

Re-run later with:
  $VENV_PYTHON -m gwen $PROJECT
EOF
