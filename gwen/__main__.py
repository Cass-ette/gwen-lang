#!/usr/bin/env python3
"""Gwen language CLI."""

import argparse
import sys
from pathlib import Path

from . import __version__
from .checker import SemanticChecker
from .interpreter import Interpreter
from .parser import parse


def run_file(path: str):
    source_path = Path(path)
    with source_path.open(encoding="utf-8") as f:
        source = f.read()
    program = parse(source)
    interp = Interpreter()
    interp.run(program, source_path=str(source_path))


def check_file(path: str):
    source_path = Path(path)
    with source_path.open(encoding="utf-8") as f:
        source = f.read()
    program = parse(source)
    checker = SemanticChecker(module_search_paths=[str(source_path.resolve().parent)])
    checker.check_program(program, source_path=str(source_path))
    print("OK")


def repl():
    print("Gwen REPL (type 'exit' to quit)")
    interp = Interpreter()
    while True:
        try:
            line = input("gwen> ")
        except (EOFError, KeyboardInterrupt):
            print()
            break
        if line.strip() == "exit":
            break
        if not line.strip():
            continue
        try:
            program = parse(line)
            interp.run(program, check_semantics=False)
        except Exception as e:
            print(f"Error: {e}")


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        prog="gwen",
        description="Gwen language reference implementation",
    )
    parser.add_argument(
        "--version",
        action="version",
        version=f"%(prog)s {__version__}",
    )
    subparsers = parser.add_subparsers(dest="command")

    run_parser = subparsers.add_parser("run", help="run a Gwen program")
    run_parser.add_argument("path", help="path to a .gw source file")

    check_parser = subparsers.add_parser("check", help="check a Gwen program without running it")
    check_parser.add_argument("path", help="path to a .gw source file")

    subparsers.add_parser("repl", help="start the Gwen REPL")
    return parser


def _execute(args: argparse.Namespace):
    if args.command in (None, "repl"):
        repl()
        return
    if args.command == "run":
        run_file(args.path)
        return
    if args.command == "check":
        check_file(args.path)
        return
    raise ValueError(f"Unknown command: {args.command}")


def main(argv: list[str] | None = None):
    argv = list(sys.argv[1:] if argv is None else argv)
    if not argv:
        repl()
        return 0

    subcommands = {"run", "check", "repl"}
    if argv[0] not in subcommands and not argv[0].startswith("-"):
        try:
            run_file(argv[0])
            return 0
        except Exception as e:
            print(e, file=sys.stderr)
            return 1

    parser = build_parser()
    try:
        args = parser.parse_args(argv)
        _execute(args)
        return 0
    except SystemExit:
        raise
    except Exception as e:
        print(e, file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
