# 使用 Secret 自签证书管理 node-exporter 证书

自 v1.6.0 起，Prometheus Node Exporter 官方已原生支持通过 `--web.config.file` 参数加载 TLS 与 Basic Auth 配置，不需要再使用 nginx 或 proxy sidecar。

✅ 最佳实现：使用自签证书 + Node Exporter 原生 TLS & Basic Auth 支持

💡 总体思路

我们将：
1. 手动创建自签名证书并保存为 Kubernetes Secret
2. 准备一个 `web.config.yaml` 文件，配置 TLS 和 Basic Auth
3. 使用 Node Exporter 参数 `--web.config.file=/etc/node_exporter/web.config.yaml` 启用认证和 HTTPS
4. Prometheus 中配置 Basic Auth + TLS Scrape

🔧 步骤详解

🧩 第一步：生成自签名证书并创建 Secret

首先在本地生成自签名证书：

```bash
# 1. 生成 CA 私钥
openssl genrsa -out ca.key 4096

# 2. 生成 CA 证书（自签）
openssl req -x509 -new -nodes -key ca.key -sha256 -days 3650 -out ca.crt \
  -subj "/C=CN/ST=Shanghai/O=MyOrg, Inc./CN=MyPrometheusCA"

# 3. 生成服务器私钥
openssl genrsa -out node-exporter.key 2048

# 4. 创建证书签名请求配置文件
cat > server.csr.cnf << "EOF"
[req]
default_bits = 2048
prompt = no
default_md = sha256
req_extensions = req_ext
distinguished_name = dn

[dn]
C = CN
ST = Shanghai
O = MyOrg, Inc.
CN = node-exporter.ske-system.svc

[req_ext]
subjectAltName = @alt_names

[alt_names]
DNS.1 = node-exporter
DNS.2 = node-exporter.ske-system
DNS.3 = node-exporter.ske-system.svc
DNS.4 = node-exporter.ske-system.svc.cluster.local
EOF

# 5. 生成证书签名请求
openssl req -new -key node-exporter.key -out tls.csr -config server.csr.cnf

# 6. 创建证书扩展配置文件
cat > ca.ext << "EOF"
authorityKeyIdentifier=keyid,issuer
basicConstraints=CA:FALSE
keyUsage = digitalSignature, nonRepudiation, keyEncipherment, dataEncipherment
extendedKeyUsage = serverAuth
subjectAltName = @alt_names

[alt_names]
DNS.1 = node-exporter
DNS.2 = node-exporter.ske-system
DNS.3 = node-exporter.ske-system.svc
DNS.4 = node-exporter.ske-system.svc.cluster.local
EOF

# 7. 使用 CA 签名生成最终证书
openssl x509 -req -in tls.csr -CA ca.crt -CAkey ca.key -CAcreateserial \
  -out node-exporter.crt -days 365 -sha256 -extfile ca.ext

# 8. 清理临时文件
rm -f tls.csr server.csr.cnf ca.ext ca.srl

# 9. 创建 Kubernetes Secret
kubectl delete secret node-exporter-cert-secret -n ske-system --ignore-not-found=true
kubectl create secret tls node-exporter-cert-secret \
  --cert=node-exporter.crt \
  --key=node-exporter.key \
  -n ske-system
```

🔐 第二步：创建 Basic Auth 密码文件（htpasswd）

```bash
htpasswd -nbB admin secret
# 输出：admin:$2y$05$apbMB8B8.41BGhQKGla.IO.H3wd.4LpJY.UCsoBAAuPenuNRHj08y

echo -n "admin:secret" | base64
# 输出：YWRtaW46c2VjcmV0


kubectl delete secret node-exporter-basic-auth
kubectl create secret generic node-exporter-basic-auth \
  --from-literal=username=admin \
  --from-literal=password=$2y$05$apbMB8B8.41BGhQKGla.IO.H3wd.4LpJY.UCsoBAAuPenuNRHj08y \
  -n ske-system

```

📄 第三步：创建 web.config.yaml 配置文件为 ConfigMap

创建 `web-config.yaml` 文件，内容如下：

