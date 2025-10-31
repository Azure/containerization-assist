package containerization.platform

# ==============================================================================
# Test: Platform and Tag Enforcement Policy
# ==============================================================================

# Test 1: Block Dockerfile without platform specification
test_block_missing_platform if {
	count(violations) > 0 with input as {"content": "FROM node:20-alpine\nLABEL tag=\"demo\"\nCMD [\"node\", \"app.js\"]"}
}

# Test 2: Block Dockerfile with wrong platform
test_block_wrong_platform if {
	count(violations) > 0 with input as {"content": "FROM --platform=linux/amd64 node:20-alpine\nLABEL tag=\"demo\"\nCMD [\"node\", \"app.js\"]"}
}

# Test 3: Block Dockerfile without demo tag label
test_block_missing_tag_label if {
	count(violations) > 0 with input as {"content": "FROM --platform=linux/arm64 node:20-alpine\nCMD [\"node\", \"app.js\"]"}
}

# Test 4: Block Dockerfile with wrong tag label
test_block_wrong_tag if {
	count(violations) > 0 with input as {"content": "FROM --platform=linux/arm64 node:20-alpine\nLABEL tag=\"production\"\nCMD [\"node\", \"app.js\"]"}
}

# Test 5: Allow valid Dockerfile with platform and demo tag
test_allow_valid_dockerfile if {
	allow with input as {"content": "FROM --platform=linux/arm64 node:20-alpine\nLABEL tag=\"demo\"\nCMD [\"node\", \"app.js\"]"}
}

# Test 6: Allow Dockerfile with correct platform and tag (different format)
test_allow_valid_dockerfile_alt_format if {
	allow with input as {"content": "FROM --platform=linux/arm64 mcr.microsoft.com/dotnet/aspnet:8.0\n\nLABEL tag=\"demo\"\nLABEL version=\"1.0.0\"\n\nCOPY . /app\nCMD [\"dotnet\", \"app.dll\"]"}
}

# Test 7: Block multi-stage Dockerfile with one stage missing platform
test_block_multistage_missing_platform if {
	count(violations) > 0 with input as {"content": "FROM --platform=linux/arm64 golang:1.21-alpine AS builder\nWORKDIR /build\nCOPY . .\nRUN go build\n\nFROM alpine:latest\nLABEL tag=\"demo\"\nCOPY --from=builder /build/app /app\nCMD [\"/app\"]"}
}

# Test 8: Allow multi-stage Dockerfile with all platforms correct
test_allow_multistage_with_platforms if {
	allow with input as {"content": "FROM --platform=linux/arm64 golang:1.21-alpine AS builder\nWORKDIR /build\nCOPY . .\nRUN go build\n\nFROM --platform=linux/arm64 alpine:latest\nLABEL tag=\"demo\"\nCOPY --from=builder /build/app /app\nCMD [\"/app\"]"}
}

# Test 9: Verify suggestion appears for correct usage
test_suggestion_for_correct_tag if {
	count(suggestions) > 0 with input as {"content": "FROM --platform=linux/arm64 node:20-alpine\nLABEL tag=\"demo\"\nCMD [\"node\", \"app.js\"]"}
}

# Test 10: Verify specific violation rules
test_violation_rule_platform if {
	some violation in violations with input as {"content": "FROM node:20-alpine\nLABEL tag=\"demo\""}
	violation.rule == "require-arm64-platform"
}

test_violation_rule_tag if {
	some violation in violations with input as {"content": "FROM --platform=linux/arm64 node:20-alpine"}
	violation.rule == "require-demo-tag-label"
}
