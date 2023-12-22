# 将你的 copilot 转成 ChatGPT API（支持GPT4）



## 使用声明❗❗❗

> 本项目可能导致你的 copilt 账号被封，请谨慎使用。



## ghu token 获取



点击链接：[cocopilot](https://cocopilot.org/copilot/token)，根据提示拿到 ghu_xxxx 格式的 token。务必保存好，不要泄露给其他人。

## 下载程序

点击右侧的 release 下载跟你运行环境一致的可执行文件

## 运行程序

`./copilot2gpt`

默认监听端口为 8081，可以在 .env 中修改

## 使用方式

可以在任意第三方客户端使用

API 域名：http://127.0.0.1:8081

API token：ghu_xxx

curl 测试

``` bash
curl --location 'http://127.0.0.1:8081/v1/chat/completions' \
--header 'Content-Type: application/json' \
--header 'Authorization: Bearer ghu_xxx' \
--data '{
  "stream": "true",
  "model": "gpt-4",
  "messages": [{"role": "user", "content": "hi"}]
}'
```



## 感谢以下项目，灵感来自于VV佬

[CaoYunzhou/cocopilot-gpt](https://github.com/CaoYunzhou/cocopilot-gpt)

[lvguanjun/copilot_to_chatgpt4](https://github.com/lvguanjun/copilot_to_chatgpt4)

