#!/usr/bin/env python3
"""Simple benchmark comparison script"""
import sys
import re

def parse_benchmark_line(line):
    """Parse a benchmark line to extract name and ns/op"""
    match = re.match(r'^(Benchmark\S+)\s+\d+\s+([0-9.]+)\s+ns/op', line)
    if match:
        return match.group(1), float(match.group(2))
    return None, None

def main():
    if len(sys.argv) != 3:
        print("Usage: compare_benchmarks.py <baseline_file> <current_file>")
        sys.exit(1)
    
    baseline_file = sys.argv[1]
    current_file = sys.argv[2]
    
    # Parse baseline
    baseline_results = {}
    try:
        with open(baseline_file, 'r') as f:
            for line in f:
                name, ns_op = parse_benchmark_line(line)
                if name:
                    baseline_results[name] = ns_op
    except FileNotFoundError:
        print(f"Baseline file not found: {baseline_file}")
        sys.exit(1)
    
    # Parse current
    current_results = {}
    try:
        with open(current_file, 'r') as f:
            for line in f:
                name, ns_op = parse_benchmark_line(line)
                if name:
                    current_results[name] = ns_op
    except FileNotFoundError:
        print(f"Current file not found: {current_file}")
        sys.exit(1)
    
    # Compare results
    print("=== BENCHMARK COMPARISON ===")
    print(f"Baseline: {baseline_file}")
    print(f"Current: {current_file}")
    print()
    
    regressions = []
    improvements = []
    
    for name, current_ns in current_results.items():
        if name in baseline_results:
            baseline_ns = baseline_results[name]
            diff_percent = ((current_ns - baseline_ns) / baseline_ns) * 100
            
            if diff_percent > 10:  # More than 10% slower
                regressions.append((name, baseline_ns, current_ns, diff_percent))
            elif diff_percent < -10:  # More than 10% faster
                improvements.append((name, baseline_ns, current_ns, diff_percent))
    
    if regressions:
        print("❌ REGRESSIONS DETECTED:")
        for name, baseline, current, diff in regressions:
            print(f"  {name}: {baseline:.2f}ns → {current:.2f}ns ({diff:+.1f}%)")
    
    if improvements:
        print("\n✅ IMPROVEMENTS:")
        for name, baseline, current, diff in improvements:
            print(f"  {name}: {baseline:.2f}ns → {current:.2f}ns ({diff:+.1f}%)")
    
    if not regressions and not improvements:
        print("✅ No significant performance changes detected")
    
    # Check 300μs target
    print("\n=== 300μs TARGET CHECK ===")
    violations = []
    for name, ns_op in current_results.items():
        if ns_op > 300000:  # 300μs = 300,000ns
            violations.append((name, ns_op))
    
    if violations:
        print("⚠️  Benchmarks exceeding 300μs target:")
        for name, ns_op in violations:
            print(f"  {name}: {ns_op:.2f}ns ({ns_op/1000:.2f}μs)")
    else:
        print("✅ All benchmarks within 300μs target")

if __name__ == "__main__":
    main()