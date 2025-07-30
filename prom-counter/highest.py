# -*- coding: utf-8 -*-
# usage: python highest.py [prometheus_url] [metric_name]

import requests
import sys
from collections import defaultdict

def get_metric_cardinality(prom_url, metric_name):
    """获取指定metric的标签基数统计"""
    
    # 查询所有匹配该 metric 的 series（标签集合）
    resp = requests.get(f"{prom_url}/api/v1/series", params={
        'match[]': metric_name
    })
    
    resp.raise_for_status()
    result = resp.json()
    
    if result["status"] != "success":
        raise Exception("Prometheus API error")
    
    series_list = result["data"]
    
    # 统计每个 label 的所有唯一值
    label_values = defaultdict(set)
    
    for series in series_list:
        for label, value in series.items():
            if label != "__name__":  # 不统计 metric 名
                label_values[label].add(value)
    
    return label_values

def print_table_header():
    """打印表格头部"""
    print("┌" + "─" * 20 + "┬" + "─" * 15 + "┬" + "─" * 50 + "┐")
    print("│ Label Name          │ Cardinality    │ Sample Values                    │")
    print("├" + "─" * 20 + "┼" + "─" * 15 + "┼" + "─" * 50 + "┤")

def print_table_row(label, cardinality, sample_values):
    """打印表格行"""
    # 格式化基数
    cardinality_str = f"{cardinality:,}"
    
    # 格式化样本值
    if len(sample_values) <= 3:
        values_str = ", ".join(sample_values)
    else:
        values_str = ", ".join(sample_values[:2]) + f" ... ({len(sample_values)-4} more) ... " + ", ".join(sample_values[-2:])
    
    # 确保每列不超过指定宽度
    label_str = label[:18] + ".." if len(label) > 18 else label.ljust(18)
    cardinality_str = cardinality_str.ljust(13)
    values_str = values_str[:48] + ".." if len(values_str) > 48 else values_str.ljust(48)
    
    print(f"│ {label_str} │ {cardinality_str} │ {values_str} │")

def print_table_footer():
    """打印表格底部"""
    print("└" + "─" * 20 + "┴" + "─" * 15 + "┴" + "─" * 50 + "┘")

def main():
    # 默认值
    default_prom_url = "http://192.168.5.200:9090"
    default_metric_name = "etcd_request_duration_seconds_bucket"
    
    # 从命令行参数获取URL和metric名称
    if len(sys.argv) >= 2:
        prom_url = sys.argv[1]
    else:
        prom_url = default_prom_url
    
    if len(sys.argv) >= 3:
        metric_name = sys.argv[2]
    else:
        metric_name = default_metric_name
    
    print(f"Prometheus URL: {prom_url}")
    print(f"Metric Name: {metric_name}")
    print("-" * 85)
    
    try:
        label_values = get_metric_cardinality(prom_url, metric_name)
        
        if not label_values:
            print("No labels found for this metric.")
            return
        
        # 打印表格头部
        print_table_header()
        
        # 按基数大小排序
        sorted_labels = sorted(label_values.items(), key=lambda x: len(x[1]), reverse=True)
        
        for label, values in sorted_labels:
            cardinality = len(values)
            sorted_values = sorted(values)
            
            # 选择样本值
            if cardinality <= 5:
                sample_values = sorted_values
            elif cardinality <= 10:
                sample_values = sorted_values[:5] + ["..."] + sorted_values[-2:]
            else:
                sample_values = sorted_values[:3] + ["..."] + sorted_values[-3:]
            
            print_table_row(label, cardinality, sample_values)
        
        # 打印表格底部
        print_table_footer()
        
        # 打印统计信息
        total_labels = len(label_values)
        total_cardinality = sum(len(values) for values in label_values.values())
        max_cardinality = max(len(values) for values in label_values.values())
        
        print(f"\n📊 Summary:")
        print(f"   Total Labels: {total_labels}")
        print(f"   Total Cardinality: {total_cardinality:,}")
        print(f"   Max Cardinality: {max_cardinality:,}")
        
        # 高基数警告
        high_cardinality_labels = [(label, len(values)) for label, values in label_values.items() if len(values) > 100]
        if high_cardinality_labels:
            print(f"\n⚠️  High Cardinality Warning:")
            for label, cardinality in high_cardinality_labels:
                print(f"   {label}: {cardinality:,} values")
            
    except Exception as e:
        print(f"Error: {e}")
        sys.exit(1)

if __name__ == "__main__":
    main()