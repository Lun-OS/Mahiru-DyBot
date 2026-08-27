# Mahiru DyBot API 接口文档

> 本项目为抖音 Web IM 的 OneBot v11 实现，通过 Playwright 操控无头浏览器，调用抖音内置 IM SDK 实现消息收发和管理。

## 目录

- [认证说明](#认证说明)
- [标准 OneBot v11 接口](#标准-onebot-v11-接口)
- [扩展接口 - 信息查询](#扩展接口---信息查询)
- [扩展接口 - 用户操作](#扩展接口---用户操作)
- [扩展接口 - 消息操作](#扩展接口---消息操作)
- [扩展接口 - 会话管理](#扩展接口---会话管理)
- [扩展接口 - 群聊管理](#扩展接口---群聊管理)
- [扩展接口 - 陌生人管理](#扩展接口---陌生人管理)
- [扩展接口 - 数据管理](#扩展接口---数据管理)
- [扩展接口 - 搜索](#扩展接口---搜索)
- [WebUI 管理接口](#webui-管理接口)
  - [认证](#认证-1)
  - [系统信息](#系统信息)
  - [运行时设置](#运行时设置)
  - [账号管理](#账号管理)
  - [适配器管理](#适配器管理)
  - [调试控制](#调试控制)
- [SSE 事件推送](#sse-事件推送)

---

## 认证说明

### OneBot v11 接口认证

所有 `POST /{action}` 请求需要携带以下 Header：

| Header | 必需 | 说明 |
|--------|------|------|
| `Authorization` | 是 | `Bearer <access_token>`，token 来自 `config.json` 中的 `access_token` |
| `X-Self-ID` | 否 | 指定操作的账号 UID，省略时使用第一个在线账号 |
| `X-Client-Role` | 否 | 客户端角色，可选 `System` / `Normal` |

### WebUI 管理接口认证

所有 `/api/webui/` 下的接口（除 `me`、`login`、`setup`、`verify`、`events`）需在 Header 中携带：

| Header | 必需 | 说明 |
|--------|------|------|
| `Authorization` | 是 | `Bearer <webui_token>`，通过登录接口获取 |

---

## 标准 OneBot v11 接口

### `get_login_info` 获取登录号信息

**参数**: 无

**响应数据**:

| 字段名 | 数据类型 | 说明 |
|--------|----------|------|
| `user_id` | number | 抖音 UID |
| `nickname` | string | 昵称 |
| `sec_uid` | string | 抖音 sec_uid |
| `unique_id` | string | 抖音号 |
| `short_id` | string | 短号 |
| `avatar` | string | 头像 URL |
| `signature` | string | 个性签名 |
| `gender` | number | 性别 |
| `sdk_ready` | boolean | SDK 是否就绪 |
| `mod_id` | number | webpack 模块 ID |
| `connection_status` | string | 连接状态 |

---

### `get_stranger_info` 获取陌生人信息

> 标准接口名为 `get_stranger_info`，本实现对应为 `get_user_info`。

**参数**:

| 字段名 | 数据类型 | 默认值 | 说明 |
|--------|----------|--------|------|
| `user_id` | number/string | - | 用户 UID 或 sec_uid |

**响应数据**:

| 字段名 | 数据类型 | 说明 |
|--------|----------|------|
| `uid` | string | 用户 UID |
| `sec_uid` | string | 抖音 sec_uid |
| `nickname` | string | 昵称 |
| `unique_id` | string | 抖音号 |
| `short_id` | string | 短号 |
| `signature` | string | 个性签名 |
| `avatar_thumb` | string | 头像 URL |
| `avatar_small` | string | 小头像 URL |
| `follow_status` | number | 关注状态 |
| `follower_status` | number | 粉丝状态 |
| `verification_type` | number | 认证类型 |
| `custom_verify` | string | 自定义认证 |
| `enterprise_verify_reason` | string | 企业认证原因 |
| `store_region` | string | 注册地区 |

---

### `get_friend_list` 获取好友列表

**参数**: 无

**响应数据**: JSON 数组

| 字段名 | 数据类型 | 说明 |
|--------|----------|------|
| `user_id` | number | 用户 UID |
| `nickname` | string | 昵称 |
| `remark` | string | 备注名 |
| `dy_id` | string | 抖音号 |
| `short_id` | string | 短号 |
| `avatar` | string | 头像 URL |

---

### `get_group_list` 获取群列表

**参数**: 无

**响应数据**: JSON 数组

| 字段名 | 数据类型 | 说明 |
|--------|----------|------|
| `group_id` | number | 群 ID（会话 shortId） |
| `group_name` | string | 群名称 |
| `member_count` | number | 成员数 |

---

### `get_group_member_list` 获取群成员列表

**参数**:

| 字段名 | 数据类型 | 默认值 | 说明 |
|--------|----------|--------|------|
| `group_id` | number | - | 群 ID |

**响应数据**:

| 字段名 | 数据类型 | 说明 |
|--------|----------|------|
| `items` | array | 成员列表 |
| `total` | number | 成员总数 |

成员对象字段:

| 字段名 | 数据类型 | 说明 |
|--------|----------|------|
| `uid` | string | 用户 UID |
| `sec_uid` | string | 抖音 sec_uid |
| `nickname` | string | 昵称 |
| `role` | number | 角色 |

---

### `send_private_msg` 发送私聊消息

**参数**:

| 字段名 | 数据类型 | 默认值 | 说明 |
|--------|----------|--------|------|
| `user_id` | number/string | - | 对方 UID |
| `message` | string | - | 消息内容 |
| `auto_escape` | boolean | `false` | 是否纯文本 |

**响应数据**:

| 字段名 | 数据类型 | 说明 |
|--------|----------|------|
| `message_id` | string | 消息 server_id |
| `server_id` | string | 服务器消息 ID |
| `conversation_id` | string | 会话 ID |
| `flight_status` | number | 投递状态 |
| `msg_error_code` | number | 消息错误码 |
| `msg_error_msg` | string | 消息错误信息 |
| `server_check_code` | number | 服务端检查码 |
| `server_check_msg` | string | 服务端检查信息 |

---

### `send_group_msg` 发送群消息

**参数**:

| 字段名 | 数据类型 | 默认值 | 说明 |
|--------|----------|--------|------|
| `group_id` | number/string | - | 群 ID |
| `message` | string | - | 消息内容 |
| `auto_escape` | boolean | `false` | 是否纯文本 |

**响应数据**: 同 `send_private_msg`

---

### `send_msg` 发送消息

**参数**:

| 字段名 | 数据类型 | 默认值 | 说明 |
|--------|----------|--------|------|
| `message_type` | string | - | `private` 或 `group` |
| `user_id` | number/string | - | 对方 UID（私聊） |
| `group_id` | number/string | - | 群 ID（群聊） |
| `message` | string | - | 消息内容 |
| `auto_escape` | boolean | `false` | 是否纯文本 |

**响应数据**: 同 `send_private_msg`

---

### `delete_msg` 撤回消息

**参数**:

| 字段名 | 数据类型 | 默认值 | 说明 |
|--------|----------|--------|------|
| `message_id` | number | - | 消息 server_id |

**响应数据**: 无

---

### `get_status` 获取运行状态

**参数**: 无

**响应数据**:

| 字段名 | 数据类型 | 说明 |
|--------|----------|------|
| `online` | boolean | 是否在线 |
| `good` | boolean | 状态是否良好 |
| `accounts_online` | number | 在线账号数 |
| `accounts_total` | number | 总账号数 |
| `sdk_ready` | boolean | SDK 是否就绪 |
| `connection_status` | string | 连接状态 |
| `conversation_count` | number | 会话数量 |

---

### `get_version_info` 获取版本信息

**参数**: 无

**响应数据**:

| 字段名 | 数据类型 | 说明 |
|--------|----------|------|
| `app_name` | string | 应用标识，固定值 `Mahiru DyBot` |
| `version` | string | 应用版本 |
| `protocol_version` | string | OneBot 标准版本 |

---

## 扩展接口 - 信息查询

### `get_user_info` 获取用户详细信息

**参数**:

| 字段名 | 数据类型 | 默认值 | 说明 |
|--------|----------|--------|------|
| `user_id` | number/string | - | 用户 UID 或 sec_uid |

**响应数据**: 同 `get_stranger_info`

---

### `get_strangers` 获取陌生人会话列表

**参数**: 无

**响应数据**:

| 字段名 | 数据类型 | 说明 |
|--------|----------|------|
| `items` | array | 陌生人会话列表 |
| `total` | number | 总数 |

---

### `get_login_info_enhanced` 获取增强登录信息

> 已合并到 `get_login_info`，此接口保留兼容。

---

## 扩展接口 - 用户操作

### `follow_user` 关注用户

**参数**:

| 字段名 | 数据类型 | 默认值 | 说明 |
|--------|----------|--------|------|
| `user_id` | number/string | - | 用户 UID |

**响应数据**:

| 字段名 | 数据类型 | 说明 |
|--------|----------|------|
| `status` | string | `ok` |

---

### `unfollow_user` 取消关注用户

**参数**:

| 字段名 | 数据类型 | 默认值 | 说明 |
|--------|----------|--------|------|
| `user_id` | number/string | - | 用户 UID |

**响应数据**: 同 `follow_user`

---

### `block_user` 拉黑用户

**参数**:

| 字段名 | 数据类型 | 默认值 | 说明 |
|--------|----------|--------|------|
| `user_id` | number/string | - | 用户 UID |

**响应数据**: 同 `follow_user`

---

### `unblock_user` 取消拉黑用户

**参数**:

| 字段名 | 数据类型 | 默认值 | 说明 |
|--------|----------|--------|------|
| `user_id` | number/string | - | 用户 UID |

**响应数据**: 同 `follow_user`

---

### `set_remark` 设置好友备注

**参数**:

| 字段名 | 数据类型 | 默认值 | 说明 |
|--------|----------|--------|------|
| `user_id` | number/string | - | 用户 UID |
| `remark` | string | - | 备注名 |

**响应数据**: 同 `follow_user`

---

## 扩展接口 - 消息操作

### `recall_message` 撤回消息

**参数**:

| 字段名 | 数据类型 | 默认值 | 说明 |
|--------|----------|--------|------|
| `conversation_id` | string | - | 会话 ID |
| `server_id` | string | - | 消息 server_id |

**响应数据**: 同 `follow_user`

---

### `delete_message` 删除消息

**参数**:

| 字段名 | 数据类型 | 默认值 | 说明 |
|--------|----------|--------|------|
| `conversation_id` | string | - | 会话 ID |
| `server_id` | string | - | 消息 server_id |

**响应数据**: 同 `follow_user`

---

### `like_message` 点赞消息

**参数**:

| 字段名 | 数据类型 | 默认值 | 说明 |
|--------|----------|--------|------|
| `conversation_id` | string | - | 会话 ID |
| `server_id` | string | - | 消息 server_id |

**响应数据**: 同 `follow_user`

---

### `reply_message` 回复消息

**参数**:

| 字段名 | 数据类型 | 默认值 | 说明 |
|--------|----------|--------|------|
| `conversation_id` | string | - | 会话 ID |
| `server_id` | string | - | 被回复消息 server_id |
| `text` | string | - | 回复内容 |

**响应数据**:

| 字段名 | 数据类型 | 说明 |
|--------|----------|------|
| `message_id` | string | 消息 server_id |
| `client_id` | string | 客户端消息 ID |
| `server_check_code` | number | 服务端检查码 |
| `server_check_msg` | string | 服务端检查信息 |

---

## 扩展接口 - 会话管理

### `set_conversation_pin` 置顶/取消置顶会话

**参数**:

| 字段名 | 数据类型 | 默认值 | 说明 |
|--------|----------|--------|------|
| `conversation_id` | string | - | 会话 ID |
| `pinned` | boolean | `true` | 是否置顶 |

**响应数据**: 同 `follow_user`

---

### `set_conversation_mute` 免打扰/取消免打扰

**参数**:

| 字段名 | 数据类型 | 默认值 | 说明 |
|--------|----------|--------|------|
| `conversation_id` | string | - | 会话 ID |
| `muted` | boolean | `true` | 是否免打扰 |

**响应数据**: 同 `follow_user`

---

### `delete_conversation` 删除会话

**参数**:

| 字段名 | 数据类型 | 默认值 | 说明 |
|--------|----------|--------|------|
| `conversation_id` | string | - | 会话 ID |

**响应数据**: 同 `follow_user`

---

### `mark_read` 标记会话已读

**参数**:

| 字段名 | 数据类型 | 默认值 | 说明 |
|--------|----------|--------|------|
| `conversation_id` | string | - | 会话 ID |

**响应数据**: 同 `follow_user`

---

### `batch_clear_read` 批量清除会话已读

**参数**:

| 字段名 | 数据类型 | 默认值 | 说明 |
|--------|----------|--------|------|
| `conversation_ids` | array | - | 会话 ID 列表 |

**响应数据**: 同 `follow_user`

---

### `clear_conversation_messages` 清空会话消息

**参数**:

| 字段名 | 数据类型 | 默认值 | 说明 |
|--------|----------|--------|------|
| `conversation_id` | string | - | 会话 ID |

**响应数据**: 同 `follow_user`

---

## 扩展接口 - 群聊管理

### `create_group` 创建群聊

**参数**:

| 字段名 | 数据类型 | 默认值 | 说明 |
|--------|----------|--------|------|
| `user_ids` | array | - | 用户 UID 列表 |
| `name` | string | `""` | 群名称 |

**响应数据**:

| 字段名 | 数据类型 | 说明 |
|--------|----------|------|
| `group_id` | number | 群 ID |
| `conversation_id` | string | 会话 ID |

---

### `leave_group` 退出群聊

**参数**:

| 字段名 | 数据类型 | 默认值 | 说明 |
|--------|----------|--------|------|
| `group_id` | number | - | 群 ID |

**响应数据**: 同 `follow_user`

---

### `dissolve_group` 解散群聊

**参数**:

| 字段名 | 数据类型 | 默认值 | 说明 |
|--------|----------|--------|------|
| `group_id` | number | - | 群 ID |

**响应数据**: 同 `follow_user`

---

### `get_group_members` 获取群成员

**参数**:

| 字段名 | 数据类型 | 默认值 | 说明 |
|--------|----------|--------|------|
| `group_id` | number | - | 群 ID |

**响应数据**: 同 `get_group_member_list`

---

### `add_group_member` 添加群成员

**参数**:

| 字段名 | 数据类型 | 默认值 | 说明 |
|--------|----------|--------|------|
| `group_id` | number | - | 群 ID |
| `user_ids` | array | - | 用户 UID 列表 |

**响应数据**: 同 `follow_user`

---

### `remove_group_member` 移除群成员

**参数**:

| 字段名 | 数据类型 | 默认值 | 说明 |
|--------|----------|--------|------|
| `group_id` | number | - | 群 ID |
| `user_ids` | array | - | 用户 UID 列表 |

**响应数据**: 同 `follow_user`

---

### `apply_join_group` 申请加入群聊

**参数**:

| 字段名 | 数据类型 | 默认值 | 说明 |
|--------|----------|--------|------|
| `group_short_id` | string | - | 群 short_id |

**响应数据**: 同 `follow_user`

---

## 扩展接口 - 陌生人管理

### `delete_stranger_conversation` 删除陌生人会话

**参数**:

| 字段名 | 数据类型 | 默认值 | 说明 |
|--------|----------|--------|------|
| `conversation_id` | string | - | 陌生人会话 ID |

**响应数据**: 同 `follow_user`

---

### `delete_all_stranger_conversations` 删除所有陌生人会话

**参数**: 无

**响应数据**: 同 `follow_user`

---

### `mark_stranger_read` 标记陌生人会话已读

**参数**:

| 字段名 | 数据类型 | 默认值 | 说明 |
|--------|----------|--------|------|
| `conversation_id` | string | - | 陌生人会话 ID |

**响应数据**: 同 `follow_user`

---

### `mark_all_stranger_read` 标记所有陌生人会话已读

**参数**: 无

**响应数据**: 同 `follow_user`

---

### `get_stranger_preview` 获取陌生人预览

**参数**: 无

**响应数据**: 预览数据对象

---

### `get_stranger_messages` 获取陌生人消息

**参数**:

| 字段名 | 数据类型 | 默认值 | 说明 |
|--------|----------|--------|------|
| `conversation_id` | string | - | 陌生人会话 ID |

**响应数据**:

| 字段名 | 数据类型 | 说明 |
|--------|----------|------|
| `items` | array | 消息列表 |
| `total` | number | 总数 |

---

## 扩展接口 - 数据管理

### `get_conversation_list` 获取会话列表

> 标准接口，返回所有会话。

**参数**: 无

**响应数据**:

| 字段名 | 数据类型 | 说明 |
|--------|----------|------|
| `items` | array | 会话列表 |
| `total` | number | 总数 |

---

### `get_history_messages` 获取历史消息

**参数**:

| 字段名 | 数据类型 | 默认值 | 说明 |
|--------|----------|--------|------|
| `user_id` | number/string | - | 用户 UID |
| `count` | number | `20` | 获取数量 |

**响应数据**: OneBot v11 格式，`data.messages` 数组

| 字段名 | 数据类型 | 说明 |
|--------|----------|------|
| `data.messages` | array | 消息列表（标准 OneBot v11 格式） |

---

### `get_messages_by_user` 按用户获取消息

**参数**:

| 字段名 | 数据类型 | 默认值 | 说明 |
|--------|----------|--------|------|
| `conversation_id` | string | - | 会话 ID |

**响应数据**:

| 字段名 | 数据类型 | 说明 |
|--------|----------|------|
| `items` | array | 消息列表 |
| `total` | number | 总数 |

---

### `get_messages_by_conversation` 按会话获取消息

**参数**:

| 字段名 | 数据类型 | 默认值 | 说明 |
|--------|----------|--------|------|
| `conversation_id` | string | - | 会话 ID |

**响应数据**: 同 `get_messages_by_user`

---

### `get_message_by_server_id` 按 server_id 获取消息

**参数**:

| 字段名 | 数据类型 | 默认值 | 说明 |
|--------|----------|--------|------|
| `server_id` | string | - | 消息 server_id |

**响应数据**: 消息对象

---

### `get_message_read_receipt` 获取消息已读回执

**参数**:

| 字段名 | 数据类型 | 默认值 | 说明 |
|--------|----------|--------|------|
| `conversation_id` | string | - | 会话 ID |
| `server_id` | string | - | 消息 server_id |

**响应数据**:

| 字段名 | 数据类型 | 说明 |
|--------|----------|------|
| `finishedParticipants` | array | 已读成员列表 |
| `expectedParticipants` | array | 未读成员列表 |

---

### `get_participants_read_index` 获取群成员已读索引

**参数**:

| 字段名 | 数据类型 | 默认值 | 说明 |
|--------|----------|--------|------|
| `conversation_id` | string | - | 群会话 ID |

**响应数据**: 已读索引数据

---

### `fetch_conversation` 拉取会话状态

**参数**:

| 字段名 | 数据类型 | 默认值 | 说明 |
|--------|----------|--------|------|
| `conversation_id` | string | - | 会话 ID |

**响应数据**: 同 `follow_user`

---

### `update_read_receipt` 更新已读回执

**参数**:

| 字段名 | 数据类型 | 默认值 | 说明 |
|--------|----------|--------|------|
| `conversation_id` | string | - | 会话 ID |

**响应数据**: 同 `follow_user`

---

### `get_all_conversations` 获取所有会话（conversationListManager）

**参数**: 无

**响应数据**: 会话列表数据

---

### `get_all_group_conversations` 获取所有群聊（conversationListManager）

**参数**: 无

**响应数据**: 群聊会话列表数据

---

### `load_more_conversations` 加载更多会话

**参数**: 无

**响应数据**: 更多会话数据

---

### `load_messages` 加载消息列表

**参数**:

| 字段名 | 数据类型 | 默认值 | 说明 |
|--------|----------|--------|------|
| `conversation_id` | string | - | 会话 ID |

**响应数据**: 消息列表数据

---

### `get_conversation_participants_async` 异步获取群成员

**参数**:

| 字段名 | 数据类型 | 默认值 | 说明 |
|--------|----------|--------|------|
| `conversation_id` | string | - | 群会话 ID |

**响应数据**:

| 字段名 | 数据类型 | 说明 |
|--------|----------|------|
| `items` | array | 成员列表 |
| `total` | number | 总数 |

---

### `get_conversation_participants_by_page` 分页获取群成员

**参数**:

| 字段名 | 数据类型 | 默认值 | 说明 |
|--------|----------|--------|------|
| `conversation_id` | string | - | 群会话 ID |
| `page` | number | `1` | 页码 |
| `page_size` | number | `50` | 每页数量 |

**响应数据**: 分页成员数据

---

### `get_conversation_bots` 获取会话机器人

**参数**:

| 字段名 | 数据类型 | 默认值 | 说明 |
|--------|----------|--------|------|
| `conversation_id` | string | - | 会话 ID |

**响应数据**: 机器人列表

---

### `upsert_conversation_ext_info` 更新会话扩展设置

**参数**:

| 字段名 | 数据类型 | 默认值 | 说明 |
|--------|----------|--------|------|
| `conversation_id` | string | - | 会话 ID |
| `ext_info` | object | - | 扩展信息 |

**响应数据**: 同 `follow_user`

---

### `add_local_exts` 添加/更新本地扩展

**参数**:

| 字段名 | 数据类型 | 默认值 | 说明 |
|--------|----------|--------|------|
| `conversation_id` | string | - | 会话 ID |
| `exts` | object | - | 扩展键值对 |

**响应数据**: 同 `follow_user`

---

### `delete_local_exts` 删除本地扩展

**参数**:

| 字段名 | 数据类型 | 默认值 | 说明 |
|--------|----------|--------|------|
| `conversation_id` | string | - | 会话 ID |
| `keys` | array | - | 要删除的键列表 |

**响应数据**: 同 `follow_user`

---

### `request_relations` 请求好友关系数据

**参数**: 无

**响应数据**: 同 `follow_user`

---

### `gen_local_users` 生成本地用户列表

**参数**: 无

**响应数据**: 用户列表数据

---

### `report_message_delay` 上报消息延迟

**参数**:

| 字段名 | 数据类型 | 默认值 | 说明 |
|--------|----------|--------|------|
| `server_id` | string | - | 消息 server_id |
| `log_id` | string | `""` | 日志 ID |

**响应数据**: 同 `follow_user`

---

### `db_clear` 清空本地数据库

**参数**: 无

**响应数据**: 同 `follow_user`

---

## 扩展接口 - 搜索

### `search_conversations` 搜索会话

**参数**:

| 字段名 | 数据类型 | 默认值 | 说明 |
|--------|----------|--------|------|
| `keyword` | string | - | 搜索关键词 |

**响应数据**: 搜索结果数据

---

### `search_participants` 搜索群成员

**参数**:

| 字段名 | 数据类型 | 默认值 | 说明 |
|--------|----------|--------|------|
| `conversation_id` | string | - | 群会话 ID |
| `keyword` | string | - | 搜索关键词 |

**响应数据**: 搜索结果数据

---

## WebUI 管理接口

WebUI 接口通过 HTTP REST 提供，用于管理账号、适配器和系统设置。

### 认证

#### `POST /api/webui/auth/setup` 首次初始化密码

**参数**:

| 字段名 | 数据类型 | 说明 |
|--------|----------|------|
| `password` | string | 管理密码（至少 6 位） |

**响应**:

| 字段名 | 数据类型 | 说明 |
|--------|----------|------|
| `ok` | boolean | 是否成功 |
| `token` | string | 登录令牌 |
| `expires_in` | number | 过期时间（秒） |

---

#### `POST /api/webui/auth/login` 登录

**参数**:

| 字段名 | 数据类型 | 说明 |
|--------|----------|------|
| `password` | string | 管理密码 |

**响应**:

| 字段名 | 数据类型 | 说明 |
|--------|----------|------|
| `ok` | boolean | 是否成功 |
| `token` | string | 登录令牌 |
| `expires_in` | number | 过期时间（秒） |

---

#### `POST /api/webui/auth/verify` 验证令牌

**Header**: `Authorization: Bearer <token>`

**响应**:

| 字段名 | 数据类型 | 说明 |
|--------|----------|------|
| `ok` | boolean | 令牌是否有效 |

---

#### `GET /api/webui/me` 获取初始化状态

**响应**:

| 字段名 | 数据类型 | 说明 |
|--------|----------|------|
| `ok` | boolean | 是否成功 |
| `initialized` | boolean | 是否已初始化密码 |
| `authenticated` | boolean | 当前 token 是否有效 |

---

#### `POST /api/webui/auth/reset` 重置密码

**Header**: `Authorization: Bearer <token>`

**参数**:

| 字段名 | 数据类型 | 说明 |
|--------|----------|------|
| `password` | string | 新密码 |

**响应**:

| 字段名 | 数据类型 | 说明 |
|--------|----------|------|
| `ok` | boolean | 是否成功 |

---

### 系统信息

#### `GET /api/webui/system/info` 获取系统信息

**Header**: `Authorization: Bearer <token>`

**响应**:

```json
{
  "system": {
    "os": "windows",
    "platform": "Microsoft Windows 10 Pro",
    "arch": "amd64",
    "kernel_version": "10.0.19045",
    "hostname": "DESKTOP-XXX",
    "go_version": "go1.22.5",
    "uptime": 380029
  },
  "cpu": {
    "model": "Intel(R) Core(TM) i5-7200U CPU @ 2.50GHz",
    "cores": 4,
    "usage_percent": 15.0
  },
  "memory": {
    "total": 8469098496,
    "used": 6317535232,
    "free": 2151563264,
    "usage_percent": 74.0
  },
  "disk": {
    "total": 431054917632,
    "used": 3262095360,
    "free": 427792822272,
    "usage_percent": 0.76
  },
  "network": {
    "bytes_sent": 2151423095,
    "bytes_recv": 3116261790,
    "upload_speed": 0,
    "download_speed": 0
  },
  "process": {
    "pid": 14148,
    "name": "mahiru-dybot.exe",
    "memory_mb": 13.98,
    "memory_percent": 0.17,
    "cpu_percent": 0.64,
    "start_time": "2026-08-28 00:06:23",
    "uptime": "0h 0m"
  }
}
```

---

### 运行时设置

#### `GET /api/webui/settings` 获取设置

**Header**: `Authorization: Bearer <token>`

**响应**:

| 字段名 | 数据类型 | 说明 |
|--------|----------|------|
| `ok` | boolean | 是否成功 |
| `onebot_access_token` | string | OneBot access token |
| `screenshot_max_fps` | number | 截图最大帧率 |
| `jpeg_quality` | number | JPEG 压缩质量 |
| `reverse_ws` | array | 反向 WebSocket 配置列表 |
| `ws_connections` | array | 当前 WebSocket 连接状态 |
| `actions` | array | 已注册的 action 列表 |

---

#### `POST /api/webui/settings` 更新设置

**Header**: `Authorization: Bearer <token>`

**参数**（均可选，局部更新）:

| 字段名 | 数据类型 | 说明 |
|--------|----------|------|
| `onebot_access_token` | string | OneBot access token |
| `screenshot_max_fps` | number | 截图最大帧率 |
| `jpeg_quality` | number | JPEG 压缩质量 |
| `reverse_ws` | array | 反向 WebSocket 配置列表 |

**响应**: 同 `GET /api/webui/settings`

---

### 账号管理

#### `GET /api/webui/accounts` 获取账号列表

**响应**:

| 字段名 | 数据类型 | 说明 |
|--------|----------|------|
| `ok` | boolean | 是否成功 |
| `accounts` | array | 账号列表 |

账号对象字段:

| 字段名 | 数据类型 | 说明 |
|--------|----------|------|
| `id` | string | 账号 ID |
| `name` | string | 账号名称 |
| `status` | string | 状态：`stopped` / `starting` / `online` / `error` / `qr_pending` |
| `viewport_width` | number | 视口宽度 |
| `viewport_height` | number | 视口高度 |
| `custom_ua` | string | 自定义 User-Agent |
| `nickname` | string | 抖音昵称（在线时填充） |

---

#### `POST /api/webui/accounts` 创建账号

**参数**:

| 字段名 | 数据类型 | 说明 |
|--------|----------|------|
| `name` | string | 账号名称（可选） |

**响应**:

| 字段名 | 数据类型 | 说明 |
|--------|----------|------|
| `ok` | boolean | 是否成功 |
| `account` | object | 新建账号信息 |

---

#### `GET /api/webui/accounts/{id}/info` 获取账号详情

**响应**: 同账号对象字段

---

#### `DELETE /api/webui/accounts/{id}` 删除账号

**响应**:

| 字段名 | 数据类型 | 说明 |
|--------|----------|------|
| `ok` | boolean | 是否成功 |

---

#### `POST /api/webui/accounts/{id}/rename` 重命名账号

**参数**:

| 字段名 | 数据类型 | 说明 |
|--------|----------|------|
| `name` | string | 新名称 |

**响应**:

| 字段名 | 数据类型 | 说明 |
|--------|----------|------|
| `ok` | boolean | 是否成功 |

---

#### `POST /api/webui/accounts/{id}/start` 启动账号

启动浏览器实例并初始化 SDK，同步执行（约需 10-30s）。

**响应**:

| 字段名 | 数据类型 | 说明 |
|--------|----------|------|
| `ok` | boolean | 是否成功 |
| `account` | object | 账号信息 |

---

#### `POST /api/webui/accounts/{id}/stop` 停止账号

**响应**:

| 字段名 | 数据类型 | 说明 |
|--------|----------|------|
| `ok` | boolean | 是否成功 |
| `account` | object | 账号信息 |

---

#### `POST /api/webui/accounts/{id}/settings` 更新账号设置

**参数**（均可选）:

| 字段名 | 数据类型 | 说明 |
|--------|----------|------|
| `name` | string | 账号名称 |
| `viewport_width` | number | 视口宽度 |
| `viewport_height` | number | 视口高度 |
| `custom_ua` | string | 自定义 User-Agent |

**响应**:

| 字段名 | 数据类型 | 说明 |
|--------|----------|------|
| `ok` | boolean | 是否成功 |
| `account` | object | 更新后的账号信息 |

---

#### `GET /api/webui/accounts/{id}/qrcode` 获取登录二维码

**响应**:

| 字段名 | 数据类型 | 说明 |
|--------|----------|------|
| `ok` | boolean | 是否成功 |
| `image_base64` | string | 二维码图片 base64 |
| `token` | string | 二维码 token |
| `state` | string | 当前状态 |

---

#### `GET /api/webui/accounts/{id}/wait-login?timeout=180` 长轮询等待登录

驱动扫码确认 → SDK 初始化 → 置 online 全流程。

**参数**:

| 参数名 | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `timeout` | number | `180` | 超时时间（秒），最大 600 |

**响应**:

| 字段名 | 数据类型 | 说明 |
|--------|----------|------|
| `ok` | boolean | 是否成功 |
| `logged_in` | boolean | 是否已登录 |
| `account` | object | 账号信息（登录成功时） |
| `timeout` | boolean | 是否超时 |
| `expired` | boolean | 二维码是否过期 |
| `error` | string | 错误信息 |

---

### 适配器管理

#### `GET /api/webui/accounts/{id}/adapters` 获取适配器列表

**响应**:

| 字段名 | 数据类型 | 说明 |
|--------|----------|------|
| `ok` | boolean | 是否成功 |
| `adapters` | array | 适配器列表 |

---

#### `POST /api/webui/accounts/{id}/adapters` 创建适配器

**参数**:

| 字段名 | 数据类型 | 说明 |
|--------|----------|------|
| `type` | string | 类型：`http_post` / `reverse_ws` / `forward_ws` |
| `name` | string | 名称 |
| `enabled` | boolean | 是否启用 |
| `url` | string | 目标 URL（http_post / reverse_ws） |
| `access_token` | string | 连接 token |
| `reconnect_interval` | number | 重连间隔（秒） |
| `max_reconnect_count` | number | 最大重连次数 |

**响应**:

| 字段名 | 数据类型 | 说明 |
|--------|----------|------|
| `ok` | boolean | 是否成功 |
| `adapter` | object | 创建的适配器信息 |

---

#### `PUT /api/webui/accounts/{id}/adapters/{aid}` 更新适配器

**参数**: 同创建，局部更新

**响应**:

| 字段名 | 数据类型 | 说明 |
|--------|----------|------|
| `ok` | boolean | 是否成功 |
| `adapter` | object | 更新后的适配器信息 |

---

#### `DELETE /api/webui/accounts/{id}/adapters/{aid}` 删除适配器

**响应**:

| 字段名 | 数据类型 | 说明 |
|--------|----------|------|
| `ok` | boolean | 是否成功 |

---

#### `GET /api/webui/accounts/{id}/adapters/status` 获取适配器实时状态

**响应**:

| 字段名 | 数据类型 | 说明 |
|--------|----------|------|
| `ok` | boolean | 是否成功 |
| `adapters` | array | 各适配器连接状态 |

---

### 调试控制

以下接口用于远程操控在线账号的浏览器页面。

#### `GET /api/webui/accounts/{id}/screenshot` 获取页面截图

**响应**: PNG 图片二进制

---

#### `GET /api/webui/accounts/{id}/console` 获取页面状态

**响应**:

| 字段名 | 数据类型 | 说明 |
|--------|----------|------|
| `ok` | boolean | 是否成功 |
| `account` | object | 账号信息 |
| `title` | string | 页面标题 |
| `url` | string | 当前 URL |
| `logged_in` | boolean | 页面内是否已登录 |
| `body_len` | number | HTML body 长度 |
| `webpack` | string | webpack 可用性 |

---

#### `GET /api/webui/accounts/{id}/viewport` 获取视口大小

**响应**:

| 字段名 | 数据类型 | 说明 |
|--------|----------|------|
| `ok` | boolean | 是否成功 |
| `width` | number | 视口宽度 |
| `height` | number | 视口高度 |

---

#### `GET /api/webui/accounts/{id}/html` 获取页面 HTML

**响应**: HTML 文本（前 50KB）

---

#### `POST /api/webui/accounts/{id}/eval` 执行 JavaScript

**参数**:

| 字段名 | 数据类型 | 说明 |
|--------|----------|------|
| `js` | string | JavaScript 代码 |

**响应**:

| 字段名 | 数据类型 | 说明 |
|--------|----------|------|
| `ok` | boolean | 是否成功 |
| `result` | any | 执行结果 |

---

#### `POST /api/webui/accounts/{id}/click` 模拟点击

**参数**:

| 字段名 | 数据类型 | 说明 |
|--------|----------|------|
| `x` | number | X 坐标 |
| `y` | number | Y 坐标 |

**响应**:

| 字段名 | 数据类型 | 说明 |
|--------|----------|------|
| `ok` | boolean | 是否成功 |
| `element` | object | 点击位置的元素信息 |

---

#### `POST /api/webui/accounts/{id}/rightclick` 模拟右键点击

**参数**:

| 字段名 | 数据类型 | 说明 |
|--------|----------|------|
| `x` | number | X 坐标 |
| `y` | number | Y 坐标 |

**响应**:

| 字段名 | 数据类型 | 说明 |
|--------|----------|------|
| `ok` | boolean | 是否成功 |

---

#### `POST /api/webui/accounts/{id}/drag` 模拟拖拽

**参数**:

| 字段名 | 数据类型 | 说明 |
|--------|----------|------|
| `from_x` | number | 起点 X |
| `from_y` | number | 起点 Y |
| `to_x` | number | 终点 X |
| `to_y` | number | 终点 Y |
| `steps` | number | 步数（可选，自动计算） |

**响应**:

| 字段名 | 数据类型 | 说明 |
|--------|----------|------|
| `ok` | boolean | 是否成功 |
| `from` | object | 起点坐标 |
| `to` | object | 终点坐标 |
| `steps` | number | 实际步数 |

---

#### `POST /api/webui/accounts/{id}/key` 模拟按键

**参数**:

| 字段名 | 数据类型 | 说明 |
|--------|----------|------|
| `key` | string | 按键名（如 `Enter`、`Escape`、`Tab`、`Backspace`） |

**响应**:

| 字段名 | 数据类型 | 说明 |
|--------|----------|------|
| `ok` | boolean | 是否成功 |
| `key` | string | 按键名 |

---

#### `POST /api/webui/accounts/{id}/type` 模拟键入文本

**参数**:

| 字段名 | 数据类型 | 说明 |
|--------|----------|------|
| `x` | number | 点击位置 X（聚焦） |
| `y` | number | 点击位置 Y（聚焦） |
| `text` | string | 要输入的文本 |

**响应**:

| 字段名 | 数据类型 | 说明 |
|--------|----------|------|
| `ok` | boolean | 是否成功 |

---

#### `POST /api/webui/accounts/{id}/scroll` 模拟鼠标滚轮

**参数**:

| 字段名 | 数据类型 | 说明 |
|--------|----------|------|
| `x` | number | X 坐标 |
| `y` | number | Y 坐标 |
| `delta_x` | number | 水平滚动量 |
| `delta_y` | number | 垂直滚动量 |

**响应**:

| 字段名 | 数据类型 | 说明 |
|--------|----------|------|
| `ok` | boolean | 是否成功 |

---

#### `GET /api/webui/accounts/{id}/logs` 获取账号运行日志

**响应**: 日志文本流

---

## 通用响应格式

### OneBot v11 接口

所有 OneBot v11 action 接口返回统一格式:

```json
{
    "status": "ok",
    "retcode": 0,
    "data": { ... },
    "message": "",
    "wording": "",
    "echo": null
}
```

失败时:

```json
{
    "status": "failed",
    "retcode": 1500,
    "data": null,
    "message": "错误信息",
    "wording": "错误信息",
    "echo": null
}
```

### WebUI 管理接口

WebUI 接口统一返回 `{ "ok": true/false, ... }` 格式。

### 错误码

| 错误码 | 说明 |
|--------|------|
| `0` | 成功 |
| `1400` | 参数缺失或非法 |
| `1401` | 无可用账号 |
| `1403` | 鉴权失败 |
| `1404` | action 不存在 |
| `1500` | 执行失败 |

---

## 注意事项

1. **会话 ID**: 抖音的会话 ID 是字符串类型（shortId），不是数字
2. **用户 ID**: 支持 UID（数字）和 sec_uid（字符串）两种格式
3. **消息 ID**: 抖音的消息 ID 是字符串类型（server_id），不是数字
4. **SDK 依赖**: 所有接口都需要 SDK 已初始化，可通过 `get_login_info` 检查 `sdk_ready` 字段
5. **账号路由**: 请求中可携带 `X-Self-ID` header 路由到指定账号

---

## SSE 事件推送

### `GET /api/webui/events` 事件流

SSE (Server-Sent Events) 用于实时推送账号状态变更和消息事件。

**认证**: 需要在 URL 参数或 Header 中携带有效的 WebUI token。

**连接方式**:

```javascript
const token = 'your-webui-token';
const eventSource = new EventSource(`/api/webui/events?token=${token}`);

eventSource.onmessage = (event) => {
  const data = JSON.parse(event.data);
  console.log('Event:', data.type, data.data);
};
```

**事件类型**:

| 事件类型 | 说明 | 数据结构 |
|----------|------|----------|
| `connected` | 连接成功 | `{ client_id: string, timestamp: number }` |
| `account_status` | 账号状态变更 | `{ account_id: string, status: string, timestamp: number }` |
| `sdk_ready` | SDK 就绪 | `{ account_id: string, timestamp: number }` |
| `message` | 新消息 | `{ account_id: string, message: object, timestamp: number }` |
| `error` | 错误事件 | `{ account_id: string, error: string, timestamp: number }` |

**账号状态值**:

| 状态 | 说明 |
|------|------|
| `stopped` | 已停止 |
| `starting` | 启动中 |
| `online` | 在线 |
| `error` | 错误 |
| `qr_pending` | 等待扫码 |

**示例**:

```javascript
// 监听账号状态变更
eventSource.onmessage = (event) => {
  const data = JSON.parse(event.data);

  switch (data.type) {
    case 'connected':
      console.log('SSE 已连接:', data.data.client_id);
      break;
    case 'account_status':
      console.log('账号状态变更:', data.data.account_id, data.data.status);
      break;
    case 'sdk_ready':
      console.log('SDK 就绪:', data.data.account_id);
      break;
    case 'message':
      console.log('收到消息:', data.data.account_id, data.data.message);
      break;
    case 'error':
      console.error('错误:', data.data.account_id, data.data.error);
      break;
  }
};
```

**注意事项**:

1. SSE 连接需要有效的 WebUI token
2. 连接断开后会自动重连（建议客户端实现重连逻辑）
3. 事件数据格式为 JSON，需要手动解析
4. 每个客户端有独立的连接和事件流
