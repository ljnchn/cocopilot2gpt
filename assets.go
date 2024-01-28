package main

import (
	"time"

	"github.com/jessevdk/go-assets"
)

var _Assetsf86c9bbb3d0fc4401249716089ed36f50263323a = "<!doctype html>\n<html>\n<h1>\n    {{ .title }}\n</h1>\n打开链接 <a href=\"https://github.com/login/device\" target=\"__blank\">https://github.com/login/device</a>\n<!-- <button onclick=\"copyToClipboard()\">复制</button> -->\n<p>输入：<input type=\"text\" value=\"{{ .userCode }}\" disabled=\"disabled\" id=\"code\" size=\"10\"></p>\n<p>获取有延迟，填写完毕请勿刷新页面</p>\n<p>ghu: <input type=\"text\" value=\"获取中\" id=\"ghu\" size=\"50\"> &nbsp <button onclick=\"checkGhu(this)\">检测</button></p>\n<script>\n    var deviceCode = \"{{ .deviceCode }}\"\n\n    let intervalId = null;\n    let count = 0;\n\n    // 每5秒执行一次的函数\n    function polling() {\n        count++;\n        console.log('Polling: ' + count);\n        var xhr = new XMLHttpRequest();\n        xhr.open(\"POST\", \"/auth/check\", true);\n        var formData = new FormData();\n        formData.append(\"deviceCode\", deviceCode);\n\n        xhr.onreadystatechange = function () {\n            if (xhr.status >= 200 && xhr.status < 300) {\n                if (xhr.responseText.length > 0) {\n                    try {\n                        var data = JSON.parse(xhr.responseText);\n                        // 现在你可以使用解析后的数据了\n                        if (data.code == \"0\") {\n                            stopPolling();\n                            var inputElement = document.getElementById('ghu');\n                            inputElement.value = data.data; // 将值更改为你想要的任何字符串\n                        }\n                    } catch (e) {\n                        console.error(\"解析JSON数据时出错: \", e);\n                    }\n                }\n            } else {\n                console.error(\"请求失败，HTTP状态码: \", xhr.status);\n            }\n        };\n        xhr.send(formData);\n    }\n\n    // 15分钟后停止轮询的函数\n    function stopPolling() {\n        clearInterval(intervalId);\n        console.log('Polling stopped');\n    }\n\n    // 开始轮询\n    intervalId = setInterval(polling, 6 * 1000); // 5秒\n\n    // 15分钟后停止轮询\n    setTimeout(stopPolling, 15 * 60 * 1000); // 15分钟\n\n    function copyToClipboard() {\n        let text = document.getElementById('code').innerHTML;\n        const copyContent = async () => {\n            try {\n                await navigator.clipboard.writeText(text);\n                console.log('Content copied to clipboard');\n            } catch (err) {\n                console.error('Failed to copy: ', err);\n            }\n        }\n    }\n\n    function checkGhu(btn) {\n        btn.disabled = true;\n\n        var ghu = document.getElementById('ghu').value;\n        var xhr = new XMLHttpRequest();\n        xhr.open(\"POST\", \"/auth/checkGhu\", true);\n        var formData = new FormData();\n        formData.append(\"ghu\", ghu);\n\n        xhr.onreadystatechange = function () {\n            // 请求结束，无论成功还是失败，都重新启用按钮\n            if (xhr.readyState == 4) {\n                btn.disabled = false;\n                if (xhr.status >= 200 && xhr.status < 300) {\n                    if (xhr.responseText.length > 0) {\n                        try {\n                            var data = JSON.parse(xhr.responseText);\n                            // 现在你可以使用解析后的数据了\n                            if (data.code == \"0\") {\n                                alert(data.data);\n                            } else {\n                                alert(data.msg);\n                            }\n                        } catch (e) {\n                            console.error(\"解析JSON数据时出错: \", e);\n                        }\n                    }\n                } else {\n                    console.error(\"请求失败，HTTP状态码: \", xhr.status);\n                }\n            }\n\n        };\n        xhr.send(formData);\n    }\n\n</script>\n\n</html>"

// Assets returns go-assets FileSystem
var Assets = assets.NewFileSystem(map[string][]string{"/": []string{"html"}, "/html": []string{"auth.tmpl"}}, map[string]*assets.File{
	"/": &assets.File{
		Path:     "/",
		FileMode: 0x800001ed,
		Mtime:    time.Unix(1706450942, 1706450942218803764),
		Data:     nil,
	}, "/html": &assets.File{
		Path:     "/html",
		FileMode: 0x800001ed,
		Mtime:    time.Unix(1706329545, 1706329545851955289),
		Data:     nil,
	}, "/html/auth.tmpl": &assets.File{
		Path:     "/html/auth.tmpl",
		FileMode: 0x1a4,
		Mtime:    time.Unix(1706451252, 1706451252248792005),
		Data:     []byte(_Assetsf86c9bbb3d0fc4401249716089ed36f50263323a),
	}}, "")
