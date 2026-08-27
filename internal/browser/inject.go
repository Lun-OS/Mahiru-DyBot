package browser

// 本文件包含所有注入到抖音页面的 JavaScript。
// 全部来自对 https://www.douyin.com/chat 页面的实际逆向探索（见 explore/ 目录）。
//
// 核心链路：
//   window.webpackChunkdouyin_web
//     -> push 注入模块拿到 __webpack_require__
//     -> 动态遍历 __webpack_require.m 找到导出 ZP 的模块（不同会话模块 ID 会变）
//     -> require(modId).ZP() 拿到 IM 上下文 Promise
//     -> imSdkService.imSdkManager.getImSdkInstance() 拿到 SDK 实例
//   发送: createMessage + sendMessage（服务器直接返回 serverId）
//   接收: newMessagePushManager.registerNewMessagePush(cb)

// jsBootstrap 初始化 IM 上下文与 SDK 实例。幂等，可重复执行。
// 动态搜索含 ZP 导出的 webpack 模块（硬编码 ID 在不同会话/UA 下会变）。
// 带超时保护，避免 ZP() Promise 永远挂起。
const jsBootstrap = `
(async () => {
    var _globalTimeout = new Promise(function(_, rej) { setTimeout(function() { rej(new Error('global timeout 25s')); }, 25000); });
    var _work = (async () => {
    try {
        if (!window.__imCtx || !window.__sdkInst) {
            if (!window.__wpRequire) {
                var captured = null;
                window.webpackChunkdouyin_web.push([['__onebot_bootstrap__'], {}, function (wr) { captured = wr; }]);
                window.__wpRequire = captured;
            }
            if (!window.__wpRequire) {
                return JSON.stringify({ ok: false, error: 'webpackChunkdouyin_web 不可用' });
            }
            var candidates = [];
            var wpM = window.__wpRequire.m || {};
            var keys = Object.keys(wpM);
            for (var i = 0; i < keys.length; i++) {
                try {
                    var mod = window.__wpRequire(parseInt(keys[i]));
                    if (mod && typeof mod === 'object' && typeof mod.ZP === 'function') {
                        candidates.push(parseInt(keys[i]));
                    }
                } catch (e) {}
            }
            if (candidates.length === 0) {
                return JSON.stringify({ ok: false, error: '未找到含 ZP 导出的模块 (共 ' + keys.length + ' 个)' });
            }
            var lastErr = '';
            for (var j = 0; j < candidates.length; j++) {
                try {
                    var ctxPromise = window.__wpRequire(candidates[j]).ZP();
                    var ctx = await Promise.race([
                        ctxPromise,
                        new Promise(function(_, rej) { setTimeout(function() { rej(new Error('timeout')); }, 8000); })
                    ]);
                    if (ctx && ctx.imSdkService && ctx.imSdkService.imSdkManager) {
                        window.__imCtx = ctx;
                        window.__sdkInst = ctx.imSdkService.imSdkManager.getImSdkInstance();
                        if (window.__sdkInst) {
                            window.__obModId = candidates[j];
                            break;
                        }
                    }
                    window.__imCtx = null;
                    window.__sdkInst = null;
                } catch (e) { lastErr = e.message || String(e); }
            }
            if (!window.__sdkInst) {
                return JSON.stringify({ ok: false, error: '未找到 IM SDK 模块 (' + candidates.length + ' 个 ZP 候选): ' + lastErr });
            }
        }
        var selfUid = '';
        try { selfUid = String(window.__sdkInst.ctx.option.userId || ''); } catch (e) {}
        return JSON.stringify({ ok: true, self_uid: selfUid, mod_id: window.__obModId });
    } catch (e) {
        return JSON.stringify({ ok: false, error: e.message });
    }
    })();
    return Promise.race([_work, _globalTimeout]);
})()
`

// jsCheckLogin 检测登录状态并返回完整用户信息。
const jsCheckLogin = `
() => {
    try {
        var loggedIn = false;
        try { loggedIn = !!(window.userInfoStore && window.userInfoStore.curLoginUserInfo); } catch (e) {}
        if (!loggedIn) {
            loggedIn = !!(window.__sdkInst);
        }
        try {
            var panel = document.querySelector('[class*="login-panel-new"], [class*="login-full-panel"]');
            if (panel && panel.offsetParent !== null) loggedIn = false;
        } catch (e) {}

        var user = null;
        try {
            var s = window.userInfoStore;
            var u = s && (s.curLoginUserInfo || s.userInfo || null);
            if (u) {
                user = {
                    uid: String(u.uid || u.uid_str || u.user_id || (u.user && u.user.uid) || ''),
                    sec_uid: String(u.sec_uid || (u.user && u.user.sec_uid) || ''),
                    nickname: String(u.nickname || u.nick_name || (u.user && u.user.nickname) || ''),
                    avatar: String(u.avatar_uri || u.avatar_thumb && u.avatar_thumb.url_list && u.avatar_thumb.url_list[0] || (u.user && u.user.avatar_thumb && u.user.avatar_thumb.url_list && u.user.avatar_thumb.url_list[0]) || ''),
                    unique_id: String(u.unique_id || (u.user && u.user.unique_id) || ''),
                    short_id: String(u.short_id || (u.user && u.user.short_id) || ''),
                    signature: String(u.signature || (u.user && u.user.signature) || ''),
                    gender: Number(u.gender || (u.user && u.user.gender) || 0),
                    enterprise: Boolean(u.enterprise_verify_status || (u.user && u.user.enterprise_verify_status) || false),
                    custom_verify: String(u.custom_verify || (u.user && u.user.custom_verify) || ''),
                };
            }
        } catch (e) {}

        var sdkReady = false;
        try { sdkReady = !!(window.__sdkInst && window.__imCtx); } catch (e) {}
        var modId = 0;
        try { modId = Number(window.__obModId) || 0; } catch (e) {}

        return JSON.stringify({
            logged_in: loggedIn,
            user: user,
            sdk_ready: sdkReady,
            mod_id: modId,
        });
    } catch (e) {
        return JSON.stringify({ logged_in: false, error: e.message });
    }
}
`

// jsSendMessage 通过 SDK 发送文本消息。
// 关键：必须用 __sdkInst.createMessage + __sdkInst.sendMessage，
// 而非 sendMessageManager.sendMessage（后者的 context 缺少 option）。
// 参数: [{ uid: 目标用户数字uid, text: 消息内容 }]
const jsSendMessage = `
async (args) => {
    try {
        var inst = window.__sdkInst;
        var cm = window.__imCtx && window.__imCtx.imSdkService && window.__imCtx.imSdkService.conversationManager;
        if (!inst || !cm) return JSON.stringify({ ok: false, error: 'SDK 未初始化', stage: 'init' });

        var uid = String(args[0].uid);
        var text = String(args[0].text);

        var conv = await cm.getOrCreatePrivateConversationByUid(uid);
        if (!conv) return JSON.stringify({ ok: false, error: '无法定位会话: ' + uid, stage: 'get_conv' });

        var msg = await inst.createMessage({
            conversation: conv,
            content: JSON.stringify({ aweType: 700, type: 0, richTextInfos: [], text: text }),
            type: 7,
            insert: true
        });
        var sendRes = await inst.sendMessage({ message: msg });

        // 解析响应：SDK 返回 {success: bool, payload: {...}} 或 {code: number}
        var isOk = false;
        var errorMsg = '';
        var sendDetail = '';
        try { sendDetail = JSON.stringify(sendRes); } catch (e) { sendDetail = String(sendRes); }
        if (sendDetail.length > 2000) sendDetail = sendDetail.substring(0, 2000);

        if (sendRes && sendRes.success !== undefined) {
            isOk = !!sendRes.success;
            if (!isOk && sendRes.payload && sendRes.payload.error) {
                errorMsg = String(sendRes.payload.error);
            }
        } else if (sendRes && sendRes.code !== undefined) {
            isOk = sendRes.code === 0;
            if (!isOk) errorMsg = 'code=' + sendRes.code + ' ' + (sendRes.msg || sendRes.message || '');
        } else {
            isOk = true;
        }

        // 消息对象上的投递状态
        var serverId = String(msg.serverId || '');
        var clientId = String(msg.clientId || '');
        var msgStatus = msg.status !== undefined ? msg.status : -1;
        var sendStatus = msg.sendStatus !== undefined ? msg.sendStatus : -1;
        var errorCode = msg.errorCode || 0;
        var errorMsg2 = msg.errorMsg || '';
        var flightStatus = msg.flightStatus !== undefined ? msg.flightStatus : -1;

        // 从 ext 中提取服务端拒绝详情（privacy/ban 等）
        var serverCheckCode = 0;
        var serverCheckMsg = '';
        try {
            var ext = msg.ext || {};
            var checkMsgRaw = ext['s:send_response_check_msg'] || '';
            if (checkMsgRaw) {
                var checkObj = typeof checkMsgRaw === 'string' ? JSON.parse(checkMsgRaw) : checkMsgRaw;
                serverCheckCode = checkObj.status_code || checkObj.raw_check_code || 0;
                if (checkObj.status_msg && checkObj.status_msg.msg_content) {
                    serverCheckMsg = checkObj.status_msg.msg_content.tips || '';
                }
                if (!serverCheckMsg && checkObj.tips) {
                    serverCheckMsg = checkObj.tips;
                }
            }
        } catch (e) {}

        if (!isOk) {
            errorMsg = errorMsg || serverCheckMsg || 'SDK 发送失败';
        }
        if (errorCode !== 0) {
            errorMsg = '消息错误: code=' + errorCode + ' msg=' + errorMsg2;
            isOk = false;
        }
        if (serverCheckCode !== 0 && !serverCheckMsg) {
            serverCheckMsg = '服务端拒绝: code=' + serverCheckCode;
        }

        return JSON.stringify({
            ok: isOk,
            server_id: serverId,
            client_id: clientId,
            conversation_id: String(conv.id || ''),
            conversation_short_id: String(conv.shortId || ''),
            error: errorMsg,
            msg_status: msgStatus,
            msg_send_status: sendStatus,
            msg_error_code: errorCode,
            msg_error_msg: errorMsg2,
            flight_status: flightStatus,
            server_check_code: serverCheckCode,
            server_check_msg: serverCheckMsg,
            send_response: sendDetail,
        });
    } catch (e) {
        return JSON.stringify({ ok: false, error: e.message || String(e), stage: 'exception' });
    }
}
`

