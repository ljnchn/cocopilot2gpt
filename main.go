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

var port = "8080"
var ghuToken = ""

func main() {
	err := godotenv.Load()
	if err == nil {
		// 从环境变量中获取配置值
		portEnv := os.Getenv("PORT")
		if portEnv != "" {
			port = portEnv
		}
		ghuToken = os.Getenv("GHU_TOKEN")
	}

	log.Printf("Server is running on port %s, with ghu: %s", port, ghuToken)

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
		c.JSON(http.StatusOK, "")
	})

	r.GET("/v1/models", func(c *gin.Context) {
		c.JSON(http.StatusOK, models())
	})

	r.POST("/v1/chat/completions", func(c *gin.Context) {
		c.Header("Cache-Control", "no-cache, must-revalidate")
		c.Header("Connection", "keep-alive")

		forwardRequest(c)
	})

	r.Run(":" + port)
}

func forwardRequest(c *gin.Context) {
	var jsonBody map[string]interface{}
	if err := c.ShouldBindJSON(&jsonBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Request body is missing or not in JSON format"})
		return
	}

	if ghuToken == "" {
		ghuToken = strings.Split(c.GetHeader("Authorization"), " ")[1]
	}

	if ghuToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "gho_token not found"})
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

	req, err := http.NewRequest("POST", completionsUrl, bytes.NewBuffer(jsonData))
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
		cache.Delete("token")
		c.AbortWithError(resp.StatusCode, err)
		return
	}

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
            {"id": "gpt-4-0314", "object": "model", "created": 1687882410, "owned_by": "openai", "root": "gpt-4-0314", "parent": null},
            {"id": "gpt-4-0613", "object": "model", "created": 1686588896, "owned_by": "openai", "root": "gpt-4-0613", "parent": null},
            {"id": "gpt-4", "object": "model", "created": 1687882411, "owned_by": "openai", "root": "gpt-4", "parent": null},
            {"id": "gpt-3.5-turbo", "object": "model", "created": 1677610602, "owned_by": "openai", "root": "gpt-3.5-turbo", "parent": null},
            {"id": "gpt-3.5-turbo-0301", "object": "model", "created": 1677649963, "owned_by": "openai", "root": "gpt-3.5-turbo-0301", "parent": null}
        ]
    }`

	var modelList ModelList
	json.Unmarshal([]byte(jsonStr), &modelList)
	return modelList
}

func getAccToken(ghuToken string) (string, error) {
	var accToken = ""

	cache := cache.New(15*time.Minute, 60*time.Minute)
	cacheToken, found := cache.Get("token")
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
			cache.Set("token", accToken, 14*time.Minute)
		} else {
			log.Printf("获取 acc_token 请求失败：%d, %s ", resp.StatusCode, string(body))
			return accToken, fmt.Errorf("获取 acc_token 请求失败： %d", resp.StatusCode)
		}
	}
	return accToken, nil
}

func getHeaders(ghoToken string) map[string]string {
	return map[string]string{
		"Host":                  "api.github.com",
		"Authorization":         "token " + ghoToken,
		"Editor-Version":        "vscode/1.84.2",
		"Editor-Plugin-Version": "copilot/1.138.0",
		"User-Agent":            "GithubCopilot/1.138.0",
		"Accept":                "*/*",
		"Accept-Encoding":       "gzip, deflate, br",
		"Connection":            "close",
	}
}

func getAccHeaders(accessToken, uuid string, sessionId string, machineId string) map[string]string {
	return map[string]string{
		"Authorization":         "Bearer " + accessToken,
		"X-Request-Id":          uuid,
		"Vscode-Sessionid":      sessionId,
		"Vscode-machineid":      machineId,
		"Editor-Version":        "vscode/1.84.2",
		"Editor-Plugin-Version": "copilot-chat/0.10.2",
		"Openai-Organization":   "github-copilot",
		"Openai-Intent":         "conversation-panel",
		"Content-Type":          "application/json",
		"User-Agent":            "GitHubCopilotChat/0.10.2",
		"Accept":                "*/*",
		"Accept-Encoding":       "gzip, deflate, br",
	}
}
