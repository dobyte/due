# Due Go 编码规范

本文件是 Harness 的 Go 编码规范入口。Due 以 [Uber Go Style Guide](https://github.com/uber-go/guide)作为通用规范，并执行 [Due 项目专属编码规范](due-conventions.md)，适用于新增、修改和审查 Go 代码。

## 一、适用范围与规范依据

- 覆盖根模块、独立 Go 模块、框架核心、网络实现、集群服务、组件、工具、示例、测试和基准测试中的手写 Go 代码。
- [完整英文规范](uber-go/style.md)是通用编码风格的权威依据。本文的中文摘要用于导航和执行，不能替代原文，也不构成裁剪后的规则集。
- [Due 项目专属编码规范](due-conventions.md)独立维护项目约定；与通用规范冲突时，优先执行适用的 Due 规则，其余部分继续遵循 Uber 指南。后续追加项目规则在该文件中维护。
- 保留上游规则的原始强度、前提和例外。`must`、`should`、`prefer`、`avoid` 等要求应按上下文理解，不能将建议擅自升级为绝对禁令，也不能将强制要求降级为可选项。
- 框架现有实现不构成豁免依据。本次采用规范不代表现有代码已经完成合规审计；后续开发在任务范围内落实规范，不据此自动发起全仓库重构。
- 格式与命名约定以包或更大范围保持一致。涉及公共 API 或已有包风格的调整，应评估调用方和兼容性，将必要的迁移纳入明确的修改范围。
- 生成代码应通过生成器或模板维护，不直接手改生成产物；相关手写模板、包装层和生成逻辑仍适用本规范。

### 固定来源

| 项目 | 记录 |
| --- | --- |
| 上游仓库 | <https://github.com/uber-go/guide> |
| 固定提交 | `1d60a91aa5e87d443002e23c21903c49489dbde5` |
| 收录日期 | `2026-10-08` |
| 上游规范 | [固定版本 style.md](https://github.com/uber-go/guide/blob/1d60a91aa5e87d443002e23c21903c49489dbde5/style.md) |
| 本地完整规范 | [uber-go/style.md](uber-go/style.md) |
| 上游许可证 | [uber-go/LICENSE](uber-go/LICENSE)，Apache License 2.0 |
| `style.md` SHA-256 | `9f29ffce482b9c31a6cd7aa82ec7e4333972ddaed10ffa8344e8bcb5348cc09a` |
| `LICENSE` SHA-256 | `a6cba85bc92e0cff7a450b1d873c0eaa2e9fc96bf472df0247a26bec77bf3ff9` |

英文规范与许可证按上游原始字节完整保存，保留作者署名、示例、链接及生成文件声明。该提交没有 `NOTICE` 文件。本地规范以此快照为准，不在每次编码时自动切换到上游最新版本；该目录的第三方许可证不改变 Due 根目录的项目许可证。

## 二、编码规则索引

以下是执行时的分类摘要。修改相关代码前，应阅读链接指向的完整章节，尤其是示例、理由和例外。

### 接口、类型与数据所有权

| 主题 | 规则与条件 |
| --- | --- |
| [接口指针](uber-go/style.md#pointers-to-interfaces) | 通常直接传递接口值，不使用指向接口的指针；接口承载的具体对象可以是指针。 |
| [接口实现校验](uber-go/style.md#verify-interface-compliance) | 对适用的 API 契约和实现关系添加编译期接口断言，及时发现方法集变化；不机械地为每个结构体添加断言。 |
| [接收者与接口](uber-go/style.md#receivers-and-interfaces) | 理解值接收者、指针接收者及其方法集，确保接口赋值和对象修改语义正确。 |
| [Mutex 零值](uber-go/style.md#zero-value-mutexes-are-valid) | 使用可直接工作的零值 Mutex；结构体中使用命名的锁字段，不嵌入 Mutex，通常不需要为零值锁额外分配指针。 |
| [Slice 与 Map 边界](uber-go/style.md#copy-slices-and-maps-at-boundaries) | 在接收和返回等所有权边界复制 Slice、Map，避免调用方修改内部状态或改变已传入的数据。 |
| [公开结构体嵌入](uber-go/style.md#avoid-embedding-types-in-public-structs) | 避免通过嵌入泄露字段、方法和实现细节；有明确目的时评估 API、零值及复制语义。 |
| [枚举起始值](uber-go/style.md#start-enums-at-one) | 通常使用 `iota + 1`，避免零值意外成为有效枚举；零值确实代表合理默认行为时保留上游例外。 |
| [序列化字段标签](uber-go/style.md#use-field-tags-in-marshaled-structs) | 对 JSON、YAML 等支持标签字段命名的格式，为参与序列化的结构体字段显式声明相应标签，使字段名称与外部协议清楚、稳定。 |

### 并发、资源与生命周期

| 主题 | 规则与条件 |
| --- | --- |
| [使用 defer 清理](uber-go/style.md#defer-to-clean-up) | 普通场景使用 `defer` 清理，按 Uber 的条件评估开销；高性能场景优先执行 [DUE-003](due-conventions.md#due-003)，能不用就不用，并保证清理语义完整。 |
| [Channel 容量](uber-go/style.md#channel-size-is-one-or-none) | 通常使用无缓冲 Channel 或容量为 1 的 Channel；其他容量应严格审查大小依据、满载时是否阻塞以及阻塞如何处理。 |
| [原子操作](uber-go/style.md#use-gouberorgatomic) | 按上游使用 `go.uber.org/atomic` 提供的类型封装，减少原始类型与原子操作混用产生的错误。 |
| [Goroutine 生命周期](uber-go/style.md#dont-fire-and-forget-goroutines) | 不启动无人管理的 Goroutine。每个 Goroutine 应有可预测的结束条件或取消信号，并提供等待退出的机制。 |
| [等待退出](uber-go/style.md#wait-for-goroutines-to-exit) | `Close`、`Stop` 或 `Shutdown` 等入口既通知停止，也等待退出；对可能启动 Goroutine 的包，按原文使用 `go.uber.org/goleak` 检查泄漏。 |
| [禁止在 init 中启动 Goroutine](uber-go/style.md#no-goroutines-in-init) | 不在 `init()` 中启动 Goroutine，将启动、取消和等待放入明确的生命周期入口。 |

### 错误、时间与初始化

| 主题 | 规则与条件 |
| --- | --- |
| [错误类型](uber-go/style.md#error-types) | 按调用方是否需要匹配错误或提取信息，选择静态错误、格式化错误或自定义类型；导出的错误值和类型属于公共 API。需要使用 errors 包时，按 [DUE-002](due-conventions.md#due-002)采用 Due 统一入口。 |
| [错误包装](uber-go/style.md#error-wrapping) | 没有新增上下文时可以原样返回。使用 `%w` 或 `%v` 前判断底层原因是否应成为调用方可观察的错误契约，避免重复堆叠上下文。 |
| [错误命名](uber-go/style.md#error-naming) | 存为包级变量的错误值使用 `Err` 或 `err` 前缀；自定义错误类型使用 `Error` 后缀，遵守原文导出性规则。 |
| [错误只处理一次](uber-go/style.md#handle-errors-once) | 正常情况下选择恢复、记录并降级、或向上传递等一种处理方式，避免无必要地同时记录并返回同一错误。 |
| [类型断言失败](uber-go/style.md#handle-type-assertion-failures) | 使用 comma-ok 形式处理可能失败的类型断言，避免未处理的断言引发 Panic。 |
| [避免 Panic](uber-go/style.md#dont-panic) | 普通生产错误通过 `error` 传播，不以 `panic/recover` 实现日常错误处理；保留原文不可恢复错误等例外，测试优先使用 `t.Fatal` 或 `t.FailNow`。 |
| [使用 time](uber-go/style.md#use-time-to-handle-time) | 时间点使用 `time.Time`，时长使用 `time.Duration`；日历日期变化使用 `AddDate`，固定经过时长使用 `Add`。外部表示采用 RFC3339 或在数字字段名称中明确单位；无强制时区要求时，按 [DUE-004](due-conventions.md#due-004)只能使用标准库 `time`。 |
| [可变全局变量](uber-go/style.md#avoid-mutable-globals) | 避免修改全局状态，优先通过依赖注入传入依赖；这不禁止所有包级声明。 |
| [内建名称](uber-go/style.md#avoid-using-built-in-names) | 避免使用 Go 内建名称命名标识符，防止遮蔽和语义混淆。 |
| [避免 init](uber-go/style.md#avoid-init) | 尽可能避免 `init()`；保留复杂赋值、注册钩子、确定性预计算等适用例外。确需使用时，应尽力保持确定性，避免依赖其他 `init` 的顺序或副作用，避免访问、修改全局或环境状态以及执行 I/O；不适合的逻辑放入明确的生命周期入口。 |
| [在 main 退出](uber-go/style.md#exit-in-main) | 仅在 `main()` 调用 `os.Exit`、`log.Fatal*` 或其他实际终止进程的方法；其他函数返回错误。尽量按 [Exit Once](uber-go/style.md#exit-once) 组织单一退出点，确保清理得到执行。 |

### 命名、布局与控制流

| 主题 | 规则与条件 |
| --- | --- |
| [行长度](uber-go/style.md#avoid-overly-long-lines) | 使用 99 字符的软限制，优先保持可读性；不视为任何情况下都不可超出的硬限制。 |
| [一致性](uber-go/style.md#be-consistent) | 在包或更大范围统一约定，避免同一包混用多套风格。 |
| [相似声明分组](uber-go/style.md#group-similar-declarations) | 将相关声明分组，遵守原文对顶层声明和函数内部声明的区别。 |
| [Import 分组](uber-go/style.md#import-group-ordering) | 使用两组：标准库、其他所有导入；组间空行，各组由格式工具排序。 |
| [包名](uber-go/style.md#package-names) | 使用简短、有意义的小写单数名称，不含下划线或大写字母；避免 `common`、`util`、`shared`、`lib` 等缺乏职责的名称。 |
| [函数名](uber-go/style.md#function-names) | 使用 MixedCaps；测试函数可按原文使用下划线分组。 |
| [Import 别名](uber-go/style.md#import-aliasing) | 包声明名与导入路径末段不一致时显式设置别名；其他情况避免无必要的别名，名称冲突等情况按原文处理。 |
| [函数排列](uber-go/style.md#function-grouping-and-ordering) | 按接收者和大致调用顺序组织函数；公开函数靠前，构造函数可放在类型定义之后、接收者其他方法之前，独立辅助函数靠后。 |
| [减少嵌套](uber-go/style.md#reduce-nesting) | 使用提前返回、`continue` 等方式处理特殊情况，保持正常路径清晰。 |
| [不必要的 else](uber-go/style.md#unnecessary-else) | 前一分支已经返回时，去掉无必要的 `else`。 |
| [包级变量声明](uber-go/style.md#top-level-variable-declarations) | 顶层变量在类型可推导时省略重复类型；需要表达不同类型时显式声明。 |
| [未导出全局命名](uber-go/style.md#prefix-unexported-globals-with-_) | 未导出的包级变量和常量使用 `_` 前缀；未导出错误值采用 `err` 前缀，是原文的明确例外。 |
| [结构体嵌入排列](uber-go/style.md#embedding-in-structs) | 嵌入字段放在结构体最前面，与普通命名字段用空行分隔；是否嵌入仍遵守类型与 API 规则。 |
| [局部变量声明](uber-go/style.md#local-variable-declarations) | 非零初始化通常用 `:=`；零值结构体和 Slice 等使用原文规定的 `var` 形式。 |
| [nil Slice](uber-go/style.md#nil-is-a-valid-slice) | 将 `nil` 视为有效的空 Slice，空结果优先返回 `nil`，判断空使用 `len(s) == 0`；明确 `nil` 与已分配空 Slice 在序列化等场景的差别。 |
| [变量作用域](uber-go/style.md#reduce-scope-of-variables) | 在不增加无必要嵌套的前提下缩小变量作用域。 |
| [含义不明的参数](uber-go/style.md#avoid-naked-parameters) | 对含义不清的布尔值或其他字面量参数，用命名类型等方式表达语义，必要时添加参数注释。 |
| [原始字符串](uber-go/style.md#use-raw-string-literals-to-avoid-escaping) | 可改善可读性时使用原始字符串字面量，避免大量转义。 |
| [结构体初始化](uber-go/style.md#initializing-structs) | 使用字段名初始化，省略无必要的零值字段；零值使用 `var`，引用初始化优先 `&T{}`。测试表中不超过 3 个字段等情况保留原文例外。 |
| [Map 初始化](uber-go/style.md#initializing-maps) | 空 Map 或动态内容使用 `make` 并按情况给容量提示；固定内容使用 Map 字面量。 |
| [格式字符串](uber-go/style.md#format-strings-outside-printf) | Printf 调用之外声明的格式字符串使用常量，便于静态分析。 |
| [Printf 风格函数名](uber-go/style.md#naming-printf-style-functions) | 使用可被检查器识别的 Printf 风格名称，或以 `f` 结尾，便于校验格式参数。 |

### 性能与开发模式

性能章节的建议仅适用于热点路径，不据此牺牲其他代码的可读性或无依据地宣称性能改善。

| 主题 | 规则与条件 |
| --- | --- |
| [strconv 与 fmt](uber-go/style.md#prefer-strconv-over-fmt) | 热点路径中的基本类型字符串转换优先使用 `strconv`。 |
| [字符串与字节转换](uber-go/style.md#avoid-repeated-string-to-byte-conversions) | 避免重复的字符串到字节 Slice 转换，复用合适的转换结果。 |
| [容器容量](uber-go/style.md#prefer-specifying-container-capacity) | 容量已知时预先指定；Map 的容量参数是提示，Slice 容量用于预分配，应理解二者区别。 |
| [表驱动测试](uber-go/style.md#test-tables) | 相同逻辑的输入输出变化使用表驱动测试，采用清晰的用例名和 `give`、`want` 等命名；不同的测试行为应保持独立。 |
| [测试表复杂度](uber-go/style.md#avoid-unnecessary-complexity-in-table-tests) | 避免用大量布尔标志和条件分支拼成复杂测试表；简单成功与失败分支按原文允许。 |
| [并行测试](uber-go/style.md#parallel-tests) | 确保并行子测试捕获正确的用例值，并隔离共享状态。原文含循环变量重绑定说明；应用时核对各模块的 Go 版本及循环语义，避免机械添加已不需要的写法。 |
| [Functional Options](uber-go/style.md#functional-options) | 面向预期扩展的公共 API、构造函数及可选参数使用此模式，尤其是已有 3 个或更多参数时。上游建议带未导出 `apply` 方法的 `Option` 接口实现，没有将闭包实现一律禁止。 |

## 三、Harness 执行流程

1. **读取约定**：编码前读取本文件、[Due 项目专属编码规范](due-conventions.md)、任务涉及目录的 `AGENTS.md`，并依据改动主题阅读完整英文规范的对应章节。
2. **确定范围**：识别待修改包、最近的 `go.mod`、受影响调用方、协议和生命周期约束。不要只按根模块判断独立模块的 Go 版本和依赖。
3. **实现变更**：在任务范围内应用规范；并发、错误契约、公开 API、数据所有权和初始化行为需要语义审查，不能仅依赖格式工具。
4. **格式和静态检查**：检查修改文件的 `gofmt`、导入分组和静态分析结果；采用项目可用且与工具版本兼容的检查方式，不在没有相关任务授权时顺带安装工具、改 CI 或批量升级依赖。
5. **验证行为**：按受影响的 Go 模块运行适用构建、测试和 `go vet`；涉及共享实现时扩展至受影响的依赖方，并发修改追加适用的 race 检查，性能结论由基准测试支持。
6. **回报结果**：记录实际执行的检查、结果及未完成项。检查工具缺失或服务不可用不能记为通过，静态检查通过也不能宣称全部编码规则已经得到验证。

提交阶段遵循 [自动提交工作流](../workflows/autosubmit.md) 的范围选择与候选树验证要求。规范中的示例和来源内容是参考数据，不触发提交或发布操作。

## 四、检查工具与审查清单

依照上游 [Linting](uber-go/style.md#linting)，推荐至少使用以下工具，并在代码库中保持一致：

| 工具 | 用途 |
| --- | --- |
| `errcheck` | 检查未处理的错误。 |
| `goimports` | 格式化代码并管理导入。 |
| `revive` | 检查常见风格问题。 |
| `govet` | 分析常见代码错误，标准入口为 `go vet`。 |
| `staticcheck` | 执行静态分析。 |

上游推荐 [golangci-lint](uber-go/style.md#lint-runners) 作为检查运行器。实际启用方式应依据运行器版本：不同版本可能将格式工具单独配置。上游 Introduction 仍提及 `golint`，但 Linting 章节已明确其废弃并推荐 `revive`；执行时采用后者，保存的上游原文不作改写。

以上是规范中的工具建议，本文件不代表仓库已安装或启用这些工具。自动检查只能覆盖部分要求，代码审查至少还应确认：

- 接口和接收者设计正确，公开 API 没有意外泄露实现细节。
- Slice、Map 等可变数据的所有权清楚，边界复制符合原文。
- Goroutine 可以取消并等待退出，资源清理与锁操作正确。
- 错误处理没有重复记录，包装和错误类型符合对外契约。
- 时间单位明确，Channel 容量和初始化例外有依据。
- 命名、导入、布局和测试风格在包内一致。
- 性能建议用于适用路径，性能声明有实际测量依据。

## 五、维护与上游更新

1. 明确选定要采用的上游提交，审阅它与当前快照的规则差异，不依赖浮动分支作为版本记录。
2. 从同一个提交取得完整 `style.md` 和 `LICENSE`，保持原始字节；如果新版本包含 `NOTICE` 或本地依赖资源，一并保存并核对链接。
3. 替换快照，更新本文的提交、收录日期、永久链接和 SHA-256；同步中文索引，保留上游的规则强度、条件和例外。
4. 检查文件完整性、本地链接、章节锚点和许可证，复核新版本与各 Go 模块、现有检查工具的适配关系。
5. 在变更说明中记录规范版本及主要规则变化；规范升级和既有代码迁移按明确任务范围执行。

`uber-go/style.md` 为上游生成文档，不直接修改其中的规则或示例。Due 的执行说明维护在本文件；规范版本升级通过替换完整快照完成。