// jsSendGroupMessage 通过 SDK 发送群聊消息。
// 参数: [{ group_id: 群会话shortId 或 id, text: 消息内容 }]
// 注意: 群聊会话必须已存在于会话列表中（已加入的群）。
const jsSendGroupMessage = `
async (args) => {
    try {
        var inst = window.__sdkInst;
        if (!inst) return JSON.stringify({ ok: false, error: 'SDK 未初始化', stage: 'init' });

        var groupId = String(args[0].group_id);
        var text = String(args[0].text);

        var convList = inst.getConversationList() || [];
        var conv = null;
        for (var i = 0; i < convList.length; i++) {
            var c = convList[i];
            if (String(c.shortId) === groupId || String(c.id) === groupId) {
                conv = c;
                break;
            }
        }
        if (!conv) return JSON.stringify({ ok: false, error: '未找到群聊会话: ' + groupId, stage: 'get_conv' });

        var msg = await inst.createMessage({
            conversation: conv,
            content: JSON.stringify({ aweType: 700, type: 0, richTextInfos: [], text: text }),
            type: 7,
            insert: true
        });
        var sendRes = await inst.sendMessage({ message: msg });

        var isOk = false;
        var errorMsg = '';
        if (sendRes && sendRes.success !== undefined) {
            isOk = !!sendRes.success;
            if (!isOk && sendRes.payload && sendRes.payload.error) errorMsg = String(sendRes.payload.error);
        } else if (sendRes && sendRes.code !== undefined) {
            isOk = sendRes.code === 0;
            if (!isOk) errorMsg = 'code=' + sendRes.code + ' ' + (sendRes.msg || '');
        } else {
            isOk = true;
        }

        var errorCode = msg.errorCode || 0;
        var errorMsg2 = msg.errorMsg || '';
        if (errorCode !== 0) { errorMsg = '消息错误: code=' + errorCode + ' msg=' + errorMsg2; isOk = false; }

        var sendDetail = '';
        try { sendDetail = JSON.stringify(sendRes); } catch (e) { sendDetail = String(sendRes); }
        if (sendDetail.length > 2000) sendDetail = sendDetail.substring(0, 2000);

        return JSON.stringify({
            ok: isOk,
            server_id: String(msg.serverId || ''),
            client_id: String(msg.clientId || ''),
            conversation_id: String(conv.id || ''),
            conversation_short_id: String(conv.shortId || ''),
            error: errorMsg,
            msg_error_code: errorCode,
            msg_error_msg: errorMsg2,
            send_response: sendDetail,
        });
    } catch (e) {
        return JSON.stringify({ ok: false, error: e.message || String(e), stage: 'exception' });
    }
}
`

// jsGetConversationList 获取会话列表（含对方uid映射）。
const jsGetConversationList = `
async () => {
    try {
        var inst = window.__sdkInst;
        if (!inst) return JSON.stringify({ ok: false, error: 'SDK 未初始化' });
        var uis = window.__imCtx ? window.__imCtx.store.usersInfoStore : null;
        var list = await inst.getConversationList();
        var out = [];
        var secUidsToFetch = [];
        var convToSecUid = {};
        for (var i = 0; i < list.length; i++) {
            var c = list[i];
            var toUid = '';
            try { toUid = c.toParticipantUserId === undefined ? '' : String(c.toParticipantUserId); } catch (e) {}
            var toSecUid = '';
            try {
                var parts = await inst.getConversationParticipants({ conversation: c });
                var selfUid = '';
                try { selfUid = String((window.userInfoStore && window.userInfoStore.curLoginUserInfo) ? window.userInfoStore.curLoginUserInfo.uid : ''); } catch (e) {}
                for (var j = 0; j < parts.length; j++) {
                    if (parts[j].userId !== selfUid && parts[j].secUid) {
                        toSecUid = parts[j].secUid;
                        break;
                    }
                }
            } catch (e) {}
            if (toSecUid) {
                secUidsToFetch.push(toSecUid);
                convToSecUid[String(c.id)] = toSecUid;
            }
        }
        if (uis && secUidsToFetch.length > 0) {
            try { await uis.doRequestUsersInfoIfNeeded(secUidsToFetch); } catch (e) {}
        }
        for (var i = 0; i < list.length; i++) {
            var c = list[i];
            var name = '';
            try { name = (c.coreInfo && c.coreInfo.name) || ''; } catch (e) {}
            var toUid = '';
            try { toUid = c.toParticipantUserId === undefined ? '' : String(c.toParticipantUserId); } catch (e) {}
            var toSecUid = convToSecUid[String(c.id)] || '';
            var nickname = '', remark = '', uniqueId = '', shortIdNum = '', avatar = '';
            if (uis && toSecUid) {
                try {
                    var u = uis.getUserBySecUid(toSecUid);
                    if (!u) u = await uis.getUserBySecUidAsync(toSecUid);
                    if (u) {
                        nickname = u.nickname || '';
                        remark = u.remark_name || '';
                        uniqueId = u.unique_id || '';
                        shortIdNum = u.short_id || '';
                        avatar = u.avatar_uri || '';
                    }
                } catch (e) {}
            }
            out.push({
                id: String(c.id),
                short_id: String(c.shortId),
                type: Number(c.type),
                to_uid: toUid,
                to_sec_uid: toSecUid,
                name: name,
                nickname: nickname,
                remark_name: remark,
                unique_id: uniqueId,
                short_id_num: shortIdNum,
                avatar: avatar,
                unread: Number(c._badgeCount || 0)
            });
        }
        return JSON.stringify({ ok: true, items: out });
    } catch (e) {
        return JSON.stringify({ ok: false, error: e.message });
    }
}
`

// jsGetHistoryMessages 获取指定会话的历史消息（多次 fetchConversation 加载更多）。
const jsGetHistoryMessages = `
async (args) => {
    try {
        var inst = window.__sdkInst;
        if (!inst) return JSON.stringify({ ok: false, error: 'SDK 未初始化' });
        var uid = String(args[0].uid);
        var count = Math.max(1, Math.min(Number(args[0].count) || 20, 200));

        var cm = window.__imCtx.imSdkService.conversationManager;
        var conv = await cm.getOrCreatePrivateConversationByUid(uid);
        if (!conv) return JSON.stringify({ ok: false, error: '无法定位会话' });

        var prevLen = 0;
        for (var attempt = 0; attempt < 8; attempt++) {
            try { await inst.fetchConversation({ conversation: conv }); } catch (e) {}
            var msgs = inst.getConversationMessages({ conversation: conv }) || [];
            if (msgs.length >= count || msgs.length === prevLen) break;
            prevLen = msgs.length;
            await new Promise(function(r) { setTimeout(r, 500); });
        }

        var msgs = inst.getConversationMessages({ conversation: conv }) || [];
        var start = Math.max(0, msgs.length - count);
        var out = [];
        for (var i = start; i < msgs.length; i++) {
            var m = msgs[i];
            var createdAt = 0;
            try { createdAt = Number(m.createdAt) || 0; } catch (e) {}
            var text = '';
            var rawContent = '';
            try {
                rawContent = typeof m.content === 'string' ? m.content : '';
                var parsed = JSON.parse(rawContent);
                text = parsed.text || '';
                if (text.indexOf('\\n') === 0) text = text.substring(1);
            } catch (e) { text = rawContent; }
            out.push({
                sender: String(m.sender || ''),
                type: Number(m.type || 0),
                text: text,
                content_raw: rawContent,
                client_id: String(m.clientId || ''),
                server_id: String(m.serverId || ''),
                created_at: Math.floor(createdAt / 1000)
            });
        }
        return JSON.stringify({ ok: true, total: msgs.length, items: out });
    } catch (e) {
        return JSON.stringify({ ok: false, error: e.message });
    }
}
`

// jsGetQRCode 从页面 DOM 截取实际展示的 QR 码（确保与页面登录通道一致）。
// 不调用 passport API 获取 token，由 WaitLoginSuccess 轮询页面登录状态。
// 返回: { ok: true, image_base64: "...", token: "", method: "dom" }
const jsGetQRCode = `
async () => {
    try {
        // 方案A：查找 QR 图片元素
        var imgs = document.querySelectorAll('img');
        for (var i = 0; i < imgs.length; i++) {
            var src = imgs[i].src || '';
            if (src.indexOf('base64,') >= 0 && imgs[i].width >= 100 && imgs[i].width <= 300) {
                var b64 = src.split('base64,')[1];
                if (b64 && b64.length > 100) {
                    return JSON.stringify({ ok: true, image_base64: b64, token: '', method: 'img' });
                }
            }
        }
        // 方案B：canvas → base64
        var canvas = document.querySelector('canvas');
        if (canvas && canvas.width >= 100 && canvas.width <= 300) {
            var dataURL = canvas.toDataURL('image/png');
            var b64 = dataURL.indexOf('base64,') >= 0 ? dataURL.split('base64,')[1] : '';
            if (b64 && b64.length > 100) {
                return JSON.stringify({ ok: true, image_base64: b64, token: '', method: 'canvas' });
            }
        }
        // 方案C：查找 QR 容器内的 img（class 名可能动态变化）
        var qrContainers = document.querySelectorAll('[class*="qr"], [class*="QR"], [class*="scan"], [class*="code"]');
        for (var i = 0; i < qrContainers.length; i++) {
            var innerImg = qrContainers[i].querySelector('img');
            if (innerImg && innerImg.src && innerImg.src.indexOf('base64,') >= 0) {
                var b64 = innerImg.src.split('base64,')[1];
                if (b64 && b64.length > 100) {
                    return JSON.stringify({ ok: true, image_base64: b64, token: '', method: 'container' });
                }
            }
        }
        return JSON.stringify({ ok: false, error: '页面中未找到 QR 码元素，页面可能未加载登录面板' });
    } catch (e) {
        return JSON.stringify({ ok: false, error: e.message });
    }
}
`

