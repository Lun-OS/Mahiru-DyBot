package browser

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ImageXUploadConfig 来自 /aweme/v1/web/im/upload/config/v2 的 STS 上传配置。
type ImageXUploadConfig struct {
	AccessKeyID     string `json:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key"`
	SessionToken    string `json:"session_token"`
	SpaceName       string `json:"space_name"`
	ExpireAt        int64  `json:"expire_at"`
}

// ImageXUploadResult 上传成功后返回的结果。
type ImageXUploadResult struct {
	URI   string `json:"uri"`   // image_id，用于消息 content
	URL   string `json:"url"`   // 图片访问 URL
	Width int    `json:"width"`
	Height int   `json:"height"`
}

// applyUploadResponse ImageX ApplyImageUpload API 响应。
type applyUploadResponse struct {
	Result struct {
		UploadAddress struct {
			StoreInfos []struct {
				StoreKey string `json:"StoreKey"`
			} `json:"StoreInfos"`
			UploadHosts []string `json:"UploadHosts"`
			SessionKey  string   `json:"SessionKey"`
		} `json:"UploadAddress"`
		RequestId string `json:"RequestId"`
	} `json:"Result"`
	ResponseMetadata struct {
		RequestId string `json:"RequestId"`
		Error     *struct {
			Code    string `json:"Code"`
			Message string `json:"Message"`
		} `json:"Error,omitempty"`
	} `json:"ResponseMetadata"`
}

// commitUploadResponse ImageX CommitUploadImage API 响应。
type commitUploadResponse struct {
	Result struct {
		PluginResult []struct {
			ImageUri string `json:"ImageUri"`
			Width    int    `json:"Width"`
			Height   int    `json:"Height"`
		} `json:"PluginResult"`
		RequestId string `json:"RequestId"`
	} `json:"Result"`
	ResponseMetadata struct {
		RequestId string `json:"RequestId"`
		Error     *struct {
			Code    string `json:"Code"`
			Message string `json:"Message"`
		} `json:"Error,omitempty"`
	} `json:"ResponseMetadata"`
}

// hmacSHA256 HMAC-SHA256 签名。
func hmacSHA256(key []byte, data string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(data))
	return h.Sum(nil)
}

