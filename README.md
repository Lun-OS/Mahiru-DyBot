<p align="center">
  <img src="./webui-src/public/logo.jpg" alt="Logo" width="128" />
</p>

<h1 align="center">Mahiru DyBot</h1>

<p align="center">
  基于 Playwright 无头浏览器的抖音 Web IM 服务，提供 OneBot v11 协议接口与 WebUI 管理面板。
</p>

> ** ！！本项目仍在积极开发中，API 和功能可能随时变更，不保证稳定性！！**

---

## 简介

Mahiru DyBot 通过 Playwright 启动 Chromium 无头浏览器，登录抖音网页版，注入 JavaScript Hook 拦截并劫持抖音内置 IM SDK 的核心模块（`createMessage`、`sendMessage`、`fetchConversation` 等），在浏览器上下文中完成消息收发、会话管理、联系人查询等操作，再通过 Go 后端将这些能力以 OneBot v11 标准协议暴露出来。

## 实现原理

```
┌─────────────┐     HTTP/WS      ┌──────────────┐     Playwright     ┌──────────────────┐
│  Bot 框架    │ ◄──────────────► │  Go 后端      │ ◄───────────────► │  Chromium 无头    │
│  (HanChat等) │   OneBot v11     │  (mahiru)    │   Page.Evaluate   │  浏览器实例       │
└─────────────┘                   └──────────────┘                    │                  │
                                                                      │  ┌────────────┐ │
                                                                      │  │ 抖音 Web IM │ │
                                                                      │  │ SDK (hook) │ │
                                                                      │  └────────────┘ │
                                                                      └──────────────────┘
```

1. **浏览器启动**: Playwright 拉起 Chromium，注入自定义 UA 和视口配置
2. **扫码登录**: 导航至抖音登录页，轮询二维码状态，等待用户扫码确认
3. **SDK Hook**: 登录完成后，通过 `webpackChunkdouyin_web` 获取 `__webpack_require__`，定位 IM SDK 模块并劫持以下核心接口：
   - `createMessage` / `sendMessage` — 消息发送
   - `fetchConversation` / `conversationListManager` — 会话列表
   - `userCacheManager` — 用户信息缓存
   - `newMessagePushManager.registerNewMessagePush` — 实时消息推送
4. **消息轮询**: Go 后端每 2 秒通过 `page.Evaluate` 从 `window.__obNewMsgs[]` 排空新消息
5. **协议转换**: 将 SDK 数据格式转换为 OneBot v11 标准事件/响应格式，通过 HTTP、正向 WebSocket、反向 WebSocket 推送给下游

## 技术栈

- **Go** — 后端服务、OneBot v11 协议实现
- **Playwright (Go)** — 无头浏览器控制
- **gopsutil** — 系统信息采集（CPU / 内存 / 磁盘 / 网络）
- **React + TypeScript** — WebUI 管理面板
- **Vite** — 前端构建
- **TailwindCSS** — 样式
- **Zustand** — 状态管理

## 快速开始

```bash
# 构建
cd webui-src && npm install && npm run build && cd ..
go build -o mahiru-dybot .

# 运行
./mahiru-dybot
```

默认监听 `:17836`，浏览器访问 `http://localhost:17836/webui` 进入管理面板。

## 配置
```
1 首次使用需在 WebUI 中设置管理密码
2 创建容器账号并启动
3 扫描登录（如遇到扫描了但是登录不上，尝试通过webui远程控制进行二级验证）
```
## 许可证

Apache License 2.0
修改分发请带原作者Lun.和原仓库地址