// jsCheckQRCode 轮询扫码状态。
// 参数: [{ token: "..." }]
// 返回: { ok: true, status: 0/1/2/3, user_info: {...} }
const jsCheckQRCode = `
async (args) => {
    try {
        var token = args[0].token;
        var baseUrl = 'https://login.douyin.com/passport/web/check_qrconnect/';
        var params = 'passport_jssdk_version=3.4.2&passport_jssdk_type=normal&is_from_ttaccountsdk=1&aid=6383&language=zh&account_app_language=zh-CN&ts=' + Math.floor(Date.now()/1000);
        var body = 'need_logo=false&is_frontier=true&token=' + encodeURIComponent(token) + '&is_new_login=1&next=https%3A%2F%2Fwww.douyin.com&need_short_url=true';
        var resp = await fetch(baseUrl + '?' + params, {
            method: 'POST',
            credentials: 'include',
            headers: {
                'Content-Type': 'application/x-www-form-urlencoded',
                'web-sdk-version': '3.4.2'
            },
            body: body
        });
        var data = await resp.json();
        if (data.data) {
            var rawStatus = data.data.status || 0;
            var statusNum = 0;
            if (typeof rawStatus === 'number') {
                statusNum = rawStatus;
            } else if (typeof rawStatus === 'string') {
                if (rawStatus === 'expired' || rawStatus === 'timeout') statusNum = 3;
                else if (rawStatus === 'scanned' || rawStatus === 'confirmed') statusNum = 2;
                else if (rawStatus === 'new' || rawStatus === 'pending') statusNum = 0;
                else statusNum = 0;
            }
            return JSON.stringify({
                ok: true,
                status: statusNum,
                raw_status: rawStatus,
                user_info: data.data.user_info || null,
                redirect_url: data.data.redirect_url || '',
                error_code: data.data.error_code || 0
            });
        }
        return JSON.stringify({ ok: false, error: data.message || 'unknown' });
    } catch (e) {
        return JSON.stringify({ ok: false, error: e.message });
    }
}
`

// jsScanQRCode 确认扫码登录（用户扫码后调用）。
// 参数: [{ token: "..." }]
const jsScanQRCode = `
async (args) => {
    try {
        var token = args[0].token;
        var resp = await fetch('https://www.douyin.com/passport/web/scan_qrcode/', {
            method: 'POST',
            credentials: 'include',
            headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
            body: 'token=' + encodeURIComponent(token) + '&action=2&aid=6383'
        });
        var data = await resp.json();
        return JSON.stringify({ ok: data.data && data.data.error_code === 0, data: data });
    } catch (e) {
        return JSON.stringify({ ok: false, error: e.message });
    }
}
`

// jsCheckLoginStatus 检测登录状态（通过 cookie 或 API）。
const jsCheckLoginStatus = `
async () => {
    try {
        var resp = await fetch('https://www.douyin.com/passport/account/info/v2/', {
            method: 'GET',
            credentials: 'include'
        });
        var data = await resp.json();
        if (data.data && data.data.user) {
            return JSON.stringify({ logged_in: true, user_id: data.data.user.uid_str || '', nickname: data.data.user.nickname || '' });
        }
        return JSON.stringify({ logged_in: false });
    } catch (e) {
        return JSON.stringify({ logged_in: false, error: e.message });
    }
}
`

// jsGetSelfInfo 获取自身 uid 与昵称（EnsureReady 成功后调用）。
const jsGetSelfInfo = `
() => {
    var uid = '', nick = '';
    try { uid = String(window.__sdkInst && window.__sdkInst.ctx && window.__sdkInst.ctx.option && window.__sdkInst.ctx.option.userId || ''); } catch (e) {}
    try {
        var s = window.userInfoStore;
        var u = s && (s.curLoginUserInfo || s.userInfo || null);
        if (u) {
            nick = u.nickname || u.nick_name || (u.user && u.user.nickname) || '';
            if (!uid) uid = String(u.uid || u.uid_str || u.user_id || (u.user && u.user.uid) || '');
        }
    } catch (e) {}
    return JSON.stringify({ uid: uid, nickname: nick });
}
`

// jsGetSDKStatus 获取完整的 SDK 运行时状态信息。
const jsGetSDKStatus = `
() => {
    try {
        var sdkReady = !!(window.__sdkInst && window.__imCtx);
        var modId = Number(window.__obModId) || 0;
        var convCount = 0;
        var recvRegistered = !!window.__recvRegistered;

        try {
            if (window.__sdkInst) {
                var cl = window.__sdkInst.getConversationList();
                if (cl) convCount = cl.length;
            }
        } catch (e) {}

        var selfUid = '';
        try { selfUid = String(window.__sdkInst.ctx.option.userId || ''); } catch (e) {}

        var connectionStatus = 'unknown';
        try {
            var cs = window.__imCtx.imSdkService;
            if (cs && cs.connectionManager) {
                var st = cs.connectionManager.status || cs.connectionManager.state;
                connectionStatus = String(st || 'unknown');
            }
        } catch (e) {}

        return JSON.stringify({
            sdk_ready: sdkReady,
            self_uid: selfUid,
            mod_id: modId,
            conversation_count: convCount,
            receiver_registered: recvRegistered,
            connection_status: connectionStatus,
        });
    } catch (e) {
        return JSON.stringify({ ok: false, error: e.message });
    }
}
`

// jsGetUserInfo 获取用户详细信息。
// 参数: [{ user_id: "uid或sec_uid" }]
const jsGetUserInfo = `
async (args) => {
    try {
        var inst = window.__sdkInst;
        var ctx = window.__imCtx;
        if (!inst || !ctx) return JSON.stringify({ ok: false, error: 'SDK 未初始化' });
        var us = ctx.store.usersInfoStore;
        var target = String(args[0].user_id);
        // 尝试作为 sec_uid 查询
        var user = await us.getUserBySecUidAsync(target);
        if (!user) {
            // 尝试作为 uid 查询 - 遍历所有用户
            var all = us.allUsers ? us.allUsers() : [];
            for (var i = 0; i < all.length; i++) {
                if (String(all[i].uid) === target) { user = all[i]; break; }
            }
        }
        if (!user) return JSON.stringify({ ok: false, error: '用户不存在: ' + target });
        return JSON.stringify({
            ok: true,
            user: {
                uid: String(user.uid || ''),
                sec_uid: String(user.sec_uid || ''),
                nickname: String(user.nickname || ''),
                unique_id: String(user.unique_id || ''),
                short_id: String(user.short_id || ''),
                signature: String(user.signature || ''),
                avatar_thumb: (user.avatar_thumb && user.avatar_thumb.url_list && user.avatar_thumb.url_list[0]) || '',
                avatar_small: (user.avatar_small && user.avatar_small.url_list && user.avatar_small.url_list[0]) || '',
                follow_status: Number(user.follow_status || 0),
                follower_status: Number(user.follower_status || 0),
                verification_type: Number(user.verification_type || 0),
                custom_verify: String(user.custom_verify || ''),
                enterprise_verify_reason: String(user.enterprise_verify_reason || ''),
                store_region: String(user.store_region || ''),
            }
        });
    } catch (e) {
        return JSON.stringify({ ok: false, error: e.message });
    }
}
`

// jsGetStrangers 获取陌生人会话列表。
const jsGetStrangers = `
async () => {
    try {
        var inst = window.__sdkInst;
        if (!inst) return JSON.stringify({ ok: false, error: 'SDK 未初始化' });
        var list = await inst.getStrangerConversationList();
        var items = [];
        if (list && list.length) {
            for (var i = 0; i < list.length; i++) {
                var c = list[i];
                items.push({
                    id: String(c.id || ''),
                    short_id: String(c.shortId || ''),
                    name: String(c.coreInfo && c.coreInfo.name || ''),
                });
            }
        }
        return JSON.stringify({ ok: true, items: items });
    } catch (e) {
        return JSON.stringify({ ok: false, error: e.message });
    }
}
`

// jsFollowUser 关注用户。
// 参数: [{ sec_uid: "目标用户 sec_uid" }]
const jsFollowUser = `
async (args) => {
    try {
        var us = window.__imCtx && window.__imCtx.store.usersInfoStore;
        if (!us) return JSON.stringify({ ok: false, error: '用户 Store 不可用' });
        var secUid = String(args[0].sec_uid);
        await us.followUser(secUid);
        return JSON.stringify({ ok: true });
    } catch (e) {
        return JSON.stringify({ ok: false, error: e.message });
    }
}
`

// jsSetUserBlockStatus 拉黑/取消拉黑用户。
// 参数: [{ sec_uid: "目标用户 sec_uid", block: true/false }]
const jsSetUserBlockStatus = `
async (args) => {
    try {
        var us = window.__imCtx && window.__imCtx.store.usersInfoStore;
        if (!us) return JSON.stringify({ ok: false, error: '用户 Store 不可用' });
        var secUid = String(args[0].sec_uid);
        var block = !!args[0].block;
        await us.setUserBlockStatus(secUid, block);
        return JSON.stringify({ ok: true });
    } catch (e) {
        return JSON.stringify({ ok: false, error: e.message });
    }
}
`

