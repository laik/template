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
openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
  -keyout node-exporter.key \
  -out node-exporter.crt \
  -subj "/CN=node-exporter.${NAME_SPACE}.svc"
```

然后创建 Kubernetes Secret：

```bash
kubectl create secret tls node-exporter-cert-secret \
  --cert=node-exporter.crt \
  --key=node-exporter.key \
  -n ${NAME_SPACE}
```

🔐 第二步：创建 Basic Auth 密码文件（htpasswd）

```bash
htpasswd -nbB admin secret
# 输出形如： admin:$apr1$uKreR...（保留它）
```

将这个内容保存到 Secret：

```bash
kubectl delete secret node-exporter-basic-auth
kubectl create secret generic node-exporter-basic-auth \
  --from-literal=auth='admin:$2y$05$SJKoEVgjsw0PCvGBB6fkxOuEClOG5Pn8dMMUrJQ1SlQonXOzTAzdq' \
  -n ${NAME_SPACE}
```

📄 第三步：创建 web.config.yaml 配置文件为 ConfigMap

创建 `node-exporter-web-config.yaml` 文件 ,内容如下：

```bash
kubectl delete configmap node-exporter-web-config
cat << "EOF" | kubectl apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: node-exporter-web-config
  namespace: ${NAME_SPACE}
data:
  web.config.yaml: |
    tls_server_config:
      cert_file: /etc/node_exporter/tls/tls.crt
      key_file: /etc/node_exporter/tls/tls.key
EOF
```

if need add basic auth
```
    basic_auth_users:
      admin: "$2y$05$SJKoEVgjsw0PCvGBB6fkxOuEClOG5Pn8dMMUrJQ1SlQonXOzTAzdq"
```

📦 第四步：在 Node Exporter 中挂载证书 & config

如果使用 Helm 安装 kube-prometheus-stack，在 `values.yaml` 中添加如下内容：

```yaml
prometheus-node-exporter:
  namespaceOverride: ""
  podLabels:
    ## Add the 'node-exporter' label to be used by serviceMonitor to match standard common usage in rules and grafana dashboards
    ##
    jobLabel: node-exporter
  releaseLabel: true
  ## Liveness probe
  # TODO： 0729
  livenessProbe:
    failureThreshold: 3
    httpGet:
      httpHeaders: []
      scheme: https
    initialDelaySeconds: 0
    periodSeconds: 10
    successThreshold: 1
    timeoutSeconds: 1

  ## Readiness probe
  # TODO： 0729
  readinessProbe:
    failureThreshold: 3
    httpGet:
      httpHeaders: []
      scheme: https
    initialDelaySeconds: 0
    periodSeconds: 10
    successThreshold: 1
    timeoutSeconds: 1

  extraArgs:
    - --collector.filesystem.mount-points-exclude=^/(dev|proc|sys|var/lib/docker/.+|var/lib/kubelet/.+)($|/)
    - --collector.filesystem.fs-types-exclude=^(autofs|binfmt_misc|bpf|cgroup2?|configfs|debugfs|devpts|devtmpfs|fusectl|hugetlbfs|iso9660|mqueue|nsfs|overlay|proc|procfs|pstore|rpc_pipefs|securityfs|selinuxfs|squashfs|sysfs|tracefs|erofs)$
    # TODO： 0729
    - '--web.config.file=/etc/node_exporter/web.config.yaml'
  extraVolumeMounts:
    - name: tls
      mountPath: /etc/node_exporter/tls
      readOnly: true
    - name: web-config
      mountPath: /etc/node_exporter/web.config.yaml
      subPath: web.config.yaml
      readOnly: true
  extraVolumes:
    - name: tls
      secret:
        secretName: node-exporter-cert-secret
    - name: web-config
      configMap:
        name: node-exporter-web-config
    # TODO： 0729
  service:
    portName: http-metrics
    ipDualStack:
      enabled: false
      ipFamilies: ["IPv6", "IPv4"]
      ipFamilyPolicy: "PreferDualStack"
    labels:
      jobLabel: node-exporter

  prometheus:
    monitor:
      enabled: true
      # TODO： 0729 
      scheme: https

      jobLabel: jobLabel

      ## Scrape interval. If not set, the Prometheus default scrape interval is used.
      ##
      interval: ""

      ## SampleLimit defines per-scrape limit on number of scraped samples that will be accepted.
      ##
      sampleLimit: 0

      ## TargetLimit defines a limit on the number of scraped targets that will be accepted.
      ##
      targetLimit: 0

      ## Per-scrape limit on number of labels that will be accepted for a sample. Only valid in Prometheus versions 2.27.0 and newer.
      ##
      labelLimit: 0

      ## Per-scrape limit on length of labels name that will be accepted for a sample. Only valid in Prometheus versions 2.27.0 and newer.
      ##
      labelNameLengthLimit: 0

      ## Per-scrape limit on length of labels value that will be accepted for a sample. Only valid in Prometheus versions 2.27.0 and newer.
      ##
      labelValueLengthLimit: 0

      ## How long until a scrape request times out. If not set, the Prometheus default scape timeout is used.
      ##
      scrapeTimeout: ""

      ## proxyUrl: URL of a proxy that should be used for scraping.
      ##
      proxyUrl: ""

      ## MetricRelabelConfigs to apply to samples after scraping, but before ingestion.
      ## ref: https://github.com/prometheus-operator/prometheus-operator/blob/main/Documentation/api-reference/api.md#relabelconfig
      ##
      metricRelabelings: []
      # - sourceLabels: [__name__]
      #   separator: ;
      #   regex: ^node_mountstats_nfs_(event|operations|transport)_.+
      #   replacement: $1
      #   action: drop

      ## RelabelConfigs to apply to samples before scraping
      ## ref: https://github.com/prometheus-operator/prometheus-operator/blob/main/Documentation/api-reference/api.md#relabelconfig
      ##
      relabelings: []
      # - sourceLabels: [__meta_kubernetes_pod_node_name]
      #   separator: ;
      #   regex: ^(.*)$
      #   targetLabel: nodename
      #   replacement: $1
      #   action: replace

      ## Attach node metadata to discovered targets. Requires Prometheus v2.35.0 and above.
      ##
      # attachMetadata:
      #   node: false

  rbac:
    ## If true, create PSPs for node-exporter
    ##
    pspEnabled: false

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
kubectl get pods -n ${NAME_SPACE} -l app.kubernetes.io/name=node-exporter

