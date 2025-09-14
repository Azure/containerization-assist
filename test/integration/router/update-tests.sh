#!/bin/bash

# Update all test files to add mock context

for file in force-flag.test.ts preconditions.test.ts idempotency.test.ts error-recovery.test.ts; do
  echo "Updating $file..."

  # Add import for mock context
  sed -i "/import { MockSessionManager/a import { createMockContext } from './fixtures/mock-context';" "$file"

  # Add mockContext variable declaration
  sed -i "s/let logger: pino.Logger;/let logger: pino.Logger;\n  let mockContext: any;/" "$file"

  # Initialize mockContext in beforeEach
  sed -i "s/logger = pino({ level: 'silent' });/logger = pino({ level: 'silent' });\n    mockContext = createMockContext();/" "$file"

  # Add context to all router.route calls (careful pattern to avoid duplicates)
  perl -i -pe 's/await router\.route\(\{(?!\s*context:)/await router.route({\n        context: mockContext,/g' "$file"
  perl -i -pe 's/await failingRouter\.route\(\{(?!\s*context:)/await failingRouter.route({\n        context: mockContext,/g' "$file"
  perl -i -pe 's/await recoveringRouter\.route\(\{(?!\s*context:)/await recoveringRouter.route({\n        context: mockContext,/g' "$file"
  perl -i -pe 's/await incompleteRouter\.route\(\{(?!\s*context:)/await incompleteRouter.route({\n        context: mockContext,/g' "$file"
  perl -i -pe 's/await crashingRouter\.route\(\{(?!\s*context:)/await crashingRouter.route({\n        context: mockContext,/g' "$file"
  perl -i -pe 's/await rejectingRouter\.route\(\{(?!\s*context:)/await rejectingRouter.route({\n        context: mockContext,/g' "$file"
  perl -i -pe 's/await routerWithBadSession\.route\(\{(?!\s*context:)/await routerWithBadSession.route({\n        context: mockContext,/g' "$file"
done

echo "Done!"