// jsUpdateUserRemarkName 修改好友备注名。
// 参数: [{ sec_uid: "目标用户 sec_uid", remark: "新备注" }]
const jsUpdateUserRemarkName = `
async (args) => {
    try {
        var us = window.__imCtx && window.__imCtx.store.usersInfoStore;
        if (!us) return JSON.stringify({ ok: false, error: '用户 Store 不可用' });
        var secUid = String(args[0].sec_uid);
        var remark = String(args[0].remark);
        await us.updateUserRemarkName(secUid, remark);
        return JSON.stringify({ ok: true });
    } catch (e) {
        return JSON.stringify({ ok: false, error: e.message });
    }
}
`

// jsRecallMessage 撤回消息。
// 参数: [{ conversation_id: "会话 shortId", server_id: "消息 serverId" }]
const jsRecallMessage = `
async (args) => {
    try {
        var inst = window.__sdkInst;
        var cm = window.__imCtx && window.__imCtx.imSdkService.conversationManager;
        if (!inst || !cm) return JSON.stringify({ ok: false, error: 'SDK 未初始化' });
        var convId = String(args[0].conversation_id);
        var serverId = String(args[0].server_id);
        // 获取会话对象
        var conv = cm.getConversationById ? cm.getConversationById(convId) : null;
        if (!conv) return JSON.stringify({ ok: false, error: '会话不存在: ' + convId });
        // 获取消息对象
        var msgs = inst.getConversationMessages({ conversation: conv });
        var targetMsg = null;
        for (var i = msgs.length - 1; i >= 0; i--) {
            if (String(msgs[i].serverId) === serverId) { targetMsg = msgs[i]; break; }
        }
        if (!targetMsg) return JSON.stringify({ ok: false, error: '消息不存在: ' + serverId });
        var result = await inst.recallMessage({ conversation: conv, message: targetMsg });
        return JSON.stringify({ ok: true, result: result });
    } catch (e) {
        return JSON.stringify({ ok: false, error: e.message });
    }
}
`

// jsDeleteMessage 删除消息。
// 参数: [{ conversation_id: "会话 shortId", server_id: "消息 serverId" }]
const jsDeleteMessage = `
async (args) => {
    try {
        var inst = window.__sdkInst;
        var cm = window.__imCtx && window.__imCtx.imSdkService.conversationManager;
        if (!inst || !cm) return JSON.stringify({ ok: false, error: 'SDK 未初始化' });
        var convId = String(args[0].conversation_id);
        var serverId = String(args[0].server_id);
        var conv = cm.getConversationById ? cm.getConversationById(convId) : null;
        if (!conv) return JSON.stringify({ ok: false, error: '会话不存在: ' + convId });
        var msgs = inst.getConversationMessages({ conversation: conv });
        var targetMsg = null;
        for (var i = msgs.length - 1; i >= 0; i--) {
            if (String(msgs[i].serverId) === serverId) { targetMsg = msgs[i]; break; }
        }
        if (!targetMsg) return JSON.stringify({ ok: false, error: '消息不存在: ' + serverId });
        var result = await inst.deleteMessage({ conversation: conv, message: targetMsg });
        return JSON.stringify({ ok: true, result: result });
    } catch (e) {
        return JSON.stringify({ ok: false, error: e.message });
    }
}
`

// jsLikeMessage 点赞消息。
// 参数: [{ conversation_id: "会话 shortId", server_id: "消息 serverId" }]
const jsLikeMessage = `
async (args) => {
    try {
        var inst = window.__sdkInst;
        var cm = window.__imCtx && window.__imCtx.imSdkService.conversationManager;
        if (!inst || !cm) return JSON.stringify({ ok: false, error: 'SDK 未初始化' });
        var convId = String(args[0].conversation_id);
        var serverId = String(args[0].server_id);
        var conv = cm.getConversationById ? cm.getConversationById(convId) : null;
        if (!conv) return JSON.stringify({ ok: false, error: '会话不存在: ' + convId });
        var msgs = inst.getConversationMessages({ conversation: conv });
        var targetMsg = null;
        for (var i = msgs.length - 1; i >= 0; i--) {
            if (String(msgs[i].serverId) === serverId) { targetMsg = msgs[i]; break; }
        }
        if (!targetMsg) return JSON.stringify({ ok: false, error: '消息不存在: ' + serverId });
        var result = await inst.modifyMessageProperty({ conversation: conv, message: targetMsg, property: 'like', value: true });
        return JSON.stringify({ ok: true, result: result });
    } catch (e) {
        return JSON.stringify({ ok: false, error: e.message });
    }
}
`

// jsReplyMessage 回复消息。
// 参数: [{ conversation_id: "会话 shortId", server_id: "被回复消息 serverId", text: "回复内容" }]
const jsReplyMessage = `
async (args) => {
    try {
        var inst = window.__sdkInst;
        var cm = window.__imCtx && window.__imCtx.imSdkService.conversationManager;
        if (!inst || !cm) return JSON.stringify({ ok: false, error: 'SDK 未初始化', stage: 'init' });
        var convId = String(args[0].conversation_id);
        var serverId = String(args[0].server_id);
        var text = String(args[0].text);
        if (!text) return JSON.stringify({ ok: false, error: 'text 为空' });
        var conv = cm.getConversationById ? cm.getConversationById(convId) : null;
        if (!conv) return JSON.stringify({ ok: false, error: '会话不存在: ' + convId });
        // 获取被回复的消息
        var msgs = inst.getConversationMessages({ conversation: conv });
        var replyTo = null;
        for (var i = msgs.length - 1; i >= 0; i--) {
            if (String(msgs[i].serverId) === serverId) { replyTo = msgs[i]; break; }
        }
        if (!replyTo) return JSON.stringify({ ok: false, error: '被回复消息不存在: ' + serverId });
        // 创建消息并回复
        var msg = await inst.createMessage({
            conversation: conv,
            content: JSON.stringify({ aweType: 700, type: 0, richTextInfos: [], text: text }),
            type: 7,
            insert: true,
            replyMessage: replyTo
        });
        var sendRes = await inst.sendMessage({ message: msg });
        var isOk = false;
        var errorMsg = '';
        if (sendRes && sendRes.success !== undefined) {
            isOk = !!sendRes.success;
            if (!isOk && sendRes.payload && sendRes.payload.error) errorMsg = String(sendRes.payload.error);
        } else if (sendRes && sendRes.code !== undefined) {
            isOk = sendRes.code === 0;
            if (!isOk) errorMsg = 'code=' + sendRes.code;
        } else { isOk = true; }
        var result = {
            ok: isOk,
            server_id: String(msg.serverId || ''),
            client_id: String(msg.clientId || ''),
            error: errorMsg
        };
        // 提取服务端拒绝
        try {
            var ext = msg.ext || {};
            var checkMsgRaw = ext['s:send_response_check_msg'] || '';
            if (checkMsgRaw) {
                var checkObj = typeof checkMsgRaw === 'string' ? JSON.parse(checkMsgRaw) : checkMsgRaw;
                result.server_check_code = checkObj.status_code || 0;
                if (checkObj.status_msg && checkObj.status_msg.msg_content) {
                    result.server_check_msg = checkObj.status_msg.msg_content.tips || '';
                }
            }
        } catch (e) {}
        return JSON.stringify(result);
    } catch (e) {
        return JSON.stringify({ ok: false, error: e.message });
    }
}
`

// jsSetConversationPin 置顶/取消置顶会话。
// 参数: [{ conversation_id: "会话 shortId", pinned: true/false }]
const jsSetConversationPin = `
async (args) => {
    try {
        var inst = window.__sdkInst;
        var cm = window.__imCtx && window.__imCtx.imSdkService.conversationManager;
        if (!inst || !cm) return JSON.stringify({ ok: false, error: 'SDK 未初始化' });
        var convId = String(args[0].conversation_id);
        var pinned = !!args[0].pinned;
        var conv = cm.getConversationById ? cm.getConversationById(convId) : null;
        if (!conv) return JSON.stringify({ ok: false, error: '会话不存在: ' + convId });
        var result = await inst.setConversationSettingInfo({ conversation: conv, settingType: 'pin', value: pinned });
        return JSON.stringify({ ok: true, result: result });
    } catch (e) {
        return JSON.stringify({ ok: false, error: e.message });
    }
}
`

// jsSetConversationMute 免打扰/取消免打扰。
// 参数: [{ conversation_id: "会话 shortId", muted: true/false }]
const jsSetConversationMute = `
async (args) => {
    try {
        var inst = window.__sdkInst;
        var cm = window.__imCtx && window.__imCtx.imSdkService.conversationManager;
        if (!inst || !cm) return JSON.stringify({ ok: false, error: 'SDK 未初始化' });
        var convId = String(args[0].conversation_id);
        var muted = !!args[0].muted;
        var conv = cm.getConversationById ? cm.getConversationById(convId) : null;
        if (!conv) return JSON.stringify({ ok: false, error: '会话不存在: ' + convId });
        var result = await inst.setConversationSettingInfo({ conversation: conv, settingType: 'mute', value: muted });
        return JSON.stringify({ ok: true, result: result });
    } catch (e) {
        return JSON.stringify({ ok: false, error: e.message });
    }
}
`

// jsDeleteConversation 删除会话。
// 参数: [{ conversation_id: "会话 shortId" }]
const jsDeleteConversation = `
async (args) => {
    try {
        var inst = window.__sdkInst;
        var cm = window.__imCtx && window.__imCtx.imSdkService.conversationManager;
        if (!inst || !cm) return JSON.stringify({ ok: false, error: 'SDK 未初始化' });
        var convId = String(args[0].conversation_id);
        var conv = cm.getConversationById ? cm.getConversationById(convId) : null;
        if (!conv) return JSON.stringify({ ok: false, error: '会话不存在: ' + convId });
        var result = await inst.deleteConversation({ conversation: conv });
        return JSON.stringify({ ok: true, result: result });
    } catch (e) {
        return JSON.stringify({ ok: false, error: e.message });
    }
}
`

