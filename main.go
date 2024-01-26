package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/patrickmn/go-cache"
	"github.com/tidwall/gjson"
)

const tokenUrl = "https://api.github.com/copilot_internal/v2/token"
const completionsUrl = "https://api.githubcopilot.com/chat/completions"
const embeddingsUrl = "https://api.githubcopilot.com/embeddings"

var requestUrl = ""

type Model struct {
	ID      string  `json:"id"`
	Object  string  `json:"object"`
	Created int     `json:"created"`
	OwnedBy string  `json:"owned_by"`
	Root    string  `json:"root"`
	Parent  *string `json:"parent"`
}

type ModelList struct {
	Object string  `json:"object"`
	Data   []Model `json:"data"`
}

var version = "v0.5"
var port = "8081"
var client_id = ""

func main() {
	err := godotenv.Load()
	if err == nil {
		// 从环境变量中获取配置值
		portEnv := os.Getenv("PORT")
		if portEnv != "" {
			port = portEnv
		}
	}

	log.Printf("Server is running on port %s, version: %s\n", port, version)

	gin.SetMode(gin.ReleaseMode)

	r := gin.Default()

	// CORS 中间件
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	r.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, `
		curl --location 'http://127.0.0.1:8081/v1/chat/completions' \
		--header 'Content-Type: application/json' \
		--header 'Authorization: Bearer ghu_xxx' \
		--data '{
		  "model": "gpt-4",
		  "messages": [{"role": "user", "content": "hi"}]
		}'`)
	})

	r.GET("/v1/models", func(c *gin.Context) {
		c.JSON(http.StatusOK, models())
	})

	r.POST("/v1/chat/completions", func(c *gin.Context) {
		c.Header("Cache-Control", "no-cache, must-revalidate")
		c.Header("Connection", "keep-alive")

		requestUrl = completionsUrl
		forwardRequest(c)
	})

	r.POST("/v1/embeddings", func(c *gin.Context) {
		c.Header("Cache-Control", "no-cache, must-revalidate")
		c.Header("Connection", "keep-alive")

		requestUrl = embeddingsUrl
		forwardRequest(c)
	})

	// 获取ghu
	client_id = os.Getenv("CLIENT_ID")

	r.LoadHTMLGlob("templates/**/*")
	r.GET("/auth", func(c *gin.Context) {
		if client_id == "" {
			c.String(http.StatusOK, `clent id null`)
			return
		}
		// 获取设备授权码
		deviceCode, userCode, err := getDeviceCode()
		if err != nil {
			c.String(http.StatusOK, "获取设备码失败："+err.Error())
			return
		}

		// 使用 deviceCode 和 userCode
		fmt.Println("Device Code: ", deviceCode)
		fmt.Println("User Code: ", userCode)

		c.HTML(http.StatusOK, "auth/index.tmpl", gin.H{
			"title":      "auth index",
			"deviceCode": deviceCode,
			"userCode":   userCode,
		})
	})

	r.POST("/auth/check", func(c *gin.Context) {
		returnData := map[string]string{
			"code": "1",
			"msg":  "",
			"data": "",
		}
		if client_id == "" {
			returnData["msg"] = "clent id null"
			c.JSON(http.StatusOK, returnData)
			return
		}
		deviceCode := c.PostForm("deviceCode")
		if deviceCode == "" {
			returnData["msg"] = "device code null"
			c.JSON(http.StatusOK, returnData)
			return
		}
		token, err := checkUserCode(deviceCode)
		if err != nil {
			returnData["msg"] = err.Error()
			c.JSON(http.StatusOK, returnData)
			return
		}
		if token == "" {
			returnData["msg"] = "token null"
			c.JSON(http.StatusOK, returnData)
			return
		}
		returnData["code"] = "0"
		returnData["msg"] = "success"
		returnData["data"] = token
		c.JSON(http.StatusOK, returnData)
		return
	})

	r.Run(":" + port)
}

