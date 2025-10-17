/**
 * Test Repository Fixtures Index
 * Exports all test repository fixtures for use in unit tests
 */

import { 
  nodeExpressBasicRepository, 
  expectedNodeExpressAnalysis, 
  expectedNodeExpressDockerfile 
} from './node-express-basic';

import { 
  pythonFlaskBasicRepository, 
  expectedPythonFlaskAnalysis, 
  expectedPythonFlaskDockerfile 
} from './python-flask-basic';

import { 
  javaSpringBootBasicRepository, 
  expectedJavaSpringBootAnalysis, 
  expectedJavaSpringBootDockerfile 
} from './java-springboot-basic';

import { 
  goBasicRepository, 
  expectedGoBasicAnalysis, 
  expectedGoBasicDockerfile 
} from './go-basic';

import { 
  rustBasicRepository, 
  expectedRustBasicAnalysis, 
  expectedRustBasicDockerfile 
} from './rust-basic';

// Re-export all fixtures
export { 
  nodeExpressBasicRepository, 
  expectedNodeExpressAnalysis, 
  expectedNodeExpressDockerfile,
  pythonFlaskBasicRepository, 
  expectedPythonFlaskAnalysis, 
  expectedPythonFlaskDockerfile,
  javaSpringBootBasicRepository, 
  expectedJavaSpringBootAnalysis, 
  expectedJavaSpringBootDockerfile,
  goBasicRepository, 
  expectedGoBasicAnalysis, 
  expectedGoBasicDockerfile,
  rustBasicRepository, 
  expectedRustBasicAnalysis, 
  expectedRustBasicDockerfile
};

/**
 * Repository fixture catalog for easy access
 */
export const repositoryFixtures = {
  'node-express-basic': {
    repository: nodeExpressBasicRepository,
    expectedAnalysis: expectedNodeExpressAnalysis,
    expectedDockerfile: expectedNodeExpressDockerfile,
  },
  'python-flask-basic': {
    repository: pythonFlaskBasicRepository,
    expectedAnalysis: expectedPythonFlaskAnalysis,
    expectedDockerfile: expectedPythonFlaskDockerfile,
  },
  'java-springboot-basic': {
    repository: javaSpringBootBasicRepository,
    expectedAnalysis: expectedJavaSpringBootAnalysis,
    expectedDockerfile: expectedJavaSpringBootDockerfile,
  },
  'go-basic': {
    repository: goBasicRepository,
    expectedAnalysis: expectedGoBasicAnalysis,
    expectedDockerfile: expectedGoBasicDockerfile,
  },
  'rust-basic': {
    repository: rustBasicRepository,
    expectedAnalysis: expectedRustBasicAnalysis,
    expectedDockerfile: expectedRustBasicDockerfile,
  },
};

export type RepositoryFixtureKey = keyof typeof repositoryFixtures;