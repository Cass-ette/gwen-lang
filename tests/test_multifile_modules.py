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


def test_use_rejects_module_file_with_extra_top_level_statements(tmp_path: Path):
    (tmp_path / "math_utils.gw").write_text(
        """module math_utils
export func square(x: int) -> int
  return x * x
endfunc
endmodule

write("leak")
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

    import pytest

    with pytest.raises(Exception, match="must contain exactly one top-level module definition for 'math_utils'"):
        run_path(main_path)


def test_runtime_use_also_rejects_module_file_with_extra_top_level_statements(tmp_path: Path):
    (tmp_path / "math_utils.gw").write_text(
        """module math_utils
export func square(x: int) -> int
  return x * x
endfunc
endmodule

write("leak")
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

    import pytest

    program = parse(main_path.read_text())
    interp = Interpreter()
    with pytest.raises(Exception, match="must contain exactly one top-level module definition for 'math_utils'"):
        interp.run(program, source_path=str(main_path), check_semantics=False)


def test_use_rejects_module_file_with_assignment_inside_module_body(tmp_path: Path):
    (tmp_path / "math_utils.gw").write_text(
        """module math_utils
x := 1

export func square() -> int
  return x * x
endfunc
endmodule
""",
        encoding="utf-8",
    )
    main_path = tmp_path / "main.gw"
    main_path.write_text(
        """use square from math_utils
write(square())
""",
        encoding="utf-8",
    )

    import pytest

    with pytest.raises(Exception, match="top level only allows use/func/object/type declarations, got assignment"):
        run_path(main_path)


def test_use_rejects_type_import_conflict_in_current_scope(tmp_path: Path):
    (tmp_path / "ids.gw").write_text(
        """module ids
export type UserId = int8
endmodule
""",
        encoding="utf-8",
    )
    main_path = tmp_path / "main.gw"
    main_path.write_text(
        """type UserId = int
use UserId from ids
id: UserId := 17
write(id)
""",
        encoding="utf-8",
    )

    import pytest

    with pytest.raises(Exception, match="Cannot import type 'UserId' from module 'ids': type name already defined in current scope"):
        run_path(main_path)


def test_use_rejects_cyclic_file_module_imports(tmp_path: Path):
    (tmp_path / "alpha.gw").write_text(
        """module alpha
use beta_value from beta

export func alpha_value() -> int
  return beta_value()
endfunc
endmodule
""",
        encoding="utf-8",
    )
    (tmp_path / "beta.gw").write_text(
        """module beta
use alpha_value from alpha

export func beta_value() -> int
  return alpha_value()
endfunc
endmodule
""",
        encoding="utf-8",
    )
    main_path = tmp_path / "main.gw"
    main_path.write_text(
        """use alpha_value from alpha
write(alpha_value())
""",
        encoding="utf-8",
    )

    import pytest

    with pytest.raises(Exception, match="Cyclic module import detected while loading 'alpha'"):
        run_path(main_path)


def test_runtime_use_rejects_cyclic_file_module_imports(tmp_path: Path):
    (tmp_path / "alpha.gw").write_text(
        """module alpha
use beta_value from beta

export func alpha_value() -> int
  return beta_value()
endfunc
endmodule
""",
        encoding="utf-8",
    )
    (tmp_path / "beta.gw").write_text(
        """module beta
use alpha_value from alpha

export func beta_value() -> int
  return alpha_value()
endfunc
endmodule
""",
        encoding="utf-8",
    )
    main_path = tmp_path / "main.gw"
    main_path.write_text(
        """use alpha_value from alpha
write(alpha_value())
""",
        encoding="utf-8",
    )

    import pytest

    program = parse(main_path.read_text())
    interp = Interpreter()
    with pytest.raises(Exception, match="Cyclic module import detected while loading 'alpha'"):
        interp.run(program, source_path=str(main_path), check_semantics=False)


def test_use_rejects_late_use_statement_inside_file_module(tmp_path: Path):
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
export func quadruple(x: int) -> int
  return double(double(x))
endfunc

use double from helpers
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

    import pytest

    with pytest.raises(Exception, match="must place use statements before func/object/type declarations"):
        run_path(main_path)
