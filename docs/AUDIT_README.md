# API 审计功能使用说明

## 概述

SlimRAG 现在支持对 OpenAI API 调用进行审计日志记录。当使用 `--trace` 标志时，所有的 API 请求和响应都会被忠实地记录到 Markdown 格式的文件中。

## 使用方法

### 基本用法

```bash
# 启用审计日志记录
srag ask --trace "你的查询"

# 指定审计日志目录
srag ask --trace --audit-log-dir ./my_audit_logs "你的查询"

# 结合其他选项使用
srag ask --trace --retrieval-limit 50 --selected-limit 15 "你的查询"
```

### 审计日志格式

审计日志以 Markdown 格式存储，包含以下信息：

- **时间戳**: API 调用的精确时间
- **API 类型**: embeddings 或 chat
- **模型名称**: 使用的 OpenAI 模型
- **持续时间**: API 调用耗时
- **请求 ID**: OpenAI 返回的请求标识符
- **请求内容**: 完整的 API 请求参数
- **响应内容**: API 返回的完整响应
- **错误信息**: 如果调用失败，包含错误详情

### 示例审计日志

```markdown
# API Call Audit Log

**Timestamp:** 2025-08-09 13:58:17.000  
**API Type:** embeddings  
**Model:** text-embedding-3-small  
**Duration:** 145ms  
**Request ID:** req_123456789  

## Request

```json
{
  "model": "text-embedding-3-small",
  "input": "测试查询文本",
  "dimensions": 1536,
  "encoding_format": "float"
}
```

## Response

```json
{
  "data": [
    {
      "embedding": [0.1, 0.2, 0.3, ...],
      "index": 0,
      "object": "embedding"
    }
  ],
  "model": "text-embedding-3-small",
  "usage": {
    "prompt_tokens": 5,
    "total_tokens": 5
  }
}
```
```

## 文件存储

- 默认存储位置: `./audit_logs/`
- 文件命名格式: `api_call_{api_type}_{timestamp}.md`
- 例如: `api_call_embeddings_1754719097957981756.md`

## 隐私保护

为了保护用户隐私，审计日志会对消息内容进行脱敏处理：

- 用户消息: 记录为 `[Content length: X chars]`
- 助手消息: 记录为 `[Content length: X chars]`
- 系统消息: 记录为 `[Content length: X chars]`

## 性能考虑

- 审计日志记录会增加少量开销（通常 < 1ms）
- 日志文件大小取决于 API 响应的大小
- 建议在生产环境中定期清理或归档旧的日志文件

## 故障排除

### 常见问题

1. **审计日志未生成**
   - 确保使用了 `--trace` 标志
   - 检查文件系统权限
   - 验证 `--audit-log-dir` 指定的目录是否存在且可写

2. **日志文件为空**
   - 检查 API 调用是否成功
   - 查看应用程序日志中的错误信息

3. **磁盘空间不足**
   - 定期清理审计日志目录
   - 考虑使用日志轮转策略

## 集成示例

### 在 CI/CD 中使用

```bash
#!/bin/bash
# 在 CI/CD 管道中启用审计
export RAG_TRACE=true
export RAG_AUDIT_LOG_DIR=./ci_audit_logs

srag ask --trace "测试查询"
```

### 在脚本中批量处理

```bash
#!/bin/bash
# 批量处理查询并记录审计日志
queries=("查询1" "查询2" "查询3")

for query in "${queries[@]}"; do
    srag ask --trace --audit-log-dir "./batch_audit_$(date +%Y%m%d)" "$query"
done
```

## 总结

审计功能为 SlimRAG 提供了完整的 API 调用透明度，帮助用户：

1. **调试问题**: 通过查看完整的请求/响应来诊断 API 调用问题
2. **监控使用**: 跟踪 API 调用频率和性能
3. **成本控制**: 通过监控 token 使用来优化成本
4. **合规要求**: 满足某些场景下的审计和日志记录要求

使用 `--trace` 标志即可轻松启用此功能。