# 查看 Node Exporter Pod 详细信息
kubectl describe pod -n ${NAME_SPACE} -l app.kubernetes.io/name=node-exporter

# 查看 Node Exporter 日志
kubectl logs -n ${NAME_SPACE} -l app.kubernetes.io/name=node-exporter -c node-exporter
```

确保 Pod 正常运行且没有错误日志。

📡 第五步：修改 ServiceMonitor 支持 Basic Auth + TLS

创建 `servicemonitor.yaml` 文件：

```yaml
apiVersion: ${NAME_SPACE}.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: node-exporter
  namespace: ${NAME_SPACE}
spec:
  endpoints:
    # TODO 修改
    - port: https
      scheme: https
      path: /metrics
      tlsConfig:
        insecureSkipVerify: true
  selector:
    matchLabels:
      app.kubernetes.io/name: node-exporter

# 添加 Basic Auth
      basicAuth:
        username:
          name: node-exporter-auth
          key: admin
        password:
          name: node-exporter-auth
          key: password
```


创建 Prometheus 如果需要用 Basic Auth Secret：

```bash
kubectl create secret generic node-exporter-auth \
  --from-literal=username=admin \
  --from-literal=password=secret \
  -n ${NAME_SPACE}
```


✅ 成功标志
• 你可以访问 Node Exporter 的端口（如 `curl -k -u admin:secret https://<podIP>:9100/metrics`）
• Prometheus Targets 页面 Node Exporter 状态为 UP

```
# use tls + basic auth test
curl -k -u admin:secret https://192.168.5.202:9100/metrics

# use tls test
curl -k -v https://192.168.5.202:9100/metrics
```

🧰 总结

| 功能 | 是否支持 | 实现方式 |
|------|---------|---------|
| TLS 加密 | ✅ | Node Exporter 原生 via web.config.yaml |
| Basic Auth | ✅ | 内建配置 |
| 自签证书管理 | ✅ | 手动创建并存储为 Secret |
| Helm 支持 | ✅ | extraArgs, extraVolumes, extraVolumeMounts |