func forwardRequest(c *gin.Context) {
	var jsonBody map[string]interface{}
	if err := c.ShouldBindJSON(&jsonBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Request body is missing or not in JSON format"})
		return
	}

	ghuToken := strings.Split(c.GetHeader("Authorization"), " ")[1]

	if !strings.HasPrefix(ghuToken, "gh") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "auth token not found"})
		log.Printf("token 格式错误：%s\n", ghuToken)
		return
	}

	// 检查 token 是否有效
	if !checkToken(ghuToken) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "auth token is invalid"})
		log.Printf("token 无效：%s\n", ghuToken)
		return
	}
	accToken, err := getAccToken(ghuToken)
	if accToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	sessionId := fmt.Sprintf("%s%d", uuid.New().String(), time.Now().UnixNano()/int64(time.Millisecond))
	machineID := sha256.Sum256([]byte(uuid.New().String()))
	machineIDStr := hex.EncodeToString(machineID[:])
	accHeaders := getAccHeaders(accToken, uuid.New().String(), sessionId, machineIDStr)
	client := &http.Client{}

	jsonData, err := json.Marshal(jsonBody)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	isStream := gjson.GetBytes(jsonData, "stream").String() == "true"

	req, err := http.NewRequest("POST", requestUrl, bytes.NewBuffer(jsonData))
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
	}

	for key, value := range accHeaders {
		req.Header.Add(key, value)
	}

	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Fatal(err)
		}
		bodyString := string(bodyBytes)
		log.Printf("对话失败：%d, %s ", resp.StatusCode, bodyString)
		cache := cache.New(5*time.Minute, 10*time.Minute)
		cache.Delete(ghuToken)
		c.AbortWithError(resp.StatusCode, fmt.Errorf(bodyString))
		return
	}

	c.Header("Content-Type", "application/json; charset=utf-8")

	if isStream {
		returnStream(c, resp)
	} else {
		returnJson(c, resp)
	}
	return
}

func returnJson(c *gin.Context, resp *http.Response) {
	c.Header("Content-Type", "application/json; charset=utf-8")

	body, err := io.ReadAll(resp.Body.(io.Reader))
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	c.Writer.Write(body)
	return
}

func returnStream(c *gin.Context, resp *http.Response) {
	c.Header("Content-Type", "text/event-stream; charset=utf-8")

	// 创建一个新的扫描器
	scanner := bufio.NewScanner(resp.Body)

	// 使用Scan方法来读取流
	for scanner.Scan() {
		line := scanner.Bytes()

		// 替换 "content":null 为 "content":""
		modifiedLine := bytes.Replace(line, []byte(`"content":null`), []byte(`"content":""`), -1)

		// 将修改后的数据写入响应体
		if _, err := c.Writer.Write(modifiedLine); err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}

		// 添加一个换行符
		if _, err := c.Writer.Write([]byte("\n")); err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}
	}

	if scanner.Err() != nil {
		// 处理来自扫描器的任何错误
		c.AbortWithError(http.StatusInternalServerError, scanner.Err())
		return
	}
	return
}