// sha256Hex 计算 SHA-256 的十六进制。
func sha256Hex(data []byte) string {
	h := sha256.New()
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

// signHeaders 生成 AWS v4 签名头。
func signHeaders(
	method, host, path string,
	queryParams map[string]string,
	body []byte,
	accessKeyID, secretAccessKey, sessionToken, region, service string,
	date time.Time,
) map[string]string {
	dateStr := date.UTC().Format("20060102")
	datetime := date.UTC().Format("20060102T150405Z")

	// 1. 创建规范化查询字符串
	var sortedKeys []string
	for k := range queryParams {
		sortedKeys = append(sortedKeys, k)
	}
	sort.Strings(sortedKeys)
	var canonicalQuery []string
	for _, k := range sortedKeys {
		canonicalQuery = append(canonicalQuery, k+"="+queryParams[k])
	}
	canonicalQueryString := strings.Join(canonicalQuery, "&")

	// 2. 创建规范头
	bodyHash := sha256Hex(body)
	headers := map[string]string{
		"content-type":  "application/json",
		"host":          host,
		"x-date":        datetime,
		"x-content-sha256": bodyHash,
	}
	if sessionToken != "" {
		headers["x-security-token"] = sessionToken
	}

	var sortedHeaderKeys []string
	for k := range headers {
		sortedHeaderKeys = append(sortedHeaderKeys, k)
	}
	sort.Strings(sortedHeaderKeys)
	var canonicalHeaders []string
	var signedHeaderList []string
	for _, k := range sortedHeaderKeys {
		canonicalHeaders = append(canonicalHeaders, k+":"+strings.TrimSpace(headers[k]))
		signedHeaderList = append(signedHeaderList, k)
	}
	canonicalHeadersStr := strings.Join(canonicalHeaders, "\n") + "\n"
	signedHeaders := strings.Join(signedHeaderList, ";")

	// 3. 规范化请求
	canonicalRequest := method + "\n" + path + "\n" + canonicalQueryString + "\n" + canonicalHeadersStr + "\n" + signedHeaders + "\n" + bodyHash

	// 4. 创建签名字符串
	algorithm := "HMAC-SHA256"
	credentialScope := dateStr + "/" + region + "/" + service + "/aws4_request"
	stringToSign := algorithm + "\n" + datetime + "\n" + credentialScope + "\n" + sha256Hex([]byte(canonicalRequest))

	// 5. 计算签名
	kDate := hmacSHA256([]byte("AWS4"+secretAccessKey), dateStr)
	kRegion := hmacSHA256(kDate, region)
	kService := hmacSHA256(kRegion, service)
	kSigning := hmacSHA256(kService, "aws4_request")
	signature := hex.EncodeToString(hmacSHA256(kSigning, stringToSign))

	// 6. 构造 Authorization 头
	auth := algorithm + " Credential=" + accessKeyID + "/" + credentialScope + ", SignedHeaders=" + signedHeaders + ", Signature=" + signature

	result := map[string]string{
		"Authorization":  auth,
		"Content-Type":   "application/json",
		"Host":           host,
		"X-Date":         datetime,
		"X-Content-Sha256": bodyHash,
	}
	if sessionToken != "" {
		result["X-Security-Token"] = sessionToken
	}
	return result
}

// ImageXApplyImageUpload 调用 ImageX ApplyImageUpload 获取上传地址。
func ImageXApplyImageUpload(
	cfg *ImageXUploadConfig,
	serviceID string,
) (*applyUploadResponse, error) {
	host := "imagex.volcengineapi.com"
	path := "/"
	date := time.Now()

	body := []byte("{}")
	queryParams := map[string]string{
		"Action":    "ImageXApplyImageUpload",
		"Version":   "2018-08-01",
		"ServiceId": serviceID,
	}

	headers := signHeaders("POST", host, path, queryParams, body,
		cfg.AccessKeyID, cfg.SecretAccessKey, cfg.SessionToken,
		"cn-north-1", "imagex", date)

	// 构造 URL
	var qs []string
	for k, v := range queryParams {
		qs = append(qs, k+"="+v)
	}
	url := "https://" + host + path + "?" + strings.Join(qs, "&")

	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	log.Printf("[ImageX] ApplyUpload HTTP %d, body=%s", resp.StatusCode, string(respBody))

	var result applyUploadResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w (body=%s)", err, string(respBody))
	}
	if result.ResponseMetadata.Error != nil {
		return nil, fmt.Errorf("API 错误: %s - %s", result.ResponseMetadata.Error.Code, result.ResponseMetadata.Error.Message)
	}
	return &result, nil
}

// ImageXCommitUpload 调用 ImageX CommitUploadImage 完成上传。
func ImageXCommitUpload(
	cfg *ImageXUploadConfig,
	serviceID, sessionKey string,
	storeKeys []string,
) (*commitUploadResponse, error) {
	host := "imagex.volcengineapi.com"
	path := "/"
	date := time.Now()

	commitBody := map[string]interface{}{
		"SessionKey":  sessionKey,
		"Functions":   []string{},
		"UploadMetas": []map[string]interface{}{},
	}
	for _, key := range storeKeys {
		commitBody["UploadMetas"] = append(commitBody["UploadMetas"].([]map[string]interface{}), map[string]interface{}{
			"StoreUri": key,
			"IsBackup": false,
		})
	}

	body, _ := json.Marshal(commitBody)
	queryParams := map[string]string{
		"Action":    "ImageXCommitImageUpload",
		"Version":   "2018-08-01",
		"ServiceId": serviceID,
	}

	headers := signHeaders("POST", host, path, queryParams, body,
		cfg.AccessKeyID, cfg.SecretAccessKey, cfg.SessionToken,
		"cn-north-1", "imagex", date)

	var qs []string
	for k, v := range queryParams {
		qs = append(qs, k+"="+v)
	}
	url := "https://" + host + path + "?" + strings.Join(qs, "&")

	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	log.Printf("[ImageX] CommitUpload HTTP %d, body=%s", resp.StatusCode, string(respBody))

	var result commitUploadResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w (body=%s)", err, string(respBody))
	}
	if result.ResponseMetadata.Error != nil {
		return nil, fmt.Errorf("API 错误: %s - %s", result.ResponseMetadata.Error.Code, result.ResponseMetadata.Error.Message)
	}
	return &result, nil
}

