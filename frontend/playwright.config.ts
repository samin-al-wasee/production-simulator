import { defineConfig, devices } from "@playwright/test";

// Browser tests for the dashboard. They start (or reuse) the backend API and
// the dev server, so `npm run e2e` works on its own. The started API runs
// without a store (`-database=`) so the browser tests use the file-based,
// anonymous path they assert; a started API records learning progress to a
// scratch file, so test games never complete your exercises. A reused
// `make serve` talks to whatever it was started with.
export default defineConfig({
  testDir: "e2e",
  testIgnore: "**/auth.spec.ts",
  timeout: 60_000,
  retries: 0,
  use: {
    baseURL: "http://localhost:3001",
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
  },
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"], viewport: { width: 1440, height: 1000 } } }],
  webServer: [
    {
      command: "cd ../backend && go run ./cmd/forgelab serve -repo .. -database= -progress ../frontend/test-results/progress.json",
      url: "http://127.0.0.1:8090/healthz",
      reuseExistingServer: true,
      timeout: 120_000,
    },
    {
      command: "npm run dev",
      url: "http://localhost:3001/sandbox",
      reuseExistingServer: true,
      timeout: 120_000,
    },
  ],
});
