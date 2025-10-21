# 增强版 Kubernetes 管理命令
sw() {
    local config_dir="/root/.kube/configs"
    
    case "$1" in
        "cls")
            local cluster="$2"
            if [ -z "$cluster" ]; then
                echo "📋 Available clusters:"
                ls -la "$config_dir" 2>/dev/null | grep "^-" | awk '{print "  " $9}'
                return 0
            fi
            
            local config_file="${config_dir}/${cluster}"
            if [ -f "$config_file" ]; then
                export KUBECONFIG="$config_file"
                echo "🎯 Switched to cluster: $cluster"
                
                # 验证连接
                if kubectl cluster-info &>/dev/null; then
                    echo "✅ Cluster connection successful"
                    kubectl config current-context
                else
                    echo "⚠️  Connected but cannot reach cluster"
                    kubectl config current-context
                fi
            else
                echo "❌ Cluster '$cluster' not found in $config_dir"
                return 1
            fi
            ;;
            
        "current")
            echo "🔗 Current cluster:"
            kubectl config current-context
            echo "📁 KUBECONFIG: $KUBECONFIG"
            ;;
            
        "list")
            echo "📋 All available clusters:"
            ls -1 "$config_dir" 2>/dev/null || echo "No clusters configured"
            ;;
            
        *)
            echo "🔧 Kubernetes Cluster Manager"
            echo ""
            echo "Usage:"
            echo "  sw cls <cluster>    # Switch to specific cluster"
            echo "  sw cls              # List all available clusters"
            echo "  sw current          # Show current cluster"
            echo "  sw list             # List all clusters"
            echo ""
            echo "Current cluster: $(kubectl config current-context 2>/dev/null || echo 'None')"
            ;;
    esac
}
