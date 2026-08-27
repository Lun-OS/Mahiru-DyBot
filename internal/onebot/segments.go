package onebot

// 消息段构建与 CQ 码格式化。

import (
	"encoding/json"
	"strconv"
	"strings"

	"mahiru-dybot/internal/browser"
)

// strconvParse 宽松解析整数字符串。
func strconvParse(s string) (int64, bool) {
	if s == "" {
		return 0, false
	}
	n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	return n, err == nil
}

// buildSegments 将浏览器消息转为 OneBot 消息段数组。
func buildSegments(incoming *browser.IncomingMessage) []map[string]interface{} {
	var segments []map[string]interface{}
	switch incoming.MsgType {
	case "sticker", "image":
		file := "internal://" + incoming.ServerID
		// content 内若含 url 列表则透传第一个可访问地址
		var parsed struct {
			URL struct {
				URLList []string `json:"url_list"`
			} `json:"url"`
		}
		urls := []string{}
		if json.Unmarshal([]byte(incoming.Content), &parsed) == nil && len(parsed.URL.URLList) > 0 {
			urls = parsed.URL.URLList
			if len(urls) > 0 {
				file = urls[0]
			}
		}
		data := map[string]interface{}{"file": file}
		if len(urls) > 0 {
			data["url"] = urls[0]
		}
		segments = append(segments, map[string]interface{}{"type": "image", "data": data})
	default:
		text := incoming.Text
		if text == "" && incoming.Type != 7 {
			text = "[消息 type=" + strconv.Itoa(incoming.Type) + "]"
		}
		segments = append(segments, map[string]interface{}{
			"type": "text",
			"data": map[string]interface{}{"text": text},
		})
	}
	return segments
}

// formatCQCode 将消息段数组格式化为 CQ 码字符串。
func formatCQCode(segments []map[string]interface{}) string {
	var sb strings.Builder
	for _, seg := range segments {
		t, _ := seg["type"].(string)
		data, _ := seg["data"].(map[string]interface{})
		switch t {
		case "text":
			text, _ := data["text"].(string)
			sb.WriteString(text)
		case "image":
			file, _ := data["file"].(string)
			sb.WriteString("[CQ:image,file=")
			sb.WriteString(file)
			sb.WriteString("]")
		case "face":
			id, _ := data["id"].(string)
			sb.WriteString("[CQ:face,id=")
			sb.WriteString(id)
			sb.WriteString("]")
		default:
			b, _ := json.Marshal(seg)
			sb.WriteString(string(b))
		}
	}
	return sb.String()
}