// jsMarkConversationRead 标记会话已读。
// 参数: [{ conversation_id: "会话 shortId" }]
const jsMarkConversationRead = `
async (args) => {
    try {
        var inst = window.__sdkInst;
        var cm = window.__imCtx && window.__imCtx.imSdkService.conversationManager;
        if (!inst || !cm) return JSON.stringify({ ok: false, error: 'SDK 未初始化' });
        var convId = String(args[0].conversation_id);
        var conv = cm.getConversationById ? cm.getConversationById(convId) : null;
        if (!conv) return JSON.stringify({ ok: false, error: '会话不存在: ' + convId });
        var result = await inst.markConversationRead({ conversation: conv });
        return JSON.stringify({ ok: true, result: result });
    } catch (e) {
        return JSON.stringify({ ok: false, error: e.message });
    }
}
`

// jsLeaveConversation 退出群聊。
// 参数: [{ conversation_id: "群会话 shortId" }]
const jsLeaveConversation = `
async (args) => {
    try {
        var inst = window.__sdkInst;
        var cm = window.__imCtx && window.__imCtx.imSdkService.conversationManager;
        if (!inst || !cm) return JSON.stringify({ ok: false, error: 'SDK 未初始化' });
        var convId = String(args[0].conversation_id);
        var conv = cm.getConversationById ? cm.getConversationById(convId) : null;
        if (!conv) return JSON.stringify({ ok: false, error: '群会话不存在: ' + convId });
        var result = await inst.leaveConversation({ conversation: conv });
        return JSON.stringify({ ok: true, result: result });
    } catch (e) {
        return JSON.stringify({ ok: false, error: e.message });
    }
}
`

// jsDissolveConversation 解散群聊（仅群主）。
// 参数: [{ conversation_id: "群会话 shortId" }]
const jsDissolveConversation = `
async (args) => {
    try {
        var inst = window.__sdkInst;
        var cm = window.__imCtx && window.__imCtx.imSdkService.conversationManager;
        if (!inst || !cm) return JSON.stringify({ ok: false, error: 'SDK 未初始化' });
        var convId = String(args[0].conversation_id);
        var conv = cm.getConversationById ? cm.getConversationById(convId) : null;
        if (!conv) return JSON.stringify({ ok: false, error: '群会话不存在: ' + convId });
        var result = await inst.dissolveConversation({ conversation: conv });
        return JSON.stringify({ ok: true, result: result });
    } catch (e) {
        return JSON.stringify({ ok: false, error: e.message });
    }
}
`

// jsGetConversationParticipants 获取群成员列表。
// 参数: [{ conversation_id: "群会话 shortId" }]
const jsGetConversationParticipants = `
async (args) => {
    try {
        var inst = window.__sdkInst;
        var cm = window.__imCtx && window.__imCtx.imSdkService.conversationManager;
        if (!inst || !cm) return JSON.stringify({ ok: false, error: 'SDK 未初始化' });
        var convId = String(args[0].conversation_id);
        var conv = cm.getConversationById ? cm.getConversationById(convId) : null;
        if (!conv) return JSON.stringify({ ok: false, error: '群会话不存在: ' + convId });
        var participants = await inst.getConversationParticipants({ conversation: conv });
        var items = [];
        if (participants && participants.length) {
            for (var i = 0; i < participants.length; i++) {
                var p = participants[i];
                items.push({
                    uid: String(p.uid || ''),
                    sec_uid: String(p.sec_uid || p.secUid || ''),
                    nickname: String(p.nickname || ''),
                    role: Number(p.role || 0),
                });
            }
        }
        return JSON.stringify({ ok: true, items: items, total: items.length });
    } catch (e) {
        return JSON.stringify({ ok: false, error: e.message });
    }
}
`

// jsAddParticipants 添加群成员。
// 参数: [{ conversation_id: "群会话 shortId", sec_uids: ["sec_uid1", "sec_uid2"] }]
const jsAddParticipants = `
async (args) => {
    try {
        var inst = window.__sdkInst;
        var cm = window.__imCtx && window.__imCtx.imSdkService.conversationManager;
        if (!inst || !cm) return JSON.stringify({ ok: false, error: 'SDK 未初始化' });
        var convId = String(args[0].conversation_id);
        var secUids = args[0].sec_uids || [];
        var conv = cm.getConversationById ? cm.getConversationById(convId) : null;
        if (!conv) return JSON.stringify({ ok: false, error: '群会话不存在: ' + convId });
        var result = await inst.addParticipants({ conversation: conv, participants: secUids });
        return JSON.stringify({ ok: true, result: result });
    } catch (e) {
        return JSON.stringify({ ok: false, error: e.message });
    }
}
`

// jsRemoveParticipants 移除群成员。
// 参数: [{ conversation_id: "群会话 shortId", sec_uids: ["sec_uid1", "sec_uid2"] }]
const jsRemoveParticipants = `
async (args) => {
    try {
        var inst = window.__sdkInst;
        var cm = window.__imCtx && window.__imCtx.imSdkService.conversationManager;
        if (!inst || !cm) return JSON.stringify({ ok: false, error: 'SDK 未初始化' });
        var convId = String(args[0].conversation_id);
        var secUids = args[0].sec_uids || [];
        var conv = cm.getConversationById ? cm.getConversationById(convId) : null;
        if (!conv) return JSON.stringify({ ok: false, error: '群会话不存在: ' + convId });
        var result = await inst.removeParticipants({ conversation: conv, participants: secUids });
        return JSON.stringify({ ok: true, result: result });
    } catch (e) {
        return JSON.stringify({ ok: false, error: e.message });
    }
}
`

// jsCreateConversation 创建会话（私聊或群聊）。
// 参数: [{ participants: ["sec_uid1", ...], type: 1=私聊/2=群聊, name: "群名(仅群聊)" }]
const jsCreateConversation = `
async (args) => {
    try {
        var inst = window.__sdkInst;
        if (!inst) return JSON.stringify({ ok: false, error: 'SDK 未初始化' });
        var participants = args[0].participants || [];
        var convType = Number(args[0].type) || 1;
        var name = String(args[0].name || '');
        if (!participants.length) return JSON.stringify({ ok: false, error: 'participants 为空' });
        var createArg = { participants: participants, type: convType };
        if (name && convType === 2) createArg.name = name;
        var result = await inst.createConversation(createArg);
        return JSON.stringify({ ok: result && result.success, payload: result ? result.payload : null });
    } catch (e) {
        return JSON.stringify({ ok: false, error: e.message });
    }
}
`

// jsRegisterReceiver 注册新消息推送回调，通过 fetch POST 到 Go 内部端点。
// 双通道：(1) SDK newMessagePushManager 回调 (2) 会话列表轮询兜底。
// 新消息存入 window.__obNewMsgs[]，由 Go 侧 page.Evaluate 轮询拉取（绕过混合内容限制）。
const jsRegisterReceiver = `
() => {
    try {
        if (window.__recvRegistered) return JSON.stringify({ ok: true, already: true });
        var ctx = window.__imCtx;
        if (!ctx) return JSON.stringify({ ok: false, error: 'imCtx 不可用' });
        var sdk = ctx.imSdkService;
        if (!sdk) return JSON.stringify({ ok: false, error: 'imSdkService 不可用' });

        window.__obNewMsgs = [];
        window.__obSeenMsgs = {};

        function __obDispatch(msg) {
            try {
                var parsedContent = null;
                var text = '';
                var msgType = 'text';
                try {
                    parsedContent = typeof msg.content === 'string' ? JSON.parse(msg.content) : msg.content;
                } catch (e) {}
                if (parsedContent) {
                    var aweType = parsedContent.aweType || 0;
                    if (aweType === 700 || parsedContent.text !== undefined) {
                        text = parsedContent.text || '';
                        if (text.indexOf('\\n') === 0) text = text.substring(1);
                        msgType = 'text';
                    } else if (aweType === 800 || parsedContent.image_id !== undefined) {
                        msgType = 'sticker';
                    }
                }
                var senderNickname = '';
                try {
                    var userStore = sdk.userCacheManager;
                    if (userStore && userStore.getUserInfo) {
                        var userInfo = userStore.getUserInfo(msg.sender);
                        if (userInfo) senderNickname = userInfo.nickname || '';
                    }
                } catch (e) {}
                var conversationType = 0;
                try {
                    var cm = sdk.conversationManager;
                    var convMap = cm && cm.conversationMap;
                    if (convMap && convMap.get) {
                        var conv = convMap.get(msg.conversationId || msg.conversationShortId);
                        if (conv) conversationType = Number(conv.type || 0);
                    }
                } catch (e) {}
                var payload = {
                    conversation_short_id: String(msg.conversationShortId || ''),
                    conversation_id: String(msg.conversationId || ''),
                    conversation_type: conversationType,
                    sender: String(msg.sender || ''),
                    sender_nickname: senderNickname,
                    type: Number(msg.type || 0),
                    msg_type: msgType,
                    text: text,
                    content: typeof msg.content === 'string' ? msg.content : JSON.stringify(parsedContent || {}),
                    client_id: String(msg.clientId || ''),
                    server_id: String(msg.serverId || ''),
                    is_from_me: !!msg.isFromMe,
                    created_at: Math.floor((Number(msg.createdAt) || Date.now()) / 1000)
                };
                window.__obNewMsgs.push(payload);
            } catch (e) {}
        }

        var npm = sdk.newMessagePushManager;
        if (npm && npm.registerNewMessagePush) {
            npm.registerNewMessagePush(__obDispatch);
        }

        window.__obPollInterval = setInterval(function() {
            try {
                var store = ctx.store;
                if (!store || !store.conversationStore) return;
                var cm = store.conversationStore.conversationMap;
                if (!cm || !cm.forEach) return;
                cm.forEach(function(conv, convId) {
                    try {
                        if (!conv || !conv.lastMessage) return;
                        var m = conv.lastMessage;
                        var key = (m.serverId || m.clientId || convId + '_' + m.createdAt);
                        if (window.__obSeenMsgs[key]) return;
                        window.__obSeenMsgs[key] = true;
                        __obDispatch(m);
                    } catch (e) {}
                });
            } catch (e) {}
        }, 2000);

        window.__recvRegistered = true;
        return JSON.stringify({ ok: true, npm: !!npm, method: 'store+poll' });
    } catch (e) {
        return JSON.stringify({ ok: false, error: e.message });
    }
}
`

