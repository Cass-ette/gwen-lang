"""CLI smoke tests for the Gwen reference implementation."""

import subprocess
import sys
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]


def run_cli(*args: str, input_text: str | None = None) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        [sys.executable, "-m", "gwen", *args],
        cwd=ROOT,
        text=True,
        input=input_text,
        capture_output=True,
    )


def test_cli_version_flag():
    result = run_cli("--version")

    assert result.returncode == 0
    assert result.stdout.strip() == "gwen 0.1.0"
    assert result.stderr == ""


def test_cli_help_flag():
    result = run_cli("--help")

    assert result.returncode == 0
    assert "usage: gwen" in result.stdout
    assert "run" in result.stdout
    assert "check" in result.stdout
    assert "repl" in result.stdout
    assert result.stderr == ""


def test_cli_run_subcommand_executes_program():
    result = run_cli("run", "examples/hello.gw")

    assert result.returncode == 0
    assert result.stdout.strip() == "Hello, Gwen!"
    assert result.stderr == ""


def test_cli_legacy_file_argument_still_executes_program():
    result = run_cli("examples/hello.gw")

    assert result.returncode == 0
    assert result.stdout.strip() == "Hello, Gwen!"
    assert result.stderr == ""


def test_cli_check_subcommand_reports_ok(tmp_path: Path):
    program = tmp_path / "main.gw"
    program.write_text('write("ok")\n', encoding="utf-8")

    result = run_cli("check", str(program))

    assert result.returncode == 0
    assert result.stdout.strip() == "OK"
    assert result.stderr == ""


def test_cli_repl_command_starts_and_exits_cleanly():
    result = run_cli("repl", input_text="exit\n")

    assert result.returncode == 0
    assert "Gwen REPL" in result.stdout
    assert "gwen>" in result.stdout
    assert result.stderr == ""