```bash
# 生成 bcrypt 哈希（密码：secret）
htpasswd -nbB admin secret
# 输出：admin:$2y$05$apbMB8B8.41BGhQKGla.IO.H3wd.4LpJY.UCsoBAAuPenuNRHj08y

# 创建 web.config.yaml 文件
cat > web-config.yaml << "EOF"
tls_server_config:
  cert_file: /etc/node_exporter/tls/tls.crt
  key_file: /etc/node_exporter/tls/tls.key
basic_auth_users:
  admin: $2y$05$apbMB8B8.41BGhQKGla.IO.H3wd.4LpJY.UCsoBAAuPenuNRHj08y
EOF

# 创建 ConfigMap
kubectl delete configmap node-exporter-web-config -n ske-system --ignore-not-found=true
kubectl create configmap node-exporter-web-config --from-file=web.config.yaml=web-config.yaml -n ske-system
```

📦 第四步：在 Node Exporter 中挂载证书 & config

如果使用 Helm 安装 kube-prometheus-stack，在 `values.yaml` 中添加如下内容：

```yaml
ref: values
```

⚠️ 注意：mountPath 为文件时需 subPath。

 Helm 更新操作：

保存上述配置到 `values.yaml` 文件后，使用以下命令更新 Node Exporter：

```bash
# 如果是首次安装
helm install monitoring-operator prometheus-community/kube-prometheus-stack -f values.yaml

# 如果已经安装，需要更新配置
helm upgrade monitoring-operator prometheus-community/kube-prometheus-stack -f values.yaml
```

看查看和确认 Helm 更新状态：

```bash
# 查看 Helm release 状态
helm status monitoring-operator

# 查看 Node Exporter Pod 状态
kubectl get pods -n ske-system -l app.kubernetes.io/name=prometheus-node-exporter

# 查看 Node Exporter Pod 详细信息
kubectl describe pod -n ske-system -l app.kubernetes.io/name=prometheus-node-exporter

# 查看 Node Exporter 日志
kubectl logs -n ske-system -l app.kubernetes.io/name=prometheus-node-exporter
```

确保 Pod 正常运行且没有错误日志。

🔍 验证步骤

```bash
# 1. 检查 pod 状态
kubectl get pods -n ske-system -l app.kubernetes.io/name=prometheus-node-exporter

# 2. 检查证书 secret
kubectl get secret node-exporter-cert-secret -n ske-system

# 3. 检查 web config
kubectl get configmap node-exporter-web-config -n ske-system

# 4. 检查日志（应该没有 bcrypt 错误）
kubectl logs -n ske-system -l app.kubernetes.io/name=prometheus-node-exporter --tail=5

# 5. 测试访问
kubectl port-forward -n ske-system svc/monitoring-operator-prometheus-node-exporter 9100:9100 &
sleep 3
curl -k -u admin:secret https://localhost:9100/metrics | head -5
pkill -f "kubectl port-forward"
```

📡 第五步：修改 ServiceMonitor 支持 Basic Auth + TLS

创建 `servicemonitor.yaml` 文件：

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: node-exporter
  namespace: ske-system
spec:
  endpoints:
    - port: http-metrics
      scheme: https
      path: /metrics
      tlsConfig:
        insecureSkipVerify: true
      basicAuth:
        username:
          name: node-exporter-basic-auth
          key: username
        password:
          name: node-exporter-basic-auth
          key: password
  selector:
    matchLabels:
      app.kubernetes.io/name: prometheus-node-exporter
  namespaceSelector:
    matchNames:
      - ske-system
```

注意：ServiceMonitor 会自动创建，不需要手动创建。如果需要自定义，可以创建上述文件。


✅ 成功标志
• 你可以访问 Node Exporter 的端口（如 `curl -k -u admin:secret https://<podIP>:9100/metrics`）
• Prometheus Targets 页面 Node Exporter 状态为 UP

```bash
# 测试 TLS + Basic Auth
curl -k -u admin:secret https://localhost:9100/metrics

# 测试 TLS（应该返回 401 Unauthorized）
curl -k https://localhost:9100/metrics

# 端口转发测试
kubectl port-forward -n ske-system svc/monitoring-operator-prometheus-node-exporter 9100:9100 &
curl -k -u admin:secret https://localhost:9100/metrics
```