// jsDrainNewMsgs 从 window.__obNewMsgs 拉取并清空待发消息队列（Go 侧轮询调用）。
const jsDrainNewMsgs = `(function(){try{var q=window.__obNewMsgs;if(!q||!q.length)return JSON.stringify({ok:true,msgs:[]});var msgs=q.splice(0,q.length);return JSON.stringify({ok:true,msgs:msgs});}catch(e){return JSON.stringify({ok:false,error:e.message});}})()`

// jsGetMessagesByUser 按用户获取消息列表。
// 参数: [{ conversation_id: "会话 shortId" }]
const jsGetMessagesByUser = `
async (args) => {
    try {
        var inst = window.__sdkInst;
        var cm = window.__imCtx && window.__imCtx.imSdkService.conversationManager;
        if (!inst || !cm) return JSON.stringify({ ok: false, error: 'SDK 未初始化' });
        var convId = String(args[0].conversation_id);
        var conv = cm.getConversationById ? cm.getConversationById(convId) : null;
        if (!conv) return JSON.stringify({ ok: false, error: '会话不存在: ' + convId });
        var msgs = await inst.getMessagesByUser({ conversation: conv });
        var items = [];
        if (msgs && msgs.length) {
            for (var i = 0; i < msgs.length; i++) {
                var m = msgs[i];
                items.push({
                    server_id: String(m.serverId || ''),
                    client_id: String(m.clientId || ''),
                    sender: String(m.sender || ''),
                    type: Number(m.type || 0),
                    text: '',
                    content: typeof m.content === 'string' ? m.content : '',
                    created_at: Math.floor((Number(m.createdAt) || 0) / 1000),
                    is_from_me: !!m.isFromMe,
                });
                try {
                    var parsed = typeof items[i].content === 'string' ? JSON.parse(items[i].content) : {};
                    if (parsed.text) items[i].text = parsed.text;
                } catch (e) {}
            }
        }
        return JSON.stringify({ ok: true, items: items, total: items.length });
    } catch (e) {
        return JSON.stringify({ ok: false, error: e.message });
    }
}
`

// jsGetMessagesByConversation 按会话获取消息列表。
// 参数: [{ conversation_id: "会话 shortId" }]
const jsGetMessagesByConversation = `
async (args) => {
    try {
        var inst = window.__sdkInst;
        var cm = window.__imCtx && window.__imCtx.imSdkService.conversationManager;
        if (!inst || !cm) return JSON.stringify({ ok: false, error: 'SDK 未初始化' });
        var convId = String(args[0].conversation_id);
        var conv = cm.getConversationById ? cm.getConversationById(convId) : null;
        if (!conv) return JSON.stringify({ ok: false, error: '会话不存在: ' + convId });
        var msgs = await inst.getMessagesByConversation({ conversation: conv });
        var items = [];
        if (msgs && msgs.length) {
            for (var i = 0; i < msgs.length; i++) {
                var m = msgs[i];
                items.push({
                    server_id: String(m.serverId || ''),
                    client_id: String(m.clientId || ''),
                    sender: String(m.sender || ''),
                    type: Number(m.type || 0),
                    text: '',
                    content: typeof m.content === 'string' ? m.content : '',
                    created_at: Math.floor((Number(m.createdAt) || 0) / 1000),
                    is_from_me: !!m.isFromMe,
                });
                try {
                    var parsed = typeof items[i].content === 'string' ? JSON.parse(items[i].content) : {};
                    if (parsed.text) items[i].text = parsed.text;
                } catch (e) {}
            }
        }
        return JSON.stringify({ ok: true, items: items, total: items.length });
    } catch (e) {
        return JSON.stringify({ ok: false, error: e.message });
    }
}
`

// jsFetchConversation 从服务器拉取会话状态。
// 参数: [{ conversation_id: "会话 shortId" }]
const jsFetchConversation = `
async (args) => {
    try {
        var inst = window.__sdkInst;
        var cm = window.__imCtx && window.__imCtx.imSdkService.conversationManager;
        if (!inst || !cm) return JSON.stringify({ ok: false, error: 'SDK 未初始化' });
        var convId = String(args[0].conversation_id);
        var conv = cm.getConversationById ? cm.getConversationById(convId) : null;
        if (!conv) return JSON.stringify({ ok: false, error: '会话不存在: ' + convId });
        var result = await inst.fetchConversation({ conversation: conv });
        return JSON.stringify({ ok: true, result: result });
    } catch (e) {
        return JSON.stringify({ ok: false, error: e.message });
    }
}
`

// jsUpdateConversationReadReceipt 更新已读回执。
// 参数: [{ conversation_id: "会话 shortId" }]
const jsUpdateConversationReadReceipt = `
async (args) => {
    try {
        var inst = window.__sdkInst;
        var cm = window.__imCtx && window.__imCtx.imSdkService.conversationManager;
        if (!inst || !cm) return JSON.stringify({ ok: false, error: 'SDK 未初始化' });
        var convId = String(args[0].conversation_id);
        var conv = cm.getConversationById ? cm.getConversationById(convId) : null;
        if (!conv) return JSON.stringify({ ok: false, error: '会话不存在: ' + convId });
        var result = await inst.updateConversationReadReceipt({ conversation: conv });
        return JSON.stringify({ ok: true, result: result });
    } catch (e) {
        return JSON.stringify({ ok: false, error: e.message });
    }
}
`

// jsGetMessageReadReceipt 获取消息已读回执。
// 参数: [{ conversation_id: "会话 shortId", server_id: "消息 serverId" }]
const jsGetMessageReadReceipt = `
(args) => {
    try {
        var inst = window.__sdkInst;
        var cm = window.__imCtx && window.__imCtx.imSdkService.conversationManager;
        if (!inst || !cm) return JSON.stringify({ ok: false, error: 'SDK 未初始化' });
        var convId = String(args[0].conversation_id);
        var serverId = String(args[0].server_id);
        var conv = cm.getConversationById ? cm.getConversationById(convId) : null;
        if (!conv) return JSON.stringify({ ok: false, error: '会话不存在: ' + convId });
        var msgs = inst.getConversationMessages({ conversation: conv });
        var targetMsg = null;
        for (var i = msgs.length - 1; i >= 0; i--) {
            if (String(msgs[i].serverId) === serverId) { targetMsg = msgs[i]; break; }
        }
        if (!targetMsg) return JSON.stringify({ ok: false, error: '消息不存在: ' + serverId });
        var result = inst.getMessageReadReceipt({ message: targetMsg });
        return JSON.stringify({ ok: true, receipt: result });
    } catch (e) {
        return JSON.stringify({ ok: false, error: e.message });
    }
}
`

// jsGetParticipantsReadAndMinIndex 获取群成员已读状态和最小索引。
// 参数: [{ conversation_id: "群会话 shortId" }]
const jsGetParticipantsReadAndMinIndex = `
async (args) => {
    try {
        var inst = window.__sdkInst;
        var cm = window.__imCtx && window.__imCtx.imSdkService.conversationManager;
        if (!inst || !cm) return JSON.stringify({ ok: false, error: 'SDK 未初始化' });
        var convId = String(args[0].conversation_id);
        var conv = cm.getConversationById ? cm.getConversationById(convId) : null;
        if (!conv) return JSON.stringify({ ok: false, error: '群会话不存在: ' + convId });
        var result = await inst.getParticipantsReadAndMinIndex({ conversation: conv });
        return JSON.stringify({ ok: true, result: result });
    } catch (e) {
        return JSON.stringify({ ok: false, error: e.message });
    }
}
`

// jsGetConversationParticipantsAsync 异步获取群成员列表。
// 参数: [{ conversation_id: "群会话 shortId" }]
const jsGetConversationParticipantsAsync = `
async (args) => {
    try {
        var inst = window.__sdkInst;
        var cm = window.__imCtx && window.__imCtx.imSdkService.conversationManager;
        if (!inst || !cm) return JSON.stringify({ ok: false, error: 'SDK 未初始化' });
        var convId = String(args[0].conversation_id);
        var conv = cm.getConversationById ? cm.getConversationById(convId) : null;
        if (!conv) return JSON.stringify({ ok: false, error: '群会话不存在: ' + convId });
        var participants = await inst.getConversationParticipantsAsync({ conversation: conv });
        var items = [];
        if (participants && participants.length) {
            for (var i = 0; i < participants.length; i++) {
                var p = participants[i];
                items.push({
                    uid: String(p.uid || ''),
                    sec_uid: String(p.sec_uid || p.secUid || ''),
                    nickname: String(p.nickname || ''),
                    role: Number(p.role || 0),
                });
            }
        }
        return JSON.stringify({ ok: true, items: items, total: items.length });
    } catch (e) {
        return JSON.stringify({ ok: false, error: e.message });
    }
}
`

