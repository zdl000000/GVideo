import { defineConfig } from "vitest/config";

export default defineConfig({
  test: {
    environment: "node",
    include: ["src/**/*.test.ts", "src/**/*.test.tsx"],
    pool: "threads",
    maxWorkers: 1,
    fileParallelism: false,
    // Reuse one worker for every test file. One worker startup per run instead
    // of one per file removes the intermittent "Timeout waiting for worker to
    // respond" start failure seen on Windows and cuts suite wall time notably.
    isolate: false,
    // With a shared worker, keep test files from leaking stubbed globals into
    // each other.
    unstubGlobals: true
  }
});
