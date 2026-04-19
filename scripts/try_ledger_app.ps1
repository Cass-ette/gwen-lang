$ErrorActionPreference = "Stop"

$Root = Split-Path -Parent $PSScriptRoot
$Venv = Join-Path $Root ".gwen-demo-venv"
$Project = Join-Path $Root "examples/ledger_app/main.gw"

if (Get-Command py -ErrorAction SilentlyContinue) {
  & py -3 -m venv $Venv
} elseif (Get-Command python -ErrorAction SilentlyContinue) {
  & python -m venv $Venv
} else {
  throw "Python 3 is required but was not found in PATH."
}

$VenvPython = Join-Path $Venv "Scripts/python.exe"
if (-not (Test-Path $VenvPython)) {
  throw "Virtual environment python executable not found: $VenvPython"
}

Write-Host "== Gwen trial setup =="
Write-Host "Repository: $Root"
Write-Host "Virtual environment: $Venv"

Write-Host "== Installing Gwen =="
& $VenvPython -m pip install -e $Root

Write-Host "== Checking ledger_app =="
& $VenvPython -m gwen check $Project

Write-Host "== Running ledger_app =="
& $VenvPython -m gwen $Project

Write-Host ""
Write-Host "== Next =="
Write-Host "Read the project here:"
Write-Host "  $Root/examples/ledger_app/main.gw"
Write-Host "  $Root/examples/ledger_app/ledger_seed.gw"
Write-Host "  $Root/examples/ledger_app/ledger_queries.gw"
Write-Host "  $Root/examples/ledger_app/ledger_report.gw"
Write-Host "  $Root/examples/ledger_app/ledger_store.gw"
Write-Host "  $Root/examples/ledger_app/ledger_view.gw"
Write-Host ""
Write-Host "Re-run later with:"
Write-Host "  $VenvPython -m gwen $Project"
