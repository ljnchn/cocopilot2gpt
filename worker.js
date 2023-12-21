addEventListener('fetch', event => {
    event.respondWith(fetchAndStream(event.request));
});

async function fetchAndStream(request) {
    // 从请求中获取json数据
    if (new URL(request.url).pathname === '/v1/chat/completions') {
        return handleChatCompletionsRequest(request);
    }

    if (new URL(request.url).pathname === '/v1/models' && request.method === 'GET') {
        let data = {
            "object": "list",
            "data": [
                {
                    "id": "gpt-4-0314", "object": "model", "created": 1687882410,
                    "owned_by": "openai", "root": "gpt-4-0314", "parent": null
                },
                {
                    "id": "gpt-4-0613", "object": "model", "created": 1686588896,
                    "owned_by": "openai", "root": "gpt-4-0613", "parent": null
                },
                {
                    "id": "gpt-4", "object": "model", "created": 1687882411,
                    "owned_by": "openai", "root": "gpt-4", "parent": null
                },
                {
                    "id": "gpt-3.5-turbo", "object": "model", "created": 1677610602,
                    "owned_by": "openai", "root": "gpt-3.5-turbo", "parent": null
                },
                {
                    "id": "gpt-3.5-turbo-0301", "object": "model", "created": 1677649963,
                    "owned_by": "openai", "root": "gpt-3.5-turbo-0301", "parent": null
                },
            ]
        }
        return new Response(JSON.stringify(data), {
            status: 200,
            headers: {
                "Content-Type": "application/json"
            }
        })
    } else {
        return handle404Page(request);
    }
}

async function handleChatCompletionsRequest(request) {
    let json_data;
    try {
        json_data = await request.json();
    } catch (error) {
        return new Response("Request body is missing or not in JSON format", { status: 400 });
    }

    // 获取Authorization头部信息
    let authorizationHeader = request.headers.get('Authorization');
    let GHO_TOKEN;
    if (authorizationHeader) {
        GHO_TOKEN = authorizationHeader.split(' ')[1];
    }

    if (!GHO_TOKEN) {
        return new Response("Authorization header is missing", { status: 401 });
    }
    // Check if stream option is set in the request data
    let stream = json_data.stream || false;

    // 构建请求头部
    let headers = {
        'Host': 'api.github.com',
        'Authorization': `token ${GHO_TOKEN}`,
        "Editor-Version": "vscode/1.84.2",
        "Editor-Plugin-Version": "copilot/1.138.0",
        "User-Agent": "GithubCopilot/1.138.0",
        "Accept": "*/*",
        "Accept-Encoding": "gzip, deflate, br",
        "Connection": "close",
    };

    // 向GitHub Copilot的内部API请求token
    let tokenResponse = await fetch('https://api.github.com/copilot_internal/v2/token', { headers });
    if (!tokenResponse.ok) {
        return new Response("Failed to fetch token", { status: tokenResponse.status });
    }

    let tokenData = await tokenResponse.json();
    let access_token = tokenData.token;
    let uuidValue = await uuid();
    let sessionId = uuidValue + Date.now();
    let machineId = await sha256(uuidValue);

    // 构建新的请求头部
    let acc_headers = {
        'Authorization': `Bearer ${access_token}`,
        "X-Request-Id": uuidValue,
        "Vscode-Sessionid": sessionId,
        "Vscode-machineid": machineId,
        "Editor-Version": "vscode/1.84.2",
        "Editor-Plugin-Version": "copilot-chat/0.10.2",
        "Openai-Organization": "github-copilot",
        "Openai-Intent": "conversation-panel",
        "Content-Type": "application/json",
        "User-Agent": "GitHubCopilotChat/0.10.2",
        "Accept": "*/*",
        "Accept-Encoding": "gzip, deflate, br",
    };

    // 向GitHub Copilot的聊天完成API发送请求
    let copilotResponse = await fetch('https://api.githubcopilot.com/chat/completions', {
        method: 'POST',
        headers: acc_headers,
        body: JSON.stringify(json_data),
    });
    // 如果stream为true，返回流式响应
    if (stream) {
        // let { readable, writable } = new TransformStream();
        let { readable, writable } = new TransformStream({
          async transform(chunk, controller) {
            // 在这里对数据进行修改，chunk是每次读取到的数据块

            // 例如，将数据块转为字符串，修改内容，然后再转回为Uint8Array
            let text = new TextDecoder().decode(chunk);
            let modifiedText = text.replace(/"content":null/g, '"content":""');
            let modifiedChunk = new TextEncoder().encode(modifiedText);

            // 将修改后的数据块写入可写流
            controller.enqueue(modifiedChunk);
          },
        });
        // Start pumping the body. NOTE: No await!
        copilotResponse.body.pipeTo(writable);

        var myHeaders = new Headers();
        myHeaders.append("Content-Type", "text/event-stream; charset=utf-8");
        myHeaders.append("Cache-Control", "no-cache, must-revalidate");
        myHeaders.append("Connection", "keep-alive");
        console.log(copilotResponse.headers)
        return new Response(readable, {
            status: copilotResponse.status,
            headers: myHeaders,
        });

    } else {
        // 否则，将响应内容解析为JSON并返回
        let copilotData = await copilotResponse.json();

        return new Response(JSON.stringify(copilotData), {
            status: copilotResponse.status,
            headers: copilotResponse.headers,
        });
    }
}
async function handle404Page(request) {
    return new Response('Internal Server Error', { status: 500 });
}

async function uuid() { // UUID v4 generator in JavaScript (RFC4122 compliant)
    return ([1e7] + -1e3 + -4e3 + -8e3 + -1e11).replace(/[018]/g, c =>
        (c ^ crypto.getRandomValues(new Uint8Array(1))[0] & 15 >> c / 4).toString(16)
    );
}

async function sha256(message) {
    // encode as UTF-8
    const msgBuffer = await new TextEncoder().encode(message);
    // hash the message
    const hashBuffer = await crypto.subtle.digest("SHA-256", msgBuffer);
    // convert bytes to hex string
    return [...new Uint8Array(hashBuffer)]
        .map((b) => b.toString(16).padStart(2, "0"))
        .join("");
}
