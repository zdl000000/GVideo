import { defineConfig } from "vitest/config";

export default defineConfig({
  test: {
    environment: "node",
    include: ["src/**/*.test.ts", "src/**/*.test.tsx"],
    pool: "threads",
    maxWorkers: 1,
    fileParallelism: false
  }
});
