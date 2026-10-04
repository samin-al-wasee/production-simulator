import { defineConfig, devices } from "@playwright/test";

// Sign-in browser tests. Unlike the default suite, this runs the API WITH a
// database so a seeded session is real (ADR-0033). It uses its own ports so it
// never clashes with the store-less anonymous suite. Requires
// FORGELAB_TEST_DATABASE_URL (the devcontainer and CI set it).
const dsn = process.env.FORGELAB_TEST_DATABASE_URL ?? "";

export default defineConfig({
  testDir: "e2e",
  testMatch: "**/auth.spec.ts",
  timeout: 60_000,
  retries: 0,
  use: {
    baseURL: "http://localhost:3002",
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
  },
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"], viewport: { width: 1440, height: 1000 } } }],
  webServer: [
    {
      command: "cd ../backend && go run ./cmd/forgelab serve -repo .. -addr 127.0.0.1:8091 -allow-origin http://localhost:3002",
      url: "http://127.0.0.1:8091/healthz",
      reuseExistingServer: false,
      timeout: 120_000,
      env: {
        ...process.env,
        DATABASE_URL: dsn,
        GITHUB_CLIENT_ID: "test-client-id",
        GITHUB_CLIENT_SECRET: "test-secret",
        FORGELAB_PUBLIC_URL: "http://localhost:3002",
      } as Record<string, string>,
    },
    {
      command: "npx next dev --port 3002",
      // Readiness hits the auth proxy so its route handler is compiled before
      // the tests. Otherwise the first test's request races a ~5s cold compile.
      url: "http://localhost:3002/api/v1/auth/providers",
      reuseExistingServer: false,
      timeout: 120_000,
      env: {
        ...process.env,
        NODE_ENV: "development",
        FORGELAB_API_URL: "http://127.0.0.1:8091",
        NEXT_DIST_DIR: ".next-auth",
      } as Record<string, string>,
    },
  ],
});
