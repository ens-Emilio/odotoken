import '@testing-library/jest-dom/vitest'
import { afterEach } from 'vitest'
import { cleanup } from '@testing-library/react'

// Vitest runs without injected globals here, so Testing Library cannot
// register its own afterEach cleanup. Without this, every render in a file
// shares one document and the next query sees two of everything.
afterEach(cleanup)
