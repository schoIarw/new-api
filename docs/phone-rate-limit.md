# 分组手机号限流

模型限流仍使用 `ModelRequestRateLimitGroup`，手机号策略独立保存于 `PhoneRateLimitPolicies`，无需新增数据表。

```json
{
  "group1": {"default": [10, 5], "special": {"18946512326": [100, 50]}},
  "group2": {"default": [20, 10], "special": {"18946512326": [5, 2]}}
}
```

- 每组每手机号独立计数。特殊号覆盖同组默认值；[0,0] 表示该层不限制。组/模型原有限制依旧同时生效。
- 未配置新手机号策略的分组沿用旧 `ModelRequestRateLimitGroup` 顶层号码数组；一旦配置新策略，旧全局号码规则不再作用于此组。
- 新策略要求从请求体 `user` 或请求头 `X-User-Id` 获取完整有效的 11 位中国大陆手机号；请求体优先。**必须由可信业务后端提供身份，不能信任终端自行填写的手机号。**
- Redis 使用固定周期和 Lua 原子准入计数：总请求含失败请求，成功请求保留名额直到请求完成，失败退还完成名额；内存模式仅单进程计数，多副本必须共用 Redis。
- 手机号仅以 HMAC 标识出现在 Redis Key；变更 CRYPTO_SECRET 会导致新计数空间。新旧限流 Key 不共用，部署切换时新策略计数从零开始。
- 对流式响应，HTTP 状态码已经提交为 2xx 后发生的流内错误无法可靠地识别为失败，计数遵循原系统 HTTP 状态码口径。
- 全局开关 `ModelRequestRateLimitEnabled` 以及全局周期 `ModelRequestRateLimitDurationMinutes` 仍决定本功能是否运行及周期长度。