func models() ModelList {
	jsonStr := `{
        "object": "list",
        "data": [
            {"id": "text-search-babbage-doc-001","object": "model","created": 1651172509,"owned_by": "openai-dev"},
            {"id": "gpt-4-0613","object": "model","created": 1686588896,"owned_by": "openai"},
            {"id": "gpt-4", "object": "model", "created": 1687882411, "owned_by": "openai"},
            {"id": "babbage", "object": "model", "created": 1649358449, "owned_by": "openai"},
            {"id": "gpt-3.5-turbo-0613", "object": "model", "created": 1686587434, "owned_by": "openai"},
            {"id": "text-babbage-001", "object": "model", "created": 1649364043, "owned_by": "openai"},
            {"id": "gpt-3.5-turbo", "object": "model", "created": 1677610602, "owned_by": "openai"},
            {"id": "gpt-3.5-turbo-1106", "object": "model", "created": 1698959748, "owned_by": "system"},
            {"id": "curie-instruct-beta", "object": "model", "created": 1649364042, "owned_by": "openai"},
            {"id": "gpt-3.5-turbo-0301", "object": "model", "created": 1677649963, "owned_by": "openai"},
            {"id": "gpt-3.5-turbo-16k-0613", "object": "model", "created": 1685474247, "owned_by": "openai"},
            {"id": "text-embedding-ada-002", "object": "model", "created": 1671217299, "owned_by": "openai-internal"},
            {"id": "davinci-similarity", "object": "model", "created": 1651172509, "owned_by": "openai-dev"},
            {"id": "curie-similarity", "object": "model", "created": 1651172510, "owned_by": "openai-dev"},
            {"id": "babbage-search-document", "object": "model", "created": 1651172510, "owned_by": "openai-dev"},
            {"id": "curie-search-document", "object": "model", "created": 1651172508, "owned_by": "openai-dev"},
            {"id": "babbage-code-search-code", "object": "model", "created": 1651172509, "owned_by": "openai-dev"},
            {"id": "ada-code-search-text", "object": "model", "created": 1651172510, "owned_by": "openai-dev"},
            {"id": "text-search-curie-query-001", "object": "model", "created": 1651172509, "owned_by": "openai-dev"},
            {"id": "text-davinci-002", "object": "model", "created": 1649880484, "owned_by": "openai"},
            {"id": "ada", "object": "model", "created": 1649357491, "owned_by": "openai"},
            {"id": "text-ada-001", "object": "model", "created": 1649364042, "owned_by": "openai"},
            {"id": "ada-similarity", "object": "model", "created": 1651172507, "owned_by": "openai-dev"},
            {"id": "code-search-ada-code-001", "object": "model", "created": 1651172507, "owned_by": "openai-dev"},
            {"id": "text-similarity-ada-001", "object": "model", "created": 1651172505, "owned_by": "openai-dev"},
            {"id": "text-davinci-edit-001", "object": "model", "created": 1649809179, "owned_by": "openai"},
            {"id": "code-davinci-edit-001", "object": "model", "created": 1649880484, "owned_by": "openai"},
            {"id": "text-search-curie-doc-001", "object": "model", "created": 1651172509, "owned_by": "openai-dev"},
            {"id": "text-curie-001", "object": "model", "created": 1649364043, "owned_by": "openai"},
            {"id": "curie", "object": "model", "created": 1649359874, "owned_by": "openai"},
            {"id": "davinci", "object": "model", "created": 1649359874, "owned_by": "openai"},
            {"id": "gpt-4-0314", "object": "model", "created": 1687882410, "owned_by": "openai"}
        ]
    }`

	var modelList ModelList
	json.Unmarshal([]byte(jsonStr), &modelList)
	return modelList
}

func getAccToken(ghuToken string) (string, error) {
	var accToken = ""

	cache := cache.New(15*time.Minute, 60*time.Minute)
	cacheToken, found := cache.Get(ghuToken)
	if found {
		accToken = cacheToken.(string)
	} else {
		client := &http.Client{}
		req, err := http.NewRequest("GET", tokenUrl, nil)
		if err != nil {
			return accToken, err
		}

		headers := getHeaders(ghuToken)

		for key, value := range headers {
			req.Header.Add(key, value)
		}

		resp, err := client.Do(req)
		if err != nil {
			return accToken, err
		}
		defer resp.Body.Close()

		var reader interface{}
		switch resp.Header.Get("Content-Encoding") {
		case "gzip":
			reader, err = gzip.NewReader(resp.Body)
			if err != nil {
				return accToken, fmt.Errorf("数据解压失败")
			}
		default:
			reader = resp.Body
		}

		body, err := io.ReadAll(reader.(io.Reader))
		if err != nil {
			return accToken, fmt.Errorf("数据读取失败")
		}
		if resp.StatusCode == http.StatusOK {
			accToken = gjson.GetBytes(body, "token").String()
			if accToken == "" {
				return accToken, fmt.Errorf("acc_token 未返回")
			}
			cache.Set(ghuToken, accToken, 14*time.Minute)
		} else {
			log.Printf("获取 acc_token 请求失败：%d, %s ", resp.StatusCode, string(body))
			return accToken, fmt.Errorf("获取 acc_token 请求失败： %d", resp.StatusCode)
		}
	}
	return accToken, nil
}

