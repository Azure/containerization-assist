#!/bin/bash

# Weekly Quality Progress Report Generator
# This script generates comprehensive weekly reports and can be scheduled via cron

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
REPORTS_DIR="$ROOT_DIR/reports/weekly"
TIMESTAMP=$(date +%Y%m%d)
WEEK_NUM=$(date +%V)

echo "📊 Generating Weekly Quality Progress Report"
echo "==========================================="
echo "Date: $(date)"
echo "Week: $WEEK_NUM"
echo ""

# Create reports directory
mkdir -p "$REPORTS_DIR"

# Step 1: Generate fresh quality metrics
echo "1️⃣ Collecting current quality metrics..."
cd "$ROOT_DIR"
go run tools/quality-dashboard/main.go \
    -output quality-metrics.json \
    -format json

go run tools/validate-interfaces/main.go \
    --metrics \
    --metrics-output interface-metrics.json

echo "✅ Metrics collected"

# Step 2: Generate weekly reports in multiple formats
echo ""
echo "2️⃣ Generating weekly reports..."

# Text report (for email/slack)
go run tools/weekly-progress/main.go \
    -format text \
    -output "$REPORTS_DIR/weekly-report-$TIMESTAMP.txt"

# Markdown report (for wiki/docs)
go run tools/weekly-progress/main.go \
    -format markdown \
    -output "$REPORTS_DIR/weekly-report-$TIMESTAMP.md"

# HTML report (for web viewing)
go run tools/weekly-progress/main.go \
    -format html \
    -output "$REPORTS_DIR/weekly-report-$TIMESTAMP.html"

# JSON report (for further processing)
go run tools/weekly-progress/main.go \
    -format json \
    -output "$REPORTS_DIR/weekly-report-$TIMESTAMP.json"

echo "✅ Reports generated"

# Step 3: Create summary for quick viewing
echo ""
echo "3️⃣ Creating executive summary..."
cat > "$REPORTS_DIR/latest-summary.txt" << EOF
WEEKLY QUALITY SUMMARY - Week $WEEK_NUM
======================================
Generated: $(date)

$(go run tools/weekly-progress/main.go -format text | head -30)

Full reports available in: $REPORTS_DIR/
EOF

# Step 4: Update symlinks to latest reports
ln -sf "weekly-report-$TIMESTAMP.txt" "$REPORTS_DIR/latest.txt"
ln -sf "weekly-report-$TIMESTAMP.md" "$REPORTS_DIR/latest.md"
ln -sf "weekly-report-$TIMESTAMP.html" "$REPORTS_DIR/latest.html"
ln -sf "weekly-report-$TIMESTAMP.json" "$REPORTS_DIR/latest.json"

echo "✅ Summary created"

# Step 5: Optional - Send notifications
if [ "$1" == "--notify" ]; then
    echo ""
    echo "4️⃣ Sending notifications..."
    
    # Example: Post to Slack (requires webhook URL)
    if [ -n "$SLACK_WEBHOOK_URL" ]; then
        SUMMARY=$(cat "$REPORTS_DIR/latest-summary.txt" | head -20)
        curl -X POST -H 'Content-type: application/json' \
            --data "{\"text\":\"$SUMMARY\"}" \
            "$SLACK_WEBHOOK_URL"
    fi
    
    # Example: Send email (requires mail configured)
    if command -v mail &> /dev/null && [ -n "$REPORT_EMAIL" ]; then
        cat "$REPORTS_DIR/latest.txt" | mail -s "Weekly Quality Report - Week $WEEK_NUM" "$REPORT_EMAIL"
    fi
    
    echo "✅ Notifications sent"
fi

# Step 6: Archive old reports (keep last 12 weeks)
echo ""
echo "5️⃣ Archiving old reports..."
find "$REPORTS_DIR" -name "weekly-report-*.txt" -mtime +84 -delete
find "$REPORTS_DIR" -name "weekly-report-*.md" -mtime +84 -delete
find "$REPORTS_DIR" -name "weekly-report-*.html" -mtime +84 -delete
find "$REPORTS_DIR" -name "weekly-report-*.json" -mtime +84 -delete
echo "✅ Old reports archived"

# Display summary
echo ""
echo "📋 REPORT SUMMARY"
echo "================="
cat "$REPORTS_DIR/latest-summary.txt" | grep -E "Error Handling:|Test Coverage:|Interface Compliance:" || true

echo ""
echo "📁 Reports saved to:"
echo "  Text:     $REPORTS_DIR/weekly-report-$TIMESTAMP.txt"
echo "  Markdown: $REPORTS_DIR/weekly-report-$TIMESTAMP.md"
echo "  HTML:     $REPORTS_DIR/weekly-report-$TIMESTAMP.html"
echo "  JSON:     $REPORTS_DIR/weekly-report-$TIMESTAMP.json"
echo ""
echo "  Latest:   $REPORTS_DIR/latest.*"

# Optional: Open HTML report in browser
if [ "$2" == "--open" ] && command -v open &> /dev/null; then
    open "$REPORTS_DIR/latest.html"
fi

echo ""
echo "✅ Weekly report generation complete!"

# Exit with success
exit 0