# QQ堂本地怀旧版开发说明

本仓库包含本地服务端、启动器、协议与游戏规则实现、静态客户端兼容补丁以及发行包构建脚本。原始 QQ堂客户端不进入仓库；普通玩家应直接从 GitHub Releases 下载 `QQTang-Local.zip`。

本项目源码采用 [Apache License 2.0](LICENSE)。原始客户端不在该许可范围内；随仓库提供的第三方组件继续遵循各自许可，详见 [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md)。

## 从源码构建

1. 克隆仓库。
2. 自行取得兼容的原始客户端，按 [client/original/README.md](client/original/README.md) 解压到 `client/original/`。
3. 运行 `build-release.cmd`。

仓库已经包含补全资源和各平台运行时。构建脚本从只读原始客户端生成：

```text
release/QQTang-Local/
release/QQTang-Local.zip
```

常规发行保持这个目录名、ZIP文件名和压缩包顶层目录。模型版本在`configs/models/README.md`及配套元数据中说明，不放入发行文件名。

完整工具链要求见 [CONTRIBUTING.md](CONTRIBUTING.md)。

## 源码结构

```text
cmd/                服务端、启动器和发行构建工具入口
internal/           协议、房间、战斗、持久化和启动器核心实现
configs/            默认运行规则与正式 AI 模型
data/               服务端规范数据
deploy/             Windows/Linux 发布启动脚本
scripts/            公开发行构建脚本
client/original/    用户自行放置的原始客户端，不进入 Git
build-assets/       版本化的补全资源与第三方运行时
```

开发参考：[架构说明](docs/server-architecture.md)、[协议说明](docs/protocol-overview.md)、[网络部署](docs/network-deployment.md)、[物品 ID](docs/item-id-catalog.md)、[AI 模型](configs/models/README.md)和[ONNX网络架构](docs/onnx-network-architecture.md)。

根目录`README.md`是整体运行与配置说明，`configs/models/README.md`是模型文件、接口和更换说明。发行构建将`deploy/windows/README.md`放到包根目录，并附带README引用的四份使用文档；更新用户说明时保持根目录与该发行源文件一致。

## 可编辑规则与模型

- [地图变身开关](data/qqt_competitive_map_rules.json)：列全地图ID和名称，启动时读取，编辑后重启服务端，不需要构建。语义及示例见[配置说明](docs/competitive-map-rules.md)。
- [锻造发行默认配置](build-assets/client-config/avatarforge.ini)：构建时覆盖原始客户端旧名单，发布后实际读取`runtime/client-patched/config/avatarforge.ini`。详细规则见[锻造说明](docs/avatar-forge-rules.md)。
- [水晶规则](data/qqt_forge_rules.json)：独立配置服务端概率和权重。
- [U2392部署模型](configs/models/README.md)：ONNX与JSON成对更新，采样设置和接口以元数据为准。

`build-assets/client-config`保存发行默认资源，`data`保存服务端运行时规则。修改已发布包里的规则只需重启对应进程，不必重新编译。

单独构建支持AI的服务端时，需要配置C编译器并使用`CGO_ENABLED=1`、`go build -tags onnxruntime ./cmd/qqt-server`。普通`go build`不包含ONNX Runtime后端；完整发布脚本会设置正确标签并附带平台运行库。

## 开发验证

```powershell
go test ./... -count=1
go vet ./...
go build ./cmd/...
```

请勿提交原始客户端、账号数据库、运行日志、抓包或私人配置。
