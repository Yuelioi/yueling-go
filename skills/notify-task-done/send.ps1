#!/usr/bin/env pwsh
# notify-task-done — 给月灵 Bot 发一条「✅ … 已完成」通知。
# 配置全部来自环境变量：YUELING_API_URL / YUELING_API_KEY / YUELING_OWNER_ID
param(
    [string]$Message = "任务",
    [long]$Group = 0
)

$url   = $env:YUELING_API_URL
$key   = $env:YUELING_API_KEY
$owner = $env:YUELING_OWNER_ID

if (-not $url -or -not $key) {
    Write-Error "缺少环境变量：请设置 YUELING_API_URL 和 YUELING_API_KEY"
    exit 1
}

$text = "✅ $Message 已完成"
# ConvertTo-Json 负责安全转义文本（引号 / 反斜杠 / 控制字符），返回带引号的 JSON 字符串字面量。
$textJson = ConvertTo-Json -InputObject $text -Compress
$seg = '{"type":"text","data":{"text":' + $textJson + '}}'

if ($Group -ne 0) {
    $json = '{"message_type":"group","group_id":' + $Group + ',"message":[' + $seg + ']}'
} else {
    if (-not $owner) {
        Write-Error "私聊模式需要设置 YUELING_OWNER_ID（或用 -Group 指定群号）"
        exit 1
    }
    $json = '{"message_type":"private","user_id":' + $owner + ',"message":[' + $seg + ']}'
}

# 以 UTF-8（无 BOM）写临时文件，再用 curl --data @ 按字节发送，避开终端编码。
$tmp = New-TemporaryFile
[System.IO.File]::WriteAllText($tmp.FullName, $json, (New-Object System.Text.UTF8Encoding($false)))
try {
    curl -s -w "`n[http_status:%{http_code}]`n" -X POST "$url" `
        -H "Authorization: Bearer $key" `
        -H "Content-Type: application/json" `
        --data "@$($tmp.FullName)"
} finally {
    Remove-Item $tmp.FullName -Force
}
