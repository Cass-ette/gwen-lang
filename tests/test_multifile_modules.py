"""Tests for loading Gwen modules from sibling files."""

import io
import os
import sys
from pathlib import Path

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from gwen.interpreter import Interpreter
from gwen.parser import parse


def run_path(path: Path) -> str:
    program = parse(path.read_text())
    interp = Interpreter()
    old_stdout = sys.stdout
    sys.stdout = io.StringIO()
    try:
        interp.run(program, source_path=str(path))
        return sys.stdout.getvalue().strip()
    finally:
        sys.stdout = old_stdout


def test_use_loads_module_from_same_directory(tmp_path: Path):
    (tmp_path / "math_utils.gw").write_text(
        """module math_utils
export func square(x: int) -> int
  return x * x
endfunc
endmodule
""",
        encoding="utf-8",
    )
    main_path = tmp_path / "main.gw"
    main_path.write_text(
        """use square from math_utils
write(square(9))
""",
        encoding="utf-8",
    )

    assert run_path(main_path) == "81"


def test_use_supports_transitive_module_loading(tmp_path: Path):
    (tmp_path / "helpers.gw").write_text(
        """module helpers
export func double(x: int) -> int
  return x * 2
endfunc
endmodule
""",
        encoding="utf-8",
    )
    (tmp_path / "math_utils.gw").write_text(
        """module math_utils
use double from helpers

export func quadruple(x: int) -> int
  return double(double(x))
endfunc
endmodule
""",
        encoding="utf-8",
    )
    main_path = tmp_path / "main.gw"
    main_path.write_text(
        """use quadruple from math_utils
write(quadruple(7))
""",
        encoding="utf-8",
    )

    assert run_path(main_path) == "28"


def test_use_rejects_non_exported_symbol_from_file_module(tmp_path: Path):
    (tmp_path / "math_utils.gw").write_text(
        """module math_utils
export func square(x: int) -> int
  return x * x
endfunc

func helper(x: int) -> int
  return x + 1
endfunc
endmodule
""",
        encoding="utf-8",
    )
    main_path = tmp_path / "main.gw"
    main_path.write_text(
        """use helper from math_utils
write(helper(9))
""",
        encoding="utf-8",
    )

    import pytest

    with pytest.raises(Exception, match="does not export 'helper'"):
        run_path(main_path)


def test_use_loads_exported_object_from_file_module(tmp_path: Path):
    (tmp_path / "bank.gw").write_text(
        """module bank
export object Account
  balance: int

  new(balance: int) -> Account
    return Account{balance := balance}
  endnew

  func value(self: Account) -> int
    return self.balance
  endfunc
endobject
endmodule
""",
        encoding="utf-8",
    )
    main_path = tmp_path / "main.gw"
    main_path.write_text(
        """use Account from bank
acc := Account.new(13)
write(acc.value())
""",
        encoding="utf-8",
    )

    assert run_path(main_path) == "13"


def test_use_rejects_private_object_from_file_module(tmp_path: Path):
    (tmp_path / "bank.gw").write_text(
        """module bank
object Account
  balance: int

  new(balance: int) -> Account
    return Account{balance := balance}
  endnew
endobject
endmodule
""",
        encoding="utf-8",
    )
    main_path = tmp_path / "main.gw"
    main_path.write_text(
        """use Account from bank
acc := Account.new(13)
""",
        encoding="utf-8",
    )

    import pytest

    with pytest.raises(Exception, match="does not export 'Account'"):
        run_path(main_path)


def test_use_loads_exported_type_alias_from_file_module(tmp_path: Path):
    (tmp_path / "ids.gw").write_text(
        """module ids
export type UserId = int8
endmodule
""",
        encoding="utf-8",
    )
    main_path = tmp_path / "main.gw"
    main_path.write_text(
        """use UserId from ids
id: UserId := 17
write(id)
""",
        encoding="utf-8",
    )

    assert run_path(main_path) == "17"


def test_private_type_alias_still_works_inside_exported_file_module_function(tmp_path: Path):
    (tmp_path / "ids.gw").write_text(
        """module ids
type TinyId = int8

export func echo(id: TinyId) -> TinyId
  return id
endfunc
endmodule
""",
        encoding="utf-8",
    )
    main_path = tmp_path / "main.gw"
    main_path.write_text(
        """use echo from ids
write(echo(99))
""",
        encoding="utf-8",
    )

    assert run_path(main_path) == "99"
