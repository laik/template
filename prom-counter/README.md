# Prometheus Metric Cardinality Analyzer

这个脚本用于分析 Prometheus 中指定 metric 的标签基数（cardinality）。

## 功能特性

- 统计每个标签的唯一值数量（基数）
- 按基数大小排序显示结果
- 支持动态传入 Prometheus URL 和 metric 名称
- 采用清晰的表格样式输出
- 智能显示样本值，避免信息过载
- 提供统计摘要和高基数警告

## 使用方法

### 基本用法（使用默认值）
```bash
python highest.py
```

### 指定 Prometheus URL
```bash
python highest.py http://your-prometheus-server:9090
```

### 指定 Prometheus URL 和 Metric 名称
```bash
python highest.py http://your-prometheus-server:9090 your_metric_name
```

## 使用示例

```bash
# 分析 etcd 请求持续时间指标
python highest.py http://192.168.5.200:9090 etcd_request_duration_seconds_bucket

# 分析 HTTP 请求总数指标
python highest.py http://192.168.5.200:9090 http_requests_total

# 分析其他自定义指标
python highest.py http://192.168.5.200:9090 node_cpu_seconds_total
```

## 输出说明

脚本会以表格形式显示每个标签的基数统计：

```
┌────────────────────┬───────────────┬──────────────────────────────────────────────────┐
│ Label Name          │ Cardinality    │ Sample Values                    │
├────────────────────┼───────────────┼──────────────────────────────────────────────────┤
│ type               │ 197           │ /registry/acme.cert-manager.io/challenges/, /reg.. │
│ le                 │ 10            │ +Inf, 0.005 ... (4 more) ... 5.0, 60.0           │
│ operation          │ 6             │ create, delete ... (4 more) ... listWithCount, u.. │
│ endpoint           │ 1             │ https                                            │
└────────────────────┴───────────────┴──────────────────────────────────────────────────┘
```

### 表格列说明

- **Label Name**: 标签名称
- **Cardinality**: 该标签的唯一值数量（基数）
- **Sample Values**: 样本值展示
  - 基数 ≤ 5：显示所有值
  - 基数 ≤ 10：显示前5个和后2个值
  - 基数 > 10：显示前3个和后3个值

### 统计摘要

脚本还会显示：
- 总标签数量
- 总基数和
- 最大基数
- 高基数警告（基数 > 100 的标签）

## 注意事项

- 确保 Prometheus 服务器可访问
- 确保指定的 metric 名称存在
- 高基数字段可能会影响 Prometheus 性能，建议监控和优化
- 表格会自动截断过长的值以保持格式整洁 