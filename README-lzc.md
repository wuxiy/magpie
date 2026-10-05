# 懒猫微服 LPK 打包说明

本仓库以 [yetone/magpie](https://github.com/yetone/magpie) 为上游模板，在跟进上游代码的基础上，
把它的 Docker 版打包成懒猫微服的 lpk 应用。上游代码与 lpk 打包文件完全分离，
合并上游时不会产生冲突。

## 文件结构

| 文件 | 作用 |
| ---- | ---- |
| `package.yml` | lpk 包元数据（包名 `cloud.lazycat.app.magpie`、版本、多语言描述、权限声明） |
| `lzc-manifest.yml` | 运行结构：一个 `magpie` 服务 + 路由/鉴权/注入配置 |
| `lzc-build.yml` | 构建配置：从 `lzc/gen/Dockerfile` 构建 `embed:magpie` 镜像 |
| `lzc-deploy-params.yml` | 可选部署参数：对外访问地址（`MAGPIE_PUBLIC_URL`） |
| `lzc/build.sh` | buildscript：从上游 `Dockerfile` 生成 `lzc/gen/Dockerfile` |
| `lzc/icon.png` | 应用图标（取自上游 `build/darwin/.../magpie.png`） |

`lzc/build.sh` 对上游 Dockerfile 只做两处修改，其余与上游构建完全一致：

1. 默认命令 `serve` 改为 `web --addr 0.0.0.0:3430 --no-open`
   （网关 3425 + 浏览器界面 3430，与上游 README 的 Docker Compose 示例一致）。
   命令烧进镜像是因为 manifest 里 `setup_script` 与 `command` 不能同时设置。
2. 构建阶段注入 `ENV GOPROXY=https://goproxy.cn,direct`：盒子和国内开发机
   直连 `proxy.golang.org` / `storage.googleapis.com` 会失败。

脚本会校验替换结果，上游若改了 `CMD ["serve"]` 的写法，构建会立刻报错而不是
悄悄打出 `serve` 模式的镜像。

## 运行模型（lzc-manifest.yml 的设计）

```
                     ┌─ 懒猫 ingress（强制账号登录） ─────────────────┐
浏览器 ── https://magpie.<域名>/ ────────────► 3430 magpie web（管理界面）
agent  ── https://magpie.<域名>/v1/… ────────► 3425 网关（public_path 放行）
agent  ── https://magpie.<域名>/v1beta/… ────► 3425 网关（Gemini 接口）
```

- **管理界面**：走懒猫账号登录；`request` inject 会自动带上
  `magpie_web_3430=<MAGPIE_WEB_KEY>` cookie（magpie web 自己的鉴权），
  所以用户打开即用。万一注入失效，界面地址后加
  `?k=20539eb40e9f323543e3a1e33f4bbba2` 手动换一次 cookie 即可。
- **网关 API**：`/v1`、`/v1beta` 通过 `public_path` 放行（agent 无法走浏览器登录），
  由 magpie 自己的 gateway key 鉴权。`setup_script` 会在首次启动时把
  `/config/magpie/settings.json` 种子为 `{"lan": true}`（即"在局域网共享"），
  这使得到达网关的非本机请求**必须**携带有效的 `sk-magpie-key-…`，
  否则 401。用户在界面 Gateway → Gateway keys 里创建 key 发给 agent。
  注意 lzc-ingress 对 `upstreams.location` 是**精确匹配**：`/v1` 只命中
  字面 `/v1`，子路径要走带尾斜杠的 `/v1/`，所以清单里两种都配了。
- **持久化**：`/lzcapp/var/config` 挂载到容器的 `/config`（与上游 Docker 的
  volume 布局一致：HOME、XDG 目录、登录态、缓存都在里面）。
- agent 的 base URL：OpenAI 系 `https://<域名>/v1`，Anthropic/Gemini 系
  `https://<域名>`。`MAGPIE_PUBLIC_URL` 默认渲染为 `https://<应用域名>`，
  可在部署参数里覆盖。
- 镜像以 `VERSION=dev` 构建（上游 Dockerfile 默认值）：magpie 只会自更新
  release 版本，dev 版自更新是关闭的——lpk 版本升级统一由盒子的应用更新完成。
  副作用是界面"关于"里显示 dev，属预期。

## 同步上游

```sh
git fetch upstream
git merge upstream/main
```

lpk 相关文件都在独立路径/文件名下，正常情况下合并无冲突。
若上游改了 `Dockerfile`（尤其是 `CMD ["serve"]` 一行）或端口/环境变量，
需要同步调整 `lzc/build.sh` 和 `lzc-manifest.yml`。

合并后更新 `package.yml` 的 `version` 为上游最新 tag（去掉 `v` 前缀，
如 `v0.1.940` → `0.1.940`）；只改打包不改上游代码时，追加 `+lzc2`、
`+lzc3` 这样的构建号。

## 构建与安装

前置条件：

1. `npm install -g @lazycatcloud/lzc-cli`，`lzc-cli box` 已连接盒子；
2. 本地 Docker（用于构建镜像；国内网络需先给 Docker daemon 配好代理，
   并预拉取各基础镜像的 `linux/amd64` 变体，例如
   `docker pull --platform linux/amd64 golang:1.26-alpine`）。

当前 `lzc-build.yml` 里镜像是 `builder: remote`：镜像在盒子的懒猫开发者工具
里按盒子实际架构构建（cywu 盒子是 x86_64）。本地没装开发者工具时可以临时
切成 `builder: local`——用本地 Docker 的 buildx 按 `linux/amd64` 构建后全量
内嵌，完全离线于盒子；注意本地模式架构固定为 amd64，且 `lpk install` 仍需要
盒子上的开发者工具（推镜像走它的通道）。

```sh
mkdir -p release
# 终端若挂了代理需先清掉 ALL_PROXY 等，避免 lzc-cli 连不上盒子
env -u ALL_PROXY -u HTTP_PROXY -u HTTPS_PROXY \
  lzc-cli project build -o release/cloud.lazycat.app.magpie.lpk

# 安装到盒子
lzc-cli lpk install release/cloud.lazycat.app.magpie.lpk
```

本地验证镜像（可选）：

```sh
sh lzc/build.sh
docker build -f lzc/gen/Dockerfile -t magpie-lzc:test .   # 本机架构
docker buildx build --platform linux/amd64 -t magpie-amd64:test \
  -f lzc/gen/Dockerfile --load .                            # 与 lpk 相同的 amd64
```

## 安全清单

- 网关开放路径仅 `/v1`、`/v1beta`，且强制 gateway key（`lan: true` 种子）。
- 管理界面在懒猫强制登录之后，web key 注入只是第二层；key 写在
  `lzc-manifest.yml`（environment 与 injects 两处，需保持一致），更换时
  两处一起改。
- 不要把 `public_path` 扩到其他路径；magpie 的设置界面和 key 都在登录保护下。
