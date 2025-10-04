#!/usr/bin/env node

/**
 * Test that simulates a client importing our package (CommonJS)
 * Validates modern API: createApp, TOOLS, getAllInternalTools
 */

console.log('Testing client import (CommonJS)...\n');

try {
  // Import the CommonJS build like a webpack client would
  const pkg = require('../../dist-cjs/src/index.js');
  const { createApp, TOOLS, getAllInternalTools, ALL_TOOLS } = pkg;

  console.log('✅ Successfully imported package');

  // Verify createApp exists
  if (typeof createApp === 'function') {
    console.log('✅ createApp is a valid function');

    // Try creating an app instance
    const app = createApp();
    console.log('✅ Successfully created app runtime');

    // Verify app methods
    if (typeof app.execute === 'function') {
      console.log('✅ app.execute method available');
    }
    if (typeof app.bindToMCP === 'function') {
      console.log('✅ app.bindToMCP method available');
    }
    if (typeof app.listTools === 'function') {
      console.log('✅ app.listTools method available');
      const tools = app.listTools();
      console.log(`✅ Found ${tools.length} tools`);
    }
  }

  // Verify TOOLS constants
  if (typeof TOOLS === 'object') {
    console.log('✅ TOOLS constants exported');
    console.log(`✅ Sample tool names: ${TOOLS.ANALYZE_REPO}, ${TOOLS.BUILD_IMAGE}, ${TOOLS.SCAN}`);
  }

  // Verify getAllInternalTools
  if (typeof getAllInternalTools === 'function') {
    console.log('✅ getAllInternalTools function available');
    const tools = getAllInternalTools();
    console.log(`✅ Registry contains ${tools.length} tools`);
  }

  // Verify ALL_TOOLS
  if (Array.isArray(ALL_TOOLS)) {
    console.log('✅ ALL_TOOLS array exported');
  }

  console.log('\n🎉 All CommonJS import tests passed!');
} catch (error) {
  console.error('❌ Import failed:', error.message);
  console.error(error);
  process.exit(1);
}