// jsGetConversationParticipantsByPage 分页获取群成员。
// 参数: [{ conversation_id: "群会话 shortId", page: 1, page_size: 50 }]
const jsGetConversationParticipantsByPage = `
async (args) => {
    try {
        var inst = window.__sdkInst;
        var cm = window.__imCtx && window.__imCtx.imSdkService.conversationManager;
        if (!inst || !cm) return JSON.stringify({ ok: false, error: 'SDK 未初始化' });
        var convId = String(args[0].conversation_id);
        var page = Number(args[0].page) || 1;
        var pageSize = Number(args[0].page_size) || 50;
        var conv = cm.getConversationById ? cm.getConversationById(convId) : null;
        if (!conv) return JSON.stringify({ ok: false, error: '群会话不存在: ' + convId });
        var result = await inst.getConversationParticipantsByPage({ conversation: conv, page: page, pageSize: pageSize });
        return JSON.stringify({ ok: true, result: result });
    } catch (e) {
        return JSON.stringify({ ok: false, error: e.message });
    }
}
`

// jsApplyJoinGroup 申请加入群聊。
// 参数: [{ group_short_id: "群 shortId" }]
const jsApplyJoinGroup = `
async (args) => {
    try {
        var inst = window.__sdkInst;
        if (!inst) return JSON.stringify({ ok: false, error: 'SDK 未初始化' });
        var gid = String(args[0].group_short_id);
        var result = await inst.applyJoinGroup({ groupShortId: gid });
        return JSON.stringify({ ok: true, result: result });
    } catch (e) {
        return JSON.stringify({ ok: false, error: e.message });
    }
}
`

// jsUpsertConversationSettingExtInfo 更新会话扩展设置。
// 参数: [{ conversation_id: "会话 shortId", ext_info: {...} }]
const jsUpsertConversationSettingExtInfo = `
async (args) => {
    try {
        var inst = window.__sdkInst;
        var cm = window.__imCtx && window.__imCtx.imSdkService.conversationManager;
        if (!inst || !cm) return JSON.stringify({ ok: false, error: 'SDK 未初始化' });
        var convId = String(args[0].conversation_id);
        var extInfo = args[0].ext_info || {};
        var conv = cm.getConversationById ? cm.getConversationById(convId) : null;
        if (!conv) return JSON.stringify({ ok: false, error: '会话不存在: ' + convId });
        var result = await inst.upsertConversationSettingExtInfo({ conversation: conv, extInfo: extInfo });
        return JSON.stringify({ ok: true, result: result });
    } catch (e) {
        return JSON.stringify({ ok: false, error: e.message });
    }
}
`

// jsGetConversationBots 获取会话机器人列表。
// 参数: [{ conversation_id: "会话 shortId" }]
const jsGetConversationBots = `
async (args) => {
    try {
        var inst = window.__sdkInst;
        var cm = window.__imCtx && window.__imCtx.imSdkService.conversationManager;
        if (!inst || !cm) return JSON.stringify({ ok: false, error: 'SDK 未初始化' });
        var convId = String(args[0].conversation_id);
        var conv = cm.getConversationById ? cm.getConversationById(convId) : null;
        if (!conv) return JSON.stringify({ ok: false, error: '会话不存在: ' + convId });
        var bots = await inst.getConversationBots({ conversation: conv });
        return JSON.stringify({ ok: true, bots: bots });
    } catch (e) {
        return JSON.stringify({ ok: false, error: e.message });
    }
}
`

// jsGetStrangerConversationMessage 获取陌生人会话消息。
// 参数: [{ conversation_id: "陌生人会话 shortId" }]
const jsGetStrangerConversationMessage = `
async (args) => {
    try {
        var inst = window.__sdkInst;
        var cm = window.__imCtx && window.__imCtx.imSdkService.conversationManager;
        if (!inst || !cm) return JSON.stringify({ ok: false, error: 'SDK 未初始化' });
        var convId = String(args[0].conversation_id);
        var conv = cm.getConversationById ? cm.getConversationById(convId) : null;
        if (!conv) return JSON.stringify({ ok: false, error: '陌生人会话不存在: ' + convId });
        var msgs = await inst.getStrangerConversationMessage({ conversation: conv });
        var items = [];
        if (msgs && msgs.length) {
            for (var i = 0; i < msgs.length; i++) {
                var m = msgs[i];
                items.push({
                    server_id: String(m.serverId || ''),
                    client_id: String(m.clientId || ''),
                    sender: String(m.sender || ''),
                    type: Number(m.type || 0),
                    text: '',
                    content: typeof m.content === 'string' ? m.content : '',
                    created_at: Math.floor((Number(m.createdAt) || 0) / 1000),
                });
                try {
                    var parsed = typeof items[i].content === 'string' ? JSON.parse(items[i].content) : {};
                    if (parsed.text) items[i].text = parsed.text;
                } catch (e) {}
            }
        }
        return JSON.stringify({ ok: true, items: items, total: items.length });
    } catch (e) {
        return JSON.stringify({ ok: false, error: e.message });
    }
}
`

// jsDeleteStrangerConversation 删除陌生人会话。
// 参数: [{ conversation_id: "陌生人会话 shortId" }]
const jsDeleteStrangerConversation = `
async (args) => {
    try {
        var inst = window.__sdkInst;
        var cm = window.__imCtx && window.__imCtx.imSdkService.conversationManager;
        if (!inst || !cm) return JSON.stringify({ ok: false, error: 'SDK 未初始化' });
        var convId = String(args[0].conversation_id);
        var conv = cm.getConversationById ? cm.getConversationById(convId) : null;
        if (!conv) return JSON.stringify({ ok: false, error: '陌生人会话不存在: ' + convId });
        var result = await inst.deleteStrangerConversation({ conversation: conv });
        return JSON.stringify({ ok: true, result: result });
    } catch (e) {
        return JSON.stringify({ ok: false, error: e.message });
    }
}
`

// jsDeleteAllStrangerConversation 删除所有陌生人会话。
const jsDeleteAllStrangerConversation = `
async () => {
    try {
        var inst = window.__sdkInst;
        if (!inst) return JSON.stringify({ ok: false, error: 'SDK 未初始化' });
        var result = await inst.deleteAllStrangerConversation();
        return JSON.stringify({ ok: true, result: result });
    } catch (e) {
        return JSON.stringify({ ok: false, error: e.message });
    }
}
`

// jsMarkStrangerConversationRead 标记陌生人会话已读。
// 参数: [{ conversation_id: "陌生人会话 shortId" }]
const jsMarkStrangerConversationRead = `
async (args) => {
    try {
        var inst = window.__sdkInst;
        var cm = window.__imCtx && window.__imCtx.imSdkService.conversationManager;
        if (!inst || !cm) return JSON.stringify({ ok: false, error: 'SDK 未初始化' });
        var convId = String(args[0].conversation_id);
        var conv = cm.getConversationById ? cm.getConversationById(convId) : null;
        if (!conv) return JSON.stringify({ ok: false, error: '陌生人会话不存在: ' + convId });
        var result = await inst.markStrangerConversationRead({ conversation: conv });
        return JSON.stringify({ ok: true, result: result });
    } catch (e) {
        return JSON.stringify({ ok: false, error: e.message });
    }
}
`

// jsMarkAllStrangerConversationRead 标记所有陌生人会话已读。
const jsMarkAllStrangerConversationRead = `
async () => {
    try {
        var inst = window.__sdkInst;
        if (!inst) return JSON.stringify({ ok: false, error: 'SDK 未初始化' });
        var result = await inst.markAllStrangerConversationRead();
        return JSON.stringify({ ok: true, result: result });
    } catch (e) {
        return JSON.stringify({ ok: false, error: e.message });
    }
}
`

// jsGetStrangerPreview 获取陌生人预览。
const jsGetStrangerPreview = `
() => {
    try {
        var inst = window.__sdkInst;
        if (!inst) return JSON.stringify({ ok: false, error: 'SDK 未初始化' });
        var result = inst.getStrangerPreview();
        return JSON.stringify({ ok: true, result: result });
    } catch (e) {
        return JSON.stringify({ ok: false, error: e.message });
    }
}
`

// jsBatchClearConversationRead 批量清除会话已读。
// 参数: [{ conversation_ids: ["id1", "id2"] }]
const jsBatchClearConversationRead = `
async (args) => {
    try {
        var inst = window.__sdkInst;
        var cm = window.__imCtx && window.__imCtx.imSdkService.conversationManager;
        if (!inst || !cm) return JSON.stringify({ ok: false, error: 'SDK 未初始化' });
        var convIds = args[0].conversation_ids || [];
        var convs = [];
        for (var i = 0; i < convIds.length; i++) {
            var conv = cm.getConversationById ? cm.getConversationById(convIds[i]) : null;
            if (conv) convs.push(conv);
        }
        var result = await inst.batchClearConversationRead({ conversations: convs });
        return JSON.stringify({ ok: true, result: result });
    } catch (e) {
        return JSON.stringify({ ok: false, error: e.message });
    }
}
`

// jsClearConversationMessages 清空会话消息。
// 参数: [{ conversation_id: "会话 shortId" }]
const jsClearConversationMessages = `
async (args) => {
    try {
        var inst = window.__sdkInst;
        var cm = window.__imCtx && window.__imCtx.imSdkService.conversationManager;
        if (!inst || !cm) return JSON.stringify({ ok: false, error: 'SDK 未初始化' });
        var convId = String(args[0].conversation_id);
        var conv = cm.getConversationById ? cm.getConversationById(convId) : null;
        if (!conv) return JSON.stringify({ ok: false, error: '会话不存在: ' + convId });
        var result = await inst.clearConversationMessages({ conversation: conv });
        return JSON.stringify({ ok: true, result: result });
    } catch (e) {
        return JSON.stringify({ ok: false, error: e.message });
    }
}
`