// ImageXUploadFile 上传文件到 ImageX 并返回结果。
func ImageXUploadFile(
	cfg *ImageXUploadConfig,
	fileData []byte,
	width, height int,
) (*ImageXUploadResult, error) {
	serviceID := cfg.SpaceName

	// 1. ApplyUpload 获取上传地址
	applyResp, err := ImageXApplyImageUpload(cfg, serviceID)
	if err != nil {
		return nil, fmt.Errorf("ApplyUpload 失败: %w", err)
	}

	uploadAddr := applyResp.Result.UploadAddress
	if len(uploadAddr.UploadHosts) == 0 || len(uploadAddr.StoreInfos) == 0 {
		return nil, fmt.Errorf("ApplyUpload 返回空地址")
	}

	host := uploadAddr.UploadHosts[0]
	storeKey := uploadAddr.StoreInfos[0].StoreKey
	sessionKey := uploadAddr.SessionKey

	// 2. 上传文件到 UploadHost
	uploadURL := "https://" + host + "/?Action=CommitUploadImage&Version=2018-08-01&ServiceId=" + serviceID

	// 构造 multipart form boundary
	boundary := "----GoBoundary" + strconv.FormatInt(time.Now().UnixNano(), 16)
	var buf bytes.Buffer
	buf.WriteString("--" + boundary + "\r\n")
	buf.WriteString("Content-Disposition: form-data; name=\"file\"; filename=\"image.png\"\r\n")
	buf.WriteString("Content-Type: image/png\r\n\r\n")
	buf.Write(fileData)
	buf.WriteString("\r\n--" + boundary + "--\r\n")

	uploadReq, err := http.NewRequest("POST", uploadURL, &buf)
	if err != nil {
		return nil, fmt.Errorf("创建上传请求失败: %w", err)
	}
	uploadReq.Header.Set("Content-Type", "multipart/form-data; boundary="+boundary)
	uploadReq.Header.Set("Authorization", "Basic "+hex.EncodeToString([]byte(storeKey)))

	uploadResp, err := http.DefaultClient.Do(uploadReq)
	if err != nil {
		return nil, fmt.Errorf("上传文件失败: %w", err)
	}
	defer uploadResp.Body.Close()

	uploadBody, _ := io.ReadAll(uploadResp.Body)
	log.Printf("[ImageX] UploadFile HTTP %d, body=%s", uploadResp.StatusCode, string(uploadBody))

	if uploadResp.StatusCode < 200 || uploadResp.StatusCode >= 300 {
		return nil, fmt.Errorf("上传文件失败: HTTP %d, body=%s", uploadResp.StatusCode, string(uploadBody))
	}

	// 3. CommitUpload
	commitResp, err := ImageXCommitUpload(cfg, serviceID, sessionKey, []string{storeKey})
	if err != nil {
		return nil, fmt.Errorf("CommitUpload 失败: %w", err)
	}

	if len(commitResp.Result.PluginResult) == 0 {
		return nil, fmt.Errorf("CommitUpload 返回空结果")
	}

	pr := commitResp.Result.PluginResult[0]
	return &ImageXUploadResult{
		URI:    pr.ImageUri,
		URL:    "https://" + host + "/" + pr.ImageUri,
		Width:  pr.Width,
		Height: pr.Height,
	}, nil
}
