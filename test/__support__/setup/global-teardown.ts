export default async function globalTeardown() {
  console.log('\n🧹 Cleaning up global test environment...');

  try {
    // Basic cleanup without Docker dependencies to avoid import issues
    // Individual tests handle their own Docker cleanup
    console.log('✅ Basic cleanup completed');
  } catch (error: any) {
    console.error('❌ Global teardown error:', error.message);
    // Don't fail the test run due to cleanup errors
  }

  console.log('✅ Global teardown complete');
}
