#!/usr/bin/env python3
"""
Simple example demonstrating PromQL series extraction
"""

from promql_series_extractor import extract_series_from_promql, extract_series_simple_regex


def main():
    # Example PromQL queries
    examples = [
        'cpu_usage_percent',
        'http_requests_total{method="GET"}',
        'rate(http_requests_total[5m])',
        'sum(rate(cpu_usage_seconds_total[5m])) by (mode)',
        'up{job="prometheus"} == 1',
        'rate(http_requests_total[5m]) / rate(http_requests_duration_seconds_count[5m])',
        'histogram_quantile(0.95, sum(rate(http_request_duration_seconds_bucket[5m])) by (le))',
    ]
    
    print("PromQL Series Extraction Examples")
    print("=" * 50)
    
    for query in examples:
        print(f"\nQuery: {query}")
        
        # Extract using promql-parser
        series = extract_series_from_promql(query)
        if series:
            print(f"Series found: {', '.join(series)}")
        else:
            # Fallback to regex
            series_regex = extract_series_simple_regex(query)
            print(f"Series found (regex): {', '.join(series_regex)}")


if __name__ == "__main__":
    main()