🔧 故障排除

### 常见问题

1. **bcrypt 错误**：
   ```
   crypto/bcrypt: bcrypt hashes must start with '$', but hashedSecret started with 'a'
   ```
   **解决方案**：确保 `basic_auth_users` 中只包含 bcrypt 哈希值，不包含用户名前缀。

2. **健康检查失败**：
   ```
   Readiness probe failed: HTTP probe failed with statuscode: 401
   ```
   **解决方案**：检查 Authorization header 是否正确，应该是 `admin:secret` 的 base64 编码。

3. **证书错误**：
   ```
   TLS handshake error: client sent an HTTP request to an HTTPS server
   ```
   **解决方案**：这是正常的，说明 TLS 正在工作，客户端需要使用 HTTPS。

4. **Helm 升级不生效**：
   **解决方案**：删除所有 pod 让它们重新创建：
   ```bash
   kubectl delete pods -n ske-system -l app.kubernetes.io/name=prometheus-node-exporter
   ```

### 验证命令

```bash
# 检查 web.config.yaml 内容
kubectl get configmap node-exporter-web-config -n ske-system -o jsonpath='{.data.web\.config\.yaml}'

# 检查 pod 中的 Authorization header
kubectl get pod -n ske-system -l app.kubernetes.io/name=prometheus-node-exporter -o yaml | grep -A 2 "Authorization"

# 测试 Basic Auth
curl -k -u admin:secret https://localhost:9100/metrics
```

🧰 总结

| 功能 | 是否支持 | 实现方式 |
|------|---------|---------|
| TLS 加密 | ✅ | Node Exporter 原生 via web.config.yaml |
| Basic Auth | ✅ | 内建配置 |
| 自签证书管理 | ✅ | 手动创建并存储为 Secret |
| Helm 支持 | ✅ | extraArgs, extraVolumes, extraVolumeMounts |

---

## 🔍 完善的操作步骤验证

### 📋 预检查清单

在开始配置之前，请确保以下环境已准备就绪：

```bash
# 1. 检查 Kubernetes 集群状态
kubectl cluster-info
kubectl get nodes

# 2. 检查命名空间是否存在
kubectl get namespace ske-system
# 如果不存在，创建命名空间
kubectl create namespace ske-system

# 3. 检查 Helm 是否已安装
helm version

# 4. 检查必要的工具是否可用
which openssl
which htpasswd
which curl
```

### 🚀 完整验证流程

#### 第一步：证书和密钥验证

```bash
# 验证证书文件是否存在且有效
ls -la node-exporter.crt node-exporter.key ca.crt ca.key

# 验证证书内容
openssl x509 -in node-exporter.crt -text -noout | grep -E "(Subject:|DNS:|Validity)"

# 验证私钥
openssl rsa -in node-exporter.key -check -noout

# 验证证书链
openssl verify -CAfile ca.crt node-exporter.crt
```

#### 第二步：Kubernetes 资源验证

```bash
# 验证 Secret 是否正确创建
kubectl get secret node-exporter-cert-secret -n ske-system -o yaml

# 验证 Secret 中的证书数据
kubectl get secret node-exporter-cert-secret -n ske-system -o jsonpath='{.data.tls\.crt}' | base64 -d | openssl x509 -text -noout

# 验证 Basic Auth Secret
kubectl get secret node-exporter-basic-auth -n ske-system -o yaml

# 验证 ConfigMap
kubectl get configmap node-exporter-web-config -n ske-system -o yaml
```

#### 第三步：Helm 部署验证

```bash
# 检查 Helm release 状态
helm list -n ske-system
helm status monitoring-operator -n ske-system

# 检查 Node Exporter Pod 状态
kubectl get pods -n ske-system -l app.kubernetes.io/name=prometheus-node-exporter

# 检查 Pod 详细信息
kubectl describe pod -n ske-system -l app.kubernetes.io/name=prometheus-node-exporter

# 检查 Pod 中的挂载点
kubectl exec -n ske-system -l app.kubernetes.io/name=prometheus-node-exporter -- ls -la /etc/node_exporter/

# 检查 web.config.yaml 是否正确挂载
kubectl exec -n ske-system -l app.kubernetes.io/name=prometheus-node-exporter -- cat /etc/node_exporter/web.config.yaml
```

