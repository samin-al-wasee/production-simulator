import { expect, test } from "@playwright/test";

// The pages around the game load, work, and raise no browser errors.

test("home opens the Sandbox, and Pipelines and Learning path work", async ({ page }) => {
  const errors: string[] = [];
  page.on("pageerror", (e) => errors.push(`pageerror: ${e.message}`));
  page.on("console", (m) => {
    if (m.type() === "error") errors.push(`console: ${m.text()}`);
  });

  await page.goto("/");
  await expect(page).toHaveURL(/\/sandbox$/);
  await expect(page.getByRole("heading", { name: "Production Sandbox" })).toBeVisible();
  await expect(page.locator("header.site nav a")).toHaveText(["Sandbox", "Pipelines", "Learning path"]);

  await page.getByRole("link", { name: "Pipelines" }).click();
  await page.getByRole("button", { name: "Simulate run" }).click();
  await expect(page.getByRole("region", { name: "Pipeline run" })).toContainText("virtual time");

  await page.getByRole("link", { name: "Learning path" }).click();
  await expect(page.locator("section.card h3")).toHaveCount(6);
  await expect(page.locator("section.card h3").first()).toContainText("First production");

  expect(errors, "browser errors").toEqual([]);
});