// jsAddOrUpdateLocalExts 添加/更新本地扩展。
// 参数: [{ conversation_id: "会话 shortId", exts: {key: value} }]
const jsAddOrUpdateLocalExts = `
async (args) => {
    try {
        var inst = window.__sdkInst;
        var cm = window.__imCtx && window.__imCtx.imSdkService.conversationManager;
        if (!inst || !cm) return JSON.stringify({ ok: false, error: 'SDK 未初始化' });
        var convId = String(args[0].conversation_id);
        var exts = args[0].exts || {};
        var conv = cm.getConversationById ? cm.getConversationById(convId) : null;
        if (!conv) return JSON.stringify({ ok: false, error: '会话不存在: ' + convId });
        var result = await inst.addOrUpdateLocalExts({ conversation: conv, exts: exts });
        return JSON.stringify({ ok: true, result: result });
    } catch (e) {
        return JSON.stringify({ ok: false, error: e.message });
    }
}
`

// jsDeleteLocalExts 删除本地扩展。
// 参数: [{ conversation_id: "会话 shortId", keys: ["key1", "key2"] }]
const jsDeleteLocalExts = `
async (args) => {
    try {
        var inst = window.__sdkInst;
        var cm = window.__imCtx && window.__imCtx.imSdkService.conversationManager;
        if (!inst || !cm) return JSON.stringify({ ok: false, error: 'SDK 未初始化' });
        var convId = String(args[0].conversation_id);
        var keys = args[0].keys || [];
        var conv = cm.getConversationById ? cm.getConversationById(convId) : null;
        if (!conv) return JSON.stringify({ ok: false, error: '会话不存在: ' + convId });
        var result = await inst.deleteLocalExts({ conversation: conv, keys: keys });
        return JSON.stringify({ ok: true, result: result });
    } catch (e) {
        return JSON.stringify({ ok: false, error: e.message });
    }
}
`

// jsReportMessageDelayTime 上报消息延迟。
// 参数: [{ server_id: "消息 serverId", log_id: "日志ID" }]
const jsReportMessageDelayTime = `
async (args) => {
    try {
        var inst = window.__sdkInst;
        if (!inst) return JSON.stringify({ ok: false, error: 'SDK 未初始化' });
        var serverId = String(args[0].server_id);
        var logId = String(args[0].log_id || '');
        var result = await inst.reportMessageDelayTime({ messageId: serverId, logId: logId });
        return JSON.stringify({ ok: true, result: result });
    } catch (e) {
        return JSON.stringify({ ok: false, error: e.message });
    }
}
`

// jsDbClear 清空本地数据库。
const jsDbClear = `
async () => {
    try {
        var inst = window.__sdkInst;
        if (!inst) return JSON.stringify({ ok: false, error: 'SDK 未初始化' });
        var result = await inst.dbClear();
        return JSON.stringify({ ok: true, result: result });
    } catch (e) {
        return JSON.stringify({ ok: false, error: e.message });
    }
}
`

// ==================== B 类: Store 未公开方法 ====================

// jsSearchConversations 搜索会话。
// 参数: [{ keyword: "搜索关键词" }]
const jsSearchConversations = `
async (args) => {
    try {
        var ctx = window.__imCtx;
        if (!ctx || !ctx.store || !ctx.store.searchStore) return JSON.stringify({ ok: false, error: '搜索 Store 不可用' });
        var ss = ctx.store.searchStore.conversationSearchStore;
        if (!ss || !ss.doSearch) return JSON.stringify({ ok: false, error: '会话搜索方法不可用' });
        var keyword = String(args[0].keyword);
        ss.setSearchKey(keyword);
        ss.setInputValue(keyword);
        var result = await ss.doSearch();
        return JSON.stringify({ ok: true, result: result });
    } catch (e) {
        return JSON.stringify({ ok: false, error: e.message });
    }
}
`

// jsSearchParticipants 搜索群成员。
// 参数: [{ conversation_id: "群会话 shortId", keyword: "搜索关键词" }]
const jsSearchParticipants = `
async (args) => {
    try {
        var ctx = window.__imCtx;
        if (!ctx || !ctx.store || !ctx.store.searchStore) return JSON.stringify({ ok: false, error: '搜索 Store 不可用' });
        var ps = ctx.store.searchStore.participantSearchStore;
        if (!ps || !ps.doSearch) return JSON.stringify({ ok: false, error: '成员搜索方法不可用' });
        var convId = String(args[0].conversation_id);
        var keyword = String(args[0].keyword);
        ps._conversationId = convId;
        ps.setSearchKey(keyword);
        ps.setInputValue(keyword);
        var result = await ps.doSearch();
        return JSON.stringify({ ok: true, result: result });
    } catch (e) {
        return JSON.stringify({ ok: false, error: e.message });
    }
}
`

// jsRequestRelationsData 请求好友关系数据。
const jsRequestRelationsData = `
async () => {
    try {
        var ctx = window.__imCtx;
        if (!ctx || !ctx.store) return JSON.stringify({ ok: false, error: 'Store 不可用' });
        var rs = ctx.store.relationStore;
        if (!rs || !rs.requestRelationsData) return JSON.stringify({ ok: false, error: '关系请求方法不可用' });
        var result = await rs.requestRelationsData();
        return JSON.stringify({ ok: true, result: result });
    } catch (e) {
        return JSON.stringify({ ok: false, error: e.message });
    }
}
`

// jsGenLocalUsers 生成本地用户列表。
const jsGenLocalUsers = `
() => {
    try {
        var ctx = window.__imCtx;
        if (!ctx || !ctx.store) return JSON.stringify({ ok: false, error: 'Store 不可用' });
        var rs = ctx.store.relationStore;
        if (!rs || !rs.genLocalUsers) return JSON.stringify({ ok: false, error: '本地用户生成方法不可用' });
        var result = rs.genLocalUsers();
        return JSON.stringify({ ok: true, result: result });
    } catch (e) {
        return JSON.stringify({ ok: false, error: e.message });
    }
}
`

// jsLoadMessages 加载消息列表。
// 参数: [{ conversation_id: "会话 shortId" }]
const jsLoadMessages = `
async (args) => {
    try {
        var ctx = window.__imCtx;
        if (!ctx || !ctx.store) return JSON.stringify({ ok: false, error: 'Store 不可用' });
        var mls = ctx.store.curMessageListStore;
        if (!mls || !mls.loadMessage) return JSON.stringify({ ok: false, error: '消息加载方法不可用' });
        var convId = String(args[0].conversation_id);
        var result = await mls.loadMessage(convId);
        return JSON.stringify({ ok: true, result: result });
    } catch (e) {
        return JSON.stringify({ ok: false, error: e.message });
    }
}
`

// jsGetMessageByServerId 按 serverId 或 clientId 获取消息（遍历所有会话消息列表）。
// 参数: [{ server_id: "消息 serverId 或 clientId" }]
const jsGetMessageByServerId = `
(args) => {
    try {
        var ctx = window.__imCtx;
        if (!ctx || !ctx.store) return JSON.stringify({ ok: false, error: 'Store 不可用' });
        var mls = ctx.store.curMessageListStore;
        if (!mls) return JSON.stringify({ ok: false, error: '消息 Store 不可用' });
        var searchId = String(args[0].server_id);
        function matchMsg(m) {
            return String(m.serverId || '') === searchId || String(m.clientId || '') === searchId;
        }
        var msgs = mls.messageList;
        if (msgs) {
            for (var i = 0; i < msgs.length; i++) {
                if (matchMsg(msgs[i])) return JSON.stringify({ ok: true, result: msgs[i] });
            }
        }
        var allMsgs = mls.allMessageList;
        if (allMsgs) {
            for (var k = 0; k < allMsgs.length; k++) {
                var list = allMsgs[k];
                if (list && list.messages) {
                    for (var j = 0; j < list.messages.length; j++) {
                        if (matchMsg(list.messages[j])) return JSON.stringify({ ok: true, result: list.messages[j] });
                    }
                }
            }
        }
        return JSON.stringify({ ok: false, error: '消息不存在: ' + searchId });
    } catch (e) {
        return JSON.stringify({ ok: false, error: e.message });
    }
}
`

// jsGetAllConversation 获取所有会话（conversationListManager）。
const jsGetAllConversation = `
async () => {
    try {
        var ctx = window.__imCtx;
        if (!ctx || !ctx.imSdkService) return JSON.stringify({ ok: false, error: 'imSdkService 不可用' });
        var clm = ctx.imSdkService.conversationListManager;
        if (!clm || !clm.getAllConversation) return JSON.stringify({ ok: false, error: 'getAllConversation 方法不可用' });
        var result = await clm.getAllConversation();
        return JSON.stringify({ ok: true, result: result });
    } catch (e) {
        return JSON.stringify({ ok: false, error: e.message });
    }
}
`

// jsGetAllGroupConversation 获取所有群聊（conversationListManager）。
const jsGetAllGroupConversation = `
async () => {
    try {
        var ctx = window.__imCtx;
        if (!ctx || !ctx.imSdkService) return JSON.stringify({ ok: false, error: 'imSdkService 不可用' });
        var clm = ctx.imSdkService.conversationListManager;
        if (!clm || !clm.getAllGroupConversation) return JSON.stringify({ ok: false, error: 'getAllGroupConversation 方法不可用' });
        var result = await clm.getAllGroupConversation();
        return JSON.stringify({ ok: true, result: result });
    } catch (e) {
        return JSON.stringify({ ok: false, error: e.message });
    }
}
`

// jsLoadMoreConversations 加载更多会话（conversationListManager）。
const jsLoadMoreConversations = `
async () => {
    try {
        var ctx = window.__imCtx;
        if (!ctx || !ctx.imSdkService) return JSON.stringify({ ok: false, error: 'imSdkService 不可用' });
        var clm = ctx.imSdkService.conversationListManager;
        if (!clm || !clm.loadMoreConversations) return JSON.stringify({ ok: false, error: 'loadMoreConversations 方法不可用' });
        var result = await clm.loadMoreConversations();
        return JSON.stringify({ ok: true, result: result });
    } catch (e) {
        return JSON.stringify({ ok: false, error: e.message });
    }
}
`
