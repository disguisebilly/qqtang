# 构建资源

这里保存可重建发行包所需的补全资源缓存、项目生成的 TP-free 客户端入口、Python 2.3 字节码处理依赖以及 Windows/Linux ONNX Runtime。

这些文件随源码仓库一同版本化，正常情况下不需要手工填充。Linux ONNX Runtime 仍由 `scripts/ensure-onnxruntime-linux.ps1` 校验；缺失时会从官方发布包恢复。

目录结构：

```text
build-assets/
  client-config/
  client-no-tp/
  resource-cache/
  pytools/
  onnxruntime/
    windows-amd64/
    linux-amd64/
    linux-arm64/
```

原始 QQ堂客户端不属于构建资源，必须由使用者自行放入 `client/original/`。

Python 库的用途、版本和更新方法见 [pytools/README.md](pytools/README.md)。

`client-config/avatarforge.ini`是可锻造名单的发行默认文件。构建脚本在复制原始客户端后应用它，生成`runtime/client-patched/config/avatarforge.ini`。地图变身开关属于服务端运行数据，独立放在[data/qqt_competitive_map_rules.json](../data/qqt_competitive_map_rules.json)，不由客户端资源生成。