func checkToken(ghuToken string) bool {
	client := &http.Client{}

	url := "https://api.github.com/user"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		fmt.Println(err)
		return false
	}
	req.Header.Add("Accept", "application/vnd.github+json")
	req.Header.Add("Authorization", "Bearer "+ghuToken)
	req.Header.Add("X-GitHub-Api-Version", "2022-11-28")

	resp, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func getHeaders(ghoToken string) map[string]string {
	return map[string]string{
		"Host":          "api.github.com",
		"Authorization": "token " + ghoToken,

		"Editor-Version":        "vscode/1.85.1",
		"Editor-Plugin-Version": "copilot-chat/0.11.1",
		"User-Agent":            "GitHubCopilotChat/0.11.1",
		"Accept":                "*/*",
		"Accept-Encoding":       "gzip, deflate, br",
	}
}

func getAccHeaders(accessToken, uuid string, sessionId string, machineId string) map[string]string {
	return map[string]string{
		"Host":                   "api.githubcopilot.com",
		"Authorization":          "Bearer " + accessToken,
		"X-Request-Id":           uuid,
		"X-Github-Api-Version":   "2023-07-07",
		"Vscode-Sessionid":       sessionId,
		"Vscode-machineid":       machineId,
		"Editor-Version":         "vscode/1.85.1",
		"Editor-Plugin-Version":  "copilot-chat/0.11.1",
		"Openai-Organization":    "github-copilot",
		"Openai-Intent":          "conversation-panel",
		"Content-Type":           "application/json",
		"User-Agent":             "GitHubCopilotChat/0.11.1",
		"Copilot-Integration-Id": "vscode-chat",
		"Accept":                 "*/*",
		"Accept-Encoding":        "gzip, deflate, br",
	}
}

func getDeviceCode() (string, string, error) {
	requestUrl := "https://github.com/login/device/code"

	body := url.Values{}
	headers := map[string]string{
		"Accept": "application/json",
	}

	body.Set("client_id", client_id)
	res, err := handleRequest("POST", body, requestUrl, headers)
	deviceCode := gjson.Get(res, "device_code").String()
	userCode := gjson.Get(res, "user_code").String()

	if deviceCode == "" {
		return "", "", fmt.Errorf("device code null")
	}
	if userCode == "" {
		return "", "", fmt.Errorf("user code null")
	}
	return deviceCode, userCode, err
}

func checkUserCode(deviceCode string) (string, error) {
	requestUrl := "https://github.com/login/oauth/access_token"
	body := url.Values{}
	headers := map[string]string{
		"Accept": "application/json",
	}

	body.Set("client_id", client_id)
	body.Set("device_code", deviceCode)
	body.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")
	res, err := handleRequest("POST", body, requestUrl, headers)
	fmt.Print(body)
	fmt.Printf("")
	fmt.Printf(res)
	if err != nil {
		return "", err
	}
	token := gjson.Get(res, "access_token").String()
	return token, nil
}

func handleRequest(method string, body url.Values, requestUrl string, headers map[string]string) (string, error) {
	fmt.Print(body, requestUrl)
	client := &http.Client{}

	req, err := http.NewRequest(method, requestUrl, bytes.NewBuffer([]byte(body.Encode())))
	if err != nil {
		return "", err
	}
	for key, value := range headers {
		req.Header.Add(key, value)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("status code: %d, read body error", resp.StatusCode)
	}

	return string(respBody), nil
}
