[English Version](./README.en.md)

# Gwen 文档入口

这组文档不是一篇总宣言。它分成三类读者：

- 想写 Gwen 的人
- 想理解 Gwen 设计取舍的人
- 想继续实现 Gwen 的人

如果你只是想开始写 Gwen，先看下面四篇：

- [syntax.md](./syntax.md)
- [types.md](./types.md)
- [scope.md](./scope.md)
- [stdlib.md](./stdlib.md)

如果你想理解这门语言为什么这样设计，再看：

- [philosophy.md](./philosophy.md)
- [modules.md](./modules.md)
- [concurrency.md](./concurrency.md)
- [oop.md](./oop.md)

如果你关心当前实现和编译路线，再看：

- [compiler.md](./compiler.md)
- [tracking.md](./tracking.md)

## 文档列表

| 文档 | 作用 |
|------|------|
| [syntax.md](./syntax.md) | 基础语法与控制流 |
| [types.md](./types.md) | 类型系统、显式精度数值、`money[...]` |
| [scope.md](./scope.md) | 作用域、`global`、嵌套函数 |
| [modules.md](./modules.md) | 模块定义、导入、可见性 |
| [stdlib.md](./stdlib.md) | 当前标准库表面与边界 |
| [concurrency.md](./concurrency.md) | `parallel`、共享状态、当前并发语义 |
| [memory.md](./memory.md) | 当前内存模型与 arena 方向 |
| [oop.md](./oop.md) | 受限对象系统 |
| [appendix.md](./appendix.md) | 关键字、运算符、附录材料 |
| [philosophy.md](./philosophy.md) | Gwen 接受新设计时用的判断尺 |
| [compiler.md](./compiler.md) | 前端、HIR、MIR、C emitter、编译路线 |
| [tracking.md](./tracking.md) | 文档与实现的对齐记录 |

## 读的时候注意

- `tracking.md` 是实现记录，不是语言手册。
- `compiler.md` 是实现文档，不是给第一次写 Gwen 的人看的教程。
- 某个能力如果只出现在 `tracking.md`，但没进入 `syntax/types/stdlib` 主文档，默认不要把它当成已经稳定的公开表面。
