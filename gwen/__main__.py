#!/usr/bin/env python3
"""Gwen language CLI."""

import sys
from .parser import parse
from .interpreter import Interpreter


def run_file(path: str):
    with open(path) as f:
        source = f.read()
    program = parse(source)
    interp = Interpreter()
    interp.run(program)


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
            interp.run(program)
        except Exception as e:
            print(f"Error: {e}")


def main():
    if len(sys.argv) < 2:
        repl()
    else:
        run_file(sys.argv[1])


if __name__ == "__main__":
    main()
