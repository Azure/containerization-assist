#!/bin/bash
# Run all Phase 0 CEL validation spike tests

set -e

echo "========================================="
echo "CEL Policy Support - Phase 0 Spike Tests"
echo "========================================="
echo ""

echo "Running Test 0.1: CEL Library Evaluation..."
npx tsx spike/cel-evaluation/basic-evaluation.ts
echo ""

echo "Running Test 0.2: YAML Schema Validation..."
npx tsx spike/cel-evaluation/yaml-schema-test.ts
echo ""

echo "Running Test 0.3: Integration Proof-of-Concept..."
npx tsx spike/cel-evaluation/integration-test.ts
echo ""

echo "========================================="
echo "✅ All Phase 0 spike tests completed!"
echo "========================================="
echo ""
echo "Next step: Review PHASE0-DECISION-GATE.md"
echo ""
