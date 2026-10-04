import { expect, test } from "@playwright/test";
import { execFileSync } from "node:child_process";

// Sign-in shell tests against a database-backed API (ADR-0033). The session is
// real: seed-session creates the user and session row, and the injects token as
// the browser's session cookie. Real OAuth cannot run here, so the callback is
// covered by the Go tests and a manual check.

const DB = process.env.FORGELAB_TEST_DATABASE_URL ?? "";

function seedSession(): string {
  const out = execFileSync(
    "go",
    ["run", "./cmd/seed-session", "-database", DB, "-email", `e2e-${Date.now()}@forgelab.test`],
    { cwd: "../backend", env: { ...process.env, FORGELAB_TEST_SESSION: "1" }, encoding: "utf8" },
  );
  return out.trim();
}

test.skip(!DB, "set FORGELAB_TEST_DATABASE_URL to run the sign-in browser tests");

test("anonymous is offered sign-in and keeps nothing", async ({ page }) => {
  const errors: string[] = [];
  page.on("pageerror", (e) => errors.push(`pageerror: ${e.message}`));
  page.on("console", (m) => {
    // A 401 from /auth/me is the expected anonymous path; the browser logs it
    // as a failed-resource console error, which is not an app error.
    if (m.type() === "error" && !m.text().includes("Failed to load resource")) {
      errors.push(`console: ${m.text()}`);
    }
  });

  await page.goto("/me");
  await expect(page.getByRole("heading", { name: "My ForgeLab" })).toBeVisible();
  await expect(page.getByText("Sign in to keep your work")).toBeVisible();
  // The header also offers sign-in, so scope to the page's card.
  await expect(page.getByRole("main").getByRole("link", { name: "Sign in with GitHub" })).toBeVisible();

  expect(errors).toEqual([]);
});

test("a signed-in player lists, resumes, and deletes a saved game", async ({ page, playwright, baseURL }) => {
  const errors: string[] = [];
  page.on("pageerror", (e) => errors.push(`pageerror: ${e.message}`));
  page.on("console", (m) => {
    if (m.type() === "error") errors.push(`console: ${m.text()}`);
  });

  const token = seedSession();

  // Seed a saved game through the API with the same session.
  const api = await playwright.request.newContext({
    baseURL: "http://127.0.0.1:8091",
    extraHTTPHeaders: { cookie: `forgelab_session=${token}` },
  });
  const game = await (await api.post("/api/v1/sandbox/games")).json();
  const saved = await (await api.post(`/api/v1/sandbox/games/${game.id}/save`)).json();
  await api.dispose();
  expect(saved.id).toBeTruthy();

  await page.context().addCookies([
    { name: "forgelab_session", value: token, url: baseURL!, httpOnly: true, sameSite: "Lax" },
  ]);

  await page.goto("/me");
  await expect(page.locator("p.lead").first()).toContainText("E2E Player");
  const row = page.locator("table tbody tr").first();
  await expect(row).toContainText("Untitled");

  await row.getByRole("button", { name: "Resume" }).click();
  await expect(page).toHaveURL(/\/sandbox/);

  await page.goto("/me");
  await page.locator("table tbody tr").first().getByRole("button", { name: "Delete" }).click();
  await expect(page.getByText("No saved games yet")).toBeVisible();

  expect(errors).toEqual([]);
});
