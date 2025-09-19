#!/usr/bin/env python3
"""
PromQL Series Extractor

This module uses promql-parser to parse PromQL queries and extract all accessed series (metrics).
"""

from typing import List, Set
import re
from promql_parser import parse


def extract_series_from_promql(query: str) -> List[str]:
    """
    Extract all series (metric names) from a PromQL query.
    
    Args:
        query (str): The PromQL query string
        
    Returns:
        List[str]: List of unique series names found in the query
    """
    try:
        # Parse the PromQL query
        parsed = parse(query)
        
        # Extract series from the parsed AST
        series = set()
        _extract_series_recursive(parsed, series)
        
        return sorted(list(series))
        
    except Exception as e:
        print(f"Error parsing PromQL query: {e}")
        return []


def _extract_series_recursive(node, series: Set[str]):
    """
    Recursively traverse the AST to find all metric names.
    
    Args:
        node: The current AST node
        series (Set[str]): Set to collect unique series names
    """
    # Convert node to string to inspect its structure
    node_str = str(node)
    
    # Handle different node types based on promql-parser structure
    if hasattr(node, '__class__'):
        node_type = node.__class__.__name__
        
        # VectorSelector contains the metric name
        if node_type == 'VectorSelector':
            # Extract name from the VectorSelector representation
            if 'name: Some(' in node_str:
                match = re.search(r'name: Some\(\s*"([^"]+)"', node_str)
                if match:
                    series.add(match.group(1))
        
        # MatrixSelector contains VectorSelector (vs field)
        elif node_type == 'MatrixSelector':
            # Extract from vs field in MatrixSelector
            if 'vs: VectorSelector' in node_str:
                match = re.search(r'name: Some\(\s*"([^"]+)"', node_str)
                if match:
                    series.add(match.group(1))
        
        # Call contains function calls with arguments
        elif node_type == 'Call':
            # Extract metric names from function arguments
            matches = re.findall(r'name: Some\(\s*"([^"]+)"', node_str)
            for match in matches:
                series.add(match)
        
        # BinaryExpr contains left and right expressions
        elif node_type == 'BinaryExpr':
            matches = re.findall(r'name: Some\(\s*"([^"]+)"', node_str)
            for match in matches:
                series.add(match)
        
        # AggregateExpr contains aggregation functions
        elif node_type == 'AggregateExpr':
            matches = re.findall(r'name: Some\(\s*"([^"]+)"', node_str)
            for match in matches:
                series.add(match)
    
    # Try to access internal attributes if they exist
    try:
        # For Call nodes, check args
        if hasattr(node, 'args'):
            _extract_series_recursive(node.args, series)
        
        # For MatrixSelector, check vs
        if hasattr(node, 'vs'):
            _extract_series_recursive(node.vs, series)
        
        # For BinaryExpr, check lhs and rhs
        if hasattr(node, 'lhs'):
            _extract_series_recursive(node.lhs, series)
        if hasattr(node, 'rhs'):
            _extract_series_recursive(node.rhs, series)
        
        # For AggregateExpr, check expr
        if hasattr(node, 'expr'):
            _extract_series_recursive(node.expr, series)
    except:
        pass


def extract_series_simple_regex(query: str) -> List[str]:
    """
    Simple regex-based fallback method to extract potential metric names.
    This is used as a backup when the parser fails.
    
    Args:
        query (str): The PromQL query string
        
    Returns:
        List[str]: List of potential metric names
    """
    # Pattern to match metric names (alphanumeric + underscore + colon)
    pattern = r'\b([a-zA-Z_:][a-zA-Z0-9_:]*)\s*(?:\{|\(|$|\s)'
    matches = re.findall(pattern, query)
    
    # Filter out PromQL keywords and functions
    promql_keywords = {
        'by', 'without', 'group_left', 'group_right', 'on', 'ignoring',
        'offset', 'and', 'or', 'unless', 'sum', 'min', 'max', 'avg',
        'count', 'stddev', 'stdvar', 'rate', 'irate', 'increase',
        'delta', 'idelta', 'sort', 'sort_desc', 'topk', 'bottomk',
        'quantile', 'histogram_quantile', 'label_replace', 'label_join',
        'time', 'vector', 'scalar', 'bool', 'inf', 'nan'
    }
    
    series = []
    for match in matches:
        if match.lower() not in promql_keywords and not match.isdigit():
            series.append(match)
    
    return sorted(list(set(series)))


def demo_queries():
    """
    Demonstrate the series extraction with various PromQL queries.
    """
    test_queries = [
        # Basic metric
        'cpu_usage_percent',
        
        # Metric with labels
        'http_requests_total{method="GET", status="200"}',
        
        # Rate function
        'rate(http_requests_total[5m])',
        
        # Sum aggregation
        'sum(rate(http_requests_total[5m])) by (instance)',
        
        # Binary operation
        'cpu_usage_percent > 80',
        
        # Complex query with multiple metrics
        'rate(http_requests_total[5m]) / rate(http_requests_duration_seconds_count[5m])',
        
        # Subquery
        'max_over_time(cpu_usage_percent[1h:5m])',
        
        # Multiple metrics in one query
        'up * on(instance) group_left(job) up{job="prometheus"}',
    ]
    
    print("PromQL Series Extractor Demo")
    print("=" * 50)
    
    for i, query in enumerate(test_queries, 1):
        print(f"\nQuery {i}: {query}")
        
        # Try with promql-parser
        series_parsed = extract_series_from_promql(query)
        print(f"Parsed series: {series_parsed}")
        
        # Fallback with regex
        series_regex = extract_series_simple_regex(query)
        print(f"Regex series: {series_regex}")


def main():
    """
    Main function to run the series extractor.
    """
    print("PromQL Series Extractor")
    print("Enter PromQL queries to extract series names.")
    print("Type 'demo' to see examples, 'quit' to exit.\n")
    
    while True:
        try:
            query = input("Enter PromQL query: ").strip()
            
            if query.lower() in ['quit', 'exit', 'q']:
                break
            elif query.lower() == 'demo':
                demo_queries()
                continue
            elif not query:
                continue
            
            print(f"\nAnalyzing query: {query}")
            
            # Extract series using promql-parser
            series = extract_series_from_promql(query)
            if series:
                print(f"Found series: {', '.join(series)}")
            else:
                print("No series found with parser, trying regex fallback...")
                series_regex = extract_series_simple_regex(query)
                if series_regex:
                    print(f"Found series (regex): {', '.join(series_regex)}")
                else:
                    print("No series found.")
            
            print()
            
        except KeyboardInterrupt:
            print("\nExiting...")
            break
        except Exception as e:
            print(f"Error: {e}")


if __name__ == "__main__":
    main()