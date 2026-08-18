import '@testing-library/jest-dom/vitest';
import { cleanup } from '@testing-library/react';
import { afterEach } from 'vitest';

// Unmount between tests so a component's listeners and timers cannot leak into
// the next one — the usual source of a suite that passes alone and fails in a
// run.
afterEach(cleanup);
