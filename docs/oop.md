# Gwen 受限对象系统（设计草案）

> 审计友好的 OOP：封装而不隐藏

## 设计目标

OOP 的**封装**和**代码局部性**对审计有利，但传统 OOP 的继承、多态、隐式 this 对审计是灾难。

Gwen 的 object 系统**只保留对审计有利的部分**，剔除有害的部分。

## 核心原则

| 规则 | 理由 |
|------|------|
| **无继承** | 消灭隐式行为链，一个名字只有一个实现 |
| **字段默认私有** | 强制通过方法修改，所有修改路径集中可审计 |
| **self 显式** | `func read(self: File)`，没有隐式 this 魔法 |
| **无重载/覆盖** | 方法名全局唯一，调用即所见 |
| **构造显式** | `File.new(path)` 返回普通记录，无特殊语法 |
| **可还原性** | `obj.method()` 等价展开为 `Object.method(obj)` |

## 语法

### 定义对象

```gwen
object Account
  // 字段默认私有，外部不可直接访问
  balance: float64
  owner: string
  transaction_count: int

  // 构造函数
  new(owner: string, initial: float64) -> Account
    return Account{
      balance := initial,
      owner := owner,
      transaction_count := 0
    }
  endnew

  // 方法（self 必须显式声明）
  func deposit(self: Account, amount: float64) -> result
    if amount < 0 then
      return err("negative deposit")
    endif
    self.balance := self.balance + amount
    self.transaction_count := self.transaction_count + 1
    return ok(self.balance)
  endfunc

  func withdraw(self: Account, amount: float64) -> result
    if amount > self.balance then
      return err("insufficient funds")
    endif
    self.balance := self.balance - amount
    return ok(amount)
  endfunc

  // 只读访问器
  func get_balance(self: Account) -> float64
    return self.balance
  endfunc

  func get_owner(self: Account) -> string
    return self.owner
  endfunc
endobject
```

### 使用对象

```gwen
// 构造
acc := Account.new("Alice", 1000.00)

// 方法调用（语法糖）
result := acc.deposit(500)

// 审计器等价展开
result := Account.deposit(acc, 500)

// 错误：字段私有，无法直接访问
write(acc.balance)     // 编译/运行时错误：private field
write(acc.get_balance())  // 正确：通过方法访问
```

## 对比：Gwen vs 传统 OOP

| 特性 | Java/Python | Gwen |
|------|-------------|------|
| 继承 | 支持 | ❌ 禁止 |
| 多态/虚函数 | 支持 | ❌ 禁止 |
| 字段访问控制 | `public/private/protected` | 默认私有，无 public |
| this/self | 隐式 | 显式参数 |
| 方法重载 | 支持 | ❌ 禁止 |
| 构造器 | 特殊语法 `new Object()` | 普通函数 `Object.new()` |

## 为什么这样设计？

### 审计场景对比

**传统 OOP（Java 风格）**：
```java
payment.pay(100);  // 实际是 CreditCard 还是 Crypto？
                   // 得翻继承链才能确定
```

**Gwen 风格**：
```gwen
// 情况 1：明确类型
cc := CreditCard.new(...)
cc.pay(100)        // 确定是 CreditCard.pay

// 情况 2：需要多态时，显式分派
match payment_type
  when "credit" => CreditCard.pay(account, 100)
  when "crypto" => Crypto.pay(account, 100)
endmatch
```

审计者**一眼看到所有分支**，没有隐藏的控制流。

## 实现状态

- [x] 语法解析
- [x] 字段私有性检查（仅对象方法绑定的 `self.field` 可访问）
- [x] 方法展开为函数调用（`obj.m(a)` ≡ `Object.m(obj, a)`）
- [x] 构造函数处理（`Object.new(...)`）

## 与模块系统的关系

对象和 module 可以共存（对象导出语法待对象系统实现后确定）：

```gwen
module banking
  object Account ... endobject
  object Transaction ... endobject

  // 注：当前 export 仅支持函数
  // 对象导出语法将在对象系统实现时设计
endmodule

use Account from banking

acc := Account.new("Bob", 100)
```

## 常见问答

**Q: 不支持继承，代码复用怎么办？**
> 用组合 + 显式转发。继承带来的隐式行为是审计噩梦，组合显式声明依赖关系。

**Q: 我需要接口/契约怎么办？**
> 用 `match` 分派。Gwen 相信显式分支比隐式虚函数表更审计友好。

**Q: self 显式太啰嗦了**
> 这是为了审计者能快速定位状态修改点。写代码时多打 5 个字符，读代码时少翻 3 个文件。

**Q: 和其他语言对比？**
> 类似 Rust 的 `struct + impl`，但更简单（无 trait 系统）。Go 的 struct + interface 也有相似精神，但 Gwen 连 interface 都省了，直接用 match 分派。
