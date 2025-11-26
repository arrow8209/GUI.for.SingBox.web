项目来源与 https://github.com/GUI-for-Cores/GUI.for.SingBox，将原有项目改为web版本，方便服务器部署

<div align="center">
  <img src="build/appicon.png" alt="GUI.for.SingBox" width="200">
  <h1>GUI.for.SingBox</h1>
  <p>A Web UI for sing-box built with Vue 3 + Go HTTP server.</p>
</div>

## Preview

Take a look at the live version here: 👉 <a href="https://gui-for-cores.github.io/guide/gfs/" target="_blank">Live Demo</a>

<div align="center">
  <img src="docs/imgs/light.png">
</div>

## Document

[Community](https://gui-for-cores.github.io/guide/gfs/community)

## Build

1、Build Environment

- Node.js [link](https://nodejs.org/en)
- pnpm ：`npm i -g pnpm`
- Go [link](https://go.dev/)

2、Pull and Build

```bash
git clone https://github.com/GUI-for-Cores/GUI.for.SingBox.git

cd GUI.for.SingBox/frontend
pnpm install
pnpm build

cd ..
go build -o gui-singbox
./gui-singbox
```

By default the server listens on `:22345`. Set `PORT=8080` or `SERVER_ADDR=127.0.0.1:8080` before running to customize.

## Release Bundle

打包/发布时请至少拷贝以下文件与目录：

1. `gui-singbox`（或对应平台的可执行文件）：Go 服务器二进制，内嵌了前端静态资源。
2. `data/` 目录及其所有子内容：
   - `data/sing-box/`：sing-box 内核可执行文件与配置（`config.json`、`pid.txt` 等）。
   - `data/.cache/`：内核下载缓存、进程信息、插件/规则集缓存等运行期必需的临时文件。
   - `data/locales/`：可选的自定义语言包。
   - `data/*.yaml`（`user.yaml`、`profiles.yaml`、`subscribes.yaml`、`rulesets.yaml`、`plugins.yaml`、`scheduledtasks.yaml` 等）：用户设置、节点/订阅/规则/插件/计划任务等数据。
   - `data/subscribes/`、`data/rulesets/`、`data/plugins/`：订阅、规则集、插件的本地文件版本。
   - 其他以 `data/` 开头的子目录（如 `data/rolling-release*`），若项目运行中生成了内容也需一并复制。

确保上述内容整体复制，可以让部署环境完整继承核心程序、配置以及所有运行期资源。

## Stargazers over time

[![Stargazers over time](https://starchart.cc/GUI-for-Cores/GUI.for.SingBox.svg)](https://starchart.cc/GUI-for-Cores/GUI.for.SingBox)
