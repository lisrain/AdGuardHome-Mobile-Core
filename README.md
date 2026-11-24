# AdGuard Home Mobile Core

> 基于 [AdGuard Home](https://github.com/AdguardTeam/AdGuardHome) 源码的个人魔改分支，
> 面向 Android root 环境（Magisk / KernelSU / APatch）做功能裁剪。
> **本项目与 AdGuard 官方团队无关**，不是官方发行版，也不代表上游的立场或支持。
> 上游原始说明保留在 [README_upstream.md](README_upstream.md)。

AdGuard Home 本身是一个网络级的广告与跟踪器拦截 DNS 服务器。本项目保留它最核心的
DNS 代理、过滤规则、查询日志和统计能力，移除在手机上用不到的功能，换得更小的体积
和更低的内存占用。

## 与上游的关系

- 代码基于上游 `master` 分支，当前对应正式版 `v1.0.0`。
- 本项目是独立维护的魔改分支，**不向上游提交**，也不接受上游对本分支的
  质量与安全背书。
- 上游文档、社区和支持渠道不适用于本项目的裁剪行为。
- 版本号格式为 `v1.0.0-mobile.YYYYMMDD`，其中 `YYYYMMDD` 是编译日期。
- 前端使用 `v1.0.0` 起默认提供的新版 SolidJS 界面（`client_v2`），并按本分支
  的功能裁剪；旧版 React 界面（`client/`）已移除。

## 保留的功能

- 普通 DNS 服务（UDP / TCP），支持上游、缓存、重写、上下游探测
- 过滤规则：阻止列表、允许列表、自定义规则、DNS 重写
- 查询日志：保留命中规则、过滤器、处理耗时等信息，便于排查误杀
- 统计：查询数、拦截数、域名排行、上游排行、平均处理耗时
- 客户端识别（内部容器）：用于来源识别和自定义规则中的客户端匹配，
  管理面板中保留只读的客户端列表与查询
- 安装向导、登录页、普通 HTTP 管理面板（新版 SolidJS 界面）

## 已删除的功能

| 功能 | 说明 |
| --- | --- |
| DHCP 服务器 | 含 v4/v6、租约库、静态租约、RA 与相关 HTTP API |
| 安全搜索 | 含各搜索引擎规则文件、HTTP API、客户端字段 |
| 已阻止的服务 | 含服务清单、服务图标接口、客户端与统计字段 |
| 加密 | TLS / DoT / DoQ / DoH 服务端、HTTPS 面板、证书管理、Apple 描述文件 |
| 客户端设置 | 客户端添加/编辑/删除接口与页面，仅保留只读客户端列表 |
| 客户端排行 | 统计与仪表盘中的 Top Clients |
| 系统服务安装 | systemd / launchd / Windows service 相关代码 |
| 非 Linux 平台 | 仅构建 linux/amd64 与 linux/arm64 |
| 其它构建 | Docker、Snapcraft、未完成的 next API、旧版 React 界面（`client/`） |
| 语言 | 仅保留 English、简体中文、繁體中文 |

## 体积参考

| 构建 | 大小 |
| --- | --- |
| 上游原始构建 | 约 30 MB |
| 本分支 linux/amd64 | 约 21 MB |
| 本分支 linux/arm64 | 约 20 MB |

构建参数为 `-trimpath -ldflags "-s -w"` 且 `CGO_ENABLED=0`，实际体积随 Go 版本
和依赖更新会有波动。

## 构建

依赖：

- Go 1.27.4（`go.mod` 要求）
- Node.js 24 + npm（构建前端）

方式一：使用 Makefile（需要 POSIX `make`）

```sh
make
```

方式二：手动构建

```sh
cd client_v2
npm ci
npm run build-prod
cd ..

GOOS=linux GOARCH=arm64 CGO_ENABLED=0 \
  go build -trimpath -ldflags "-s -w" -o AdGuardHome .
```

前端产物会写入 `build/static/`，再由 Go 的 `go:embed` 打进二进制。交叉编译时
无需 gcc，`CGO_ENABLED=0` 即可。

## 在 Android root 环境中的一般用法

本项目的内核是一个普通的 Linux 可执行文件，可以按自己的需要打包进
Magisk / KernelSU / APatch 模块，或由任意启动脚本拉起：

- 二进制与 `AdGuardHome.yaml` 放在同一个工作目录即可，内核会以该目录作为
  工作目录读写配置、统计和查询日志。
- 管理面板地址由配置中的 `http.address` 决定，常见写法是
  `127.0.0.1:3000`。
- DNS 监听地址和端口由配置中的 `dns.bind_hosts` 与 `dns.port` 决定；
  如果不在 root 下直接监听 53 端口，可以监听一个高位端口，再由
  iptables 等方式重定向系统 DNS。
- 使用前请关闭系统的「私人 DNS」（Private DNS），避免绕过本地解析。
- 可与常见的代理工具共存，具体转发与分流规则取决于你的组合方式。

## 兼容性说明

- 旧版 `AdGuardHome.yaml` 中的 `dhcp`、`tls`、`safe_search`、
  `blocked_services` 等键会被忽略，并在内核写回配置时自动丢弃。
- 配置 `schema_version` 保持与上游一致，`configmigrate` 的历史迁移全部保留。
- 查询日志与统计的 `reason` / `result` 编号沿用上游，旧日志与旧统计文件仍可读取。
- 语言仅保留 English、简体中文、繁體中文；旧配置中其它语言会被重置为自动检测。
- 客户端只读：保留 `GET /control/clients` 与仅供界面查询的
  `POST /control/clients/search`，不提供添加、修改和删除。
- 更新检查暂未启用；`internal/updater` 保留了代码，可通过 `AGH_UPDATE_URL`
  指向自定义 `version.json`，后续再完善。

## 已知限制

- 不提供 TLS、DoH、DoT、DoQ 服务端，也不支持 HTTPS 管理面板。
- 不提供 DHCP 服务。
- 不提供客户端级别的设置与拦截服务配置，仅保留只读客户端识别。
- 只支持 linux/amd64 与 linux/arm64，其它平台不再维护。
- 前端为新版 SolidJS 界面（`client_v2`），已按本分支功能裁剪：设置中不再有
  「协议」和「DHCP」，过滤器菜单不再有「已阻止的服务」，统计中没有
  Top Clients 页面。

## 许可与致谢

- 本项目沿用上游的 **GPL-3.0** 许可证，详见 [LICENSE.txt](LICENSE.txt)。
- AdGuard Home 的版权归 AdGuard Software Ltd. 及其贡献者所有。
- 本项目仅供个人学习与自用，使用风险自负；请遵守当地法律法规。

## English Summary

This is an unofficial, opinionated fork of AdGuard Home for Android root
environments, based on upstream v1.0.0. It keeps DNS serving, filtering, query
logging, statistics and client identification, and removes DHCP, SafeSearch,
blocked services, TLS/DoH/DoT/DoQ, client settings, top clients, system service
integration and non-Linux platforms. The UI is the SolidJS `client_v2` frontend
introduced in v1.0.0, trimmed to the features above; only English, Simplified
Chinese and Traditional Chinese locales are bundled. Only linux/amd64 and
linux/arm64 are supported. The upstream README is kept as
[README_upstream.md](README_upstream.md). This fork is not affiliated with or
endorsed by the AdGuard team.
