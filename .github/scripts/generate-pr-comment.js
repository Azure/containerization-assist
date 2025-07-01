const generatePRComment = (data) => {
  const {
    canaryTime,
    totalTime,
    coverage,
    security,
    architecture,
    testResults
  } = data;

  const statusEmoji = data.failed ? '❌' : '✅';
  const progressBar = generateProgressBar(data.completedJobs, data.totalJobs);

  return `## 🚀 CI Pipeline Status ${statusEmoji}

### Overview
| Status | Phase | Duration | Progress |
|--------|-------|----------|----------|
| ${data.canaryPassed ? '✅' : '❌'} | Canary Validation | ${canaryTime} | ████████████ 100% |
| ${data.testsPassed ? '✅' : '🔄'} | Unit Tests | ${data.testsTime || '-'} | ${progressBar} |
| ${data.integrationPassed ? '✅' : '⏸️'} | Integration | ${data.integrationTime || '-'} | ${data.integrationProgress || '░░░░░░░░░░░░ 0%'} |

### 📊 Metrics Dashboard

#### Coverage ${generateTrend(coverage.current, coverage.previous)}
\`\`\`
Total: ${coverage.current}% (${coverage.delta})
${generateCoverageGraph(coverage.history)}
\`\`\`

<details>
<summary>📋 Package Coverage</summary>

| Package | Coverage | Change |
|---------|----------|--------|
${coverage.packages.map(p => `| ${p.name} | ${p.coverage}% | ${p.delta} |`).join('\n')}

</details>

#### Build Performance
\`\`\`
Canary: ${canaryTime} ${data.canaryImprovement ? '⚡' : ''}
Build: ${data.buildTime}
Tests: ${data.testTime}
Total: ${totalTime} ${data.totalImprovement ? '⚡' : ''}
\`\`\`

### 🔒 Security Status
${security.secrets === 0 ? '✅ No secrets detected' : `❌ ${security.secrets} secrets found`}
${security.vulnerabilities === 0 ? '✅ No vulnerabilities' : `⚠️ ${security.vulnerabilities} vulnerabilities`}

### 🏗️ Architecture
- Adapters: ${architecture.adapters} ${architecture.adapters === 0 ? '✅' : '❌'}
- Wrappers: ${architecture.wrappers} ${architecture.wrappers === 0 ? '✅' : '❌'}
- Import Cycles: ${architecture.cycles} ${architecture.cycles === 0 ? '✅' : '❌'}

---
*Updated: ${new Date().toISOString()} • [View Full Report](${data.runUrl})*
`;
};

function generateProgressBar(completed, total) {
  const percentage = Math.round((completed / total) * 100);
  const filled = Math.round(percentage / 10);
  const empty = 10 - filled;
  return '█'.repeat(filled) + '░'.repeat(empty) + ` ${percentage}%`;
}

function generateTrend(current, previous) {
  if (!previous) return '';
  const diff = current - previous;
  if (diff > 0) return '📈';
  if (diff < 0) return '📉';
  return '➡️';
}

function generateCoverageGraph(history) {
  // Simple ASCII graph
  const max = Math.max(...history);
  const min = Math.min(...history);
  const height = 4;

  let graph = '';
  for (let i = height; i > 0; i--) {
    const threshold = min + (max - min) * (i / height);
    graph += history.map(v => v >= threshold ? '█' : ' ').join('') + '\n';
  }
  return graph;
}

// Enhanced comment generator with real-time updates
const generateEnhancedPRComment = (statusData) => {
  const {
    runId,
    runUrl,
    jobs,
    metrics,
    timestamp
  } = statusData;

  // Calculate overall status
  const totalJobs = Object.keys(jobs).length;
  const completedJobs = Object.values(jobs).filter(job => job.status === 'completed').length;
  const failedJobs = Object.values(jobs).filter(job => job.status === 'failure').length;
  const overallStatus = failedJobs > 0 ? '❌' : completedJobs === totalJobs ? '✅' : '🔄';

  return `## 🚀 CI Pipeline Status ${overallStatus}

### 📋 Job Status
| Job | Status | Duration | Details |
|-----|--------|----------|---------|
${Object.entries(jobs).map(([name, job]) => {
  const statusIcon = job.status === 'completed' ? '✅' : job.status === 'failure' ? '❌' : job.status === 'running' ? '🔄' : '⏸️';
  const duration = job.duration || '-';
  const details = job.details || '';
  return `| ${name} | ${statusIcon} ${job.status} | ${duration} | ${details} |`;
}).join('\n')}

### 📊 Key Metrics
${metrics ? `
#### Test Coverage
- **Total**: ${metrics.coverage?.total || 'N/A'}%
- **Change**: ${metrics.coverage?.delta || 'N/A'}%

#### Security
- **Secrets**: ${metrics.security?.secrets || 'N/A'} found
- **Vulnerabilities**: ${metrics.security?.vulnerabilities || 'N/A'} found

#### Architecture
- **Adapters**: ${metrics.architecture?.adapters || 'N/A'} ${(metrics.architecture?.adapters || 0) === 0 ? '✅' : '❌'}
- **Wrappers**: ${metrics.architecture?.wrappers || 'N/A'} ${(metrics.architecture?.wrappers || 0) === 0 ? '✅' : '❌'}
- **Import Cycles**: ${metrics.architecture?.cycles || 'N/A'} ${(metrics.architecture?.cycles || 0) === 0 ? '✅' : '❌'}

#### Performance
- **Canary Time**: ${metrics.performance?.canaryTime || 'N/A'}
- **Total Time**: ${metrics.performance?.totalTime || 'N/A'}
` : 'Metrics will be updated as jobs complete...'}

### 🎯 Progress: ${completedJobs}/${totalJobs} jobs completed
${generateProgressBar(completedJobs, totalJobs)}

---
*Last updated: ${timestamp || new Date().toISOString()} • [View Full Report](${runUrl}) • Run ID: ${runId}*
`;
};

module.exports = {
  generatePRComment,
  generateEnhancedPRComment,
  generateProgressBar,
  generateTrend,
  generateCoverageGraph
};