#### 第四步：服务验证

```bash
# 检查 Service 是否正确创建
kubectl get svc -n ske-system -l app.kubernetes.io/name=prometheus-node-exporter

# 检查 Service 详细信息
kubectl describe svc -n ske-system -l app.kubernetes.io/name=prometheus-node-exporter

# 检查 Endpoints
kubectl get endpoints -n ske-system -l app.kubernetes.io/name=prometheus-node-exporter
```

#### 第五步：功能验证

```bash
# 1. 端口转发测试
kubectl port-forward -n ske-system svc/monitoring-operator-prometheus-node-exporter 9100:9100 &
PF_PID=$!

# 等待端口转发建立
sleep 3

# 2. 测试 HTTPS 访问（应该返回 401）
echo "测试 HTTPS 无认证访问（应该返回 401）："
curl -k -s -o /dev/null -w "%{http_code}" https://localhost:9100/metrics

# 3. 测试 Basic Auth 访问
echo "测试 Basic Auth 访问："
curl -k -u admin:secret https://localhost:9100/metrics | head -10

# 4. 测试错误的密码
echo "测试错误密码（应该返回 401）："
curl -k -u admin:wrongpassword https://localhost:9100/metrics -s -o /dev/null -w "%{http_code}"

# 5. 测试 HTTP 访问（应该失败）
echo "测试 HTTP 访问（应该失败）："
curl -s http://localhost:9100/metrics -o /dev/null -w "%{http_code}" || echo "连接被拒绝"

# 清理端口转发
kill $PF_PID
```

#### 第六步：Prometheus 集成验证

```bash
# 检查 ServiceMonitor 是否正确创建
kubectl get servicemonitor -n ske-system

# 检查 Prometheus 配置
kubectl get prometheus -n ske-system

# 检查 Prometheus Pod 状态
kubectl get pods -n ske-system -l app.kubernetes.io/name=prometheus

# 检查 Prometheus 配置是否包含 Node Exporter
kubectl port-forward -n ske-system svc/monitoring-operator-prometheus 9090:9090 &
sleep 3

# 访问 Prometheus UI 检查 Targets
echo "请在浏览器中访问 http://localhost:9090 并检查 Targets 页面"
echo "Node Exporter 应该显示为 UP 状态"

# 检查 Prometheus 配置
curl -s http://localhost:9090/api/v1/targets | jq '.data.activeTargets[] | select(.labels.job == "node-exporter")'

kill %1
```

#### 第七步：日志和监控验证

```bash
# 检查 Node Exporter 日志
echo "=== Node Exporter 日志 ==="
kubectl logs -n ske-system -l app.kubernetes.io/name=prometheus-node-exporter --tail=20

# 检查是否有错误
kubectl logs -n ske-system -l app.kubernetes.io/name=prometheus-node-exporter | grep -i error

# 检查 Prometheus 日志
echo "=== Prometheus 日志 ==="
kubectl logs -n ske-system -l app.kubernetes.io/name=prometheus --tail=20

# 检查 Prometheus Operator 日志
echo "=== Prometheus Operator 日志 ==="
kubectl logs -n ske-system -l app.kubernetes.io/name=prometheus-operator --tail=20
```

### 🔧 高级故障排除

#### 证书相关问题

```bash
# 检查证书是否过期
openssl x509 -in node-exporter.crt -noout -dates

# 检查证书的 SAN 扩展
openssl x509 -in node-exporter.crt -noout -text | grep -A 10 "Subject Alternative Name"

# 验证证书与私钥匹配
openssl x509 -noout -modulus -in node-exporter.crt | openssl md5
openssl rsa -noout -modulus -in node-exporter.key | openssl md5
```

#### 网络连接问题

