import { defineConfig, devices } from "@playwright/test";

// Browser tests for the dashboard. They start (or reuse) the backend API and
// the dev server, so `npm run e2e` works on its own. A started API records
// learning progress to a scratch file, so test games never complete your
// exercises; a reused `make serve` records to your real progress file.
export default defineConfig({
  testDir: "e2e",
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
      command: "cd ../backend && go run ./cmd/forgelab serve -repo .. -progress ../frontend/test-results/progress.json",
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