```bash
# 检查 Pod 网络连接
kubectl exec -n ske-system -l app.kubernetes.io/name=prometheus-node-exporter -- curl -k -u admin:secret https://localhost:9100/metrics

# 检查 Service 到 Pod 的连接
kubectl exec -n ske-system -l app.kubernetes.io/name=prometheus-node-exporter -- curl -k -u admin:secret https://node-exporter.ske-system.svc:9100/metrics

# 检查 DNS 解析
kubectl exec -n ske-system -l app.kubernetes.io/name=prometheus-node-exporter -- nslookup node-exporter.ske-system.svc
```

#### 配置验证

```bash
# 验证 web.config.yaml 语法
kubectl exec -n ske-system -l app.kubernetes.io/name=prometheus-node-exporter -- cat /etc/node_exporter/web.config.yaml

# 验证 Node Exporter 参数
kubectl exec -n ske-system -l app.kubernetes.io/name=prometheus-node-exporter -- ps aux | grep node_exporter

# 验证挂载点
kubectl exec -n ske-system -l app.kubernetes.io/name=prometheus-node-exporter -- mount | grep node_exporter
```

### 📊 性能监控

```bash
# 检查 Node Exporter 指标收集
curl -k -u admin:secret https://localhost:9100/metrics | grep -E "(node_exporter_build_info|node_exporter_scrape_duration_seconds)"

# 检查内存使用
kubectl top pods -n ske-system -l app.kubernetes.io/name=prometheus-node-exporter

# 检查 CPU 使用
kubectl exec -n ske-system -l app.kubernetes.io/name=prometheus-node-exporter -- cat /proc/cpuinfo | grep processor | wc -l
```

### 🎯 成功标准检查清单

完成以下所有检查项表示配置成功：

- [ ] 证书和密钥文件存在且有效
- [ ] Kubernetes Secret 和 ConfigMap 正确创建
- [ ] Node Exporter Pod 处于 Running 状态
- [ ] 健康检查通过（liveness 和 readiness probe）
- [ ] HTTPS 访问需要认证（返回 401）
- [ ] Basic Auth 认证成功（返回指标数据）
- [ ] HTTP 访问被拒绝
- [ ] Prometheus Targets 页面显示 Node Exporter 为 UP
- [ ] 没有错误日志
- [ ] 指标数据正常收集

### 🚨 紧急恢复步骤

如果配置出现问题，可以按以下步骤恢复：

```bash
# 1. 删除所有相关资源
kubectl delete pods -n ske-system -l app.kubernetes.io/name=prometheus-node-exporter
kubectl delete secret node-exporter-cert-secret -n ske-system
kubectl delete secret node-exporter-basic-auth -n ske-system
kubectl delete configmap node-exporter-web-config -n ske-system

# 2. 重新创建资源
# （重新执行第一步到第三步的创建命令）

# 3. 重新部署 Helm
helm upgrade monitoring-operator prometheus-community/kube-prometheus-stack -f values.yaml

# 4. 验证恢复
kubectl get pods -n ske-system -l app.kubernetes.io/name=prometheus-node-exporter
```

### 📝 配置备份

建议定期备份重要配置：

```bash
# 备份证书和密钥
cp node-exporter.crt node-exporter.crt.backup
cp node-exporter.key node-exporter.key.backup
cp ca.crt ca.crt.backup
cp ca.key ca.key.backup

# 备份 Kubernetes 资源
kubectl get secret node-exporter-cert-secret -n ske-system -o yaml > node-exporter-cert-secret.yaml
kubectl get secret node-exporter-basic-auth -n ske-system -o yaml > node-exporter-basic-auth.yaml
kubectl get configmap node-exporter-web-config -n ske-system -o yaml > node-exporter-web-config.yaml

# 备份 values.yaml
cp values.yaml values.yaml.backup
```

这样，你就有了一个完整的、可验证的 Node Exporter TLS + Basic Auth 配置方案！




kubectl port-forward -n ske-system svc/monitoring-operator-kube-p-prometheus --address 0.0.0.0 9090:9090
curl -s "http://192.168.5.200:9090/api/v1/targets" | jq -r '.data.activeTargets[] | select(.labels.job == "node-exporter") | "\(.labels.job) - \(.labels.instance) - \(.health) - \(.lastError)"'
