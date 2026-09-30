import { expect, test, type Locator, type Page } from "@playwright/test";

// Plays the Sandbox the way a person does and fails on any browser error.

function node(page: Page, id: string): Locator {
  return page.locator(`.react-flow__node[data-id="${id}"]`);
}

async function placeAt(page: Page, label: string, x: number, y: number) {
  const canvas = page.locator(".sb-canvas");
  await page.locator(".sb-kind", { hasText: label }).dragTo(canvas, { targetPosition: { x, y } });
}

// React Flow connects on pointer moves, so drag in steps like a person would.
async function wire(page: Page, from: string, to: string) {
  const src = (await node(page, from).locator(".react-flow__handle.source").boundingBox())!;
  const dst = (await node(page, to).locator(".react-flow__handle.target").boundingBox())!;
  await page.mouse.move(src.x + src.width / 2, src.y + src.height / 2);
  await page.mouse.down();
  await page.mouse.move(src.x + 40, src.y + 10, { steps: 5 });
  await page.mouse.move(dst.x + dst.width / 2, dst.y + dst.height / 2, { steps: 15 });
  await page.mouse.up();
}

// settle waits until the canvas viewport stops moving (after fit view or pan).
async function settle(page: Page) {
  const viewport = page.locator(".react-flow__viewport");
  let last = "";
  await expect
    .poll(async () => {
      const now = (await viewport.getAttribute("style")) ?? "";
      const stable = now === last;
      last = now;
      return stable;
    }, { intervals: [150] })
    .toBe(true);
}

async function tile(page: Page, label: string): Promise<string> {
  return (await page.locator(".sb-tile", { has: page.locator(".sb-tile-label", { hasText: new RegExp(`^${label}$`) }) }).locator(".sb-tile-value").textContent()) ?? "";
}

test("build, run, scale, delete, and resume a game", async ({ page }) => {
  const errors: string[] = [];
  page.on("pageerror", (e) => errors.push(`pageerror: ${e.message}`));
  page.on("console", (m) => {
    if (m.type() === "error") errors.push(`console: ${m.text()}`);
  });
  const toastSeen: string[] = [];

  await page.goto("/sandbox");
  await page.evaluate(() => localStorage.clear());
  await page.reload();
  await page.getByRole("button", { name: "New game" }).click();
  await expect(node(page, "internet")).toBeVisible();

  // Only the first kinds are unlocked; a load balancer waits for the first request.
  const lb = page.locator('.sb-kind[data-kind="load-balancer"]');
  await expect(lb).toBeDisabled();
  await expect(lb).toContainText("goal: First request");

  // Click to place: it must not crash, and the new node becomes the selection.
  await page.locator(".sb-kind", { hasText: "Application instance" }).click();
  await expect(node(page, "app-instance-1")).toBeVisible();
  await expect(page.locator(".sb-inspector h3")).toHaveText("Application instance");

  // A click-placed node never lands on top of another.
  const internet = (await node(page, "internet").boundingBox())!;
  const app = (await node(page, "app-instance-1").boundingBox())!;
  expect(app.x >= internet.x + internet.width || app.y >= internet.y + internet.height).toBe(true);

  // Drag to place the rest at known spots, then move one by dragging.
  const box = (await page.locator(".sb-canvas").boundingBox())!;
  await placeAt(page, "Database primary", box.width * 0.85, box.height * 0.15);
  await placeAt(page, "Object storage", box.width * 0.85, box.height * 0.55);
  await expect(node(page, "db-primary-1")).toBeVisible();
  await expect(node(page, "object-storage-1")).toBeVisible();
  const before = (await node(page, "object-storage-1").boundingBox())!;
  await page.mouse.move(before.x + before.width / 2, before.y + 12);
  await page.mouse.down();
  await page.mouse.move(before.x + before.width / 2, before.y + 92, { steps: 10 });
  await page.mouse.up();
  await expect.poll(async () => (await node(page, "object-storage-1").boundingBox())!.y).toBeGreaterThan(before.y + 60);

  await page.locator(".react-flow__controls-fitview").click();
  await settle(page);
  await wire(page, "internet", "app-instance-1");
  await wire(page, "app-instance-1", "db-primary-1");
  await wire(page, "app-instance-1", "object-storage-1");
  await expect(page.locator(".react-flow__edge")).toHaveCount(3);

  // A database sends no traffic, so it offers nothing to connect from.
  await expect(node(page, "db-primary-1").locator(".react-flow__handle.source")).toHaveCount(0);

  // Run the clock: revenue appears and errors are zero.
  await page.getByRole("button", { name: "8×" }).click();
  await expect(page.locator(".sb-notice")).toContainText("Goal reached: First request. Unlocked Load balancer.");
  await expect(lb).toBeEnabled();
  await expect(page.locator('.sb-goal[data-goal="first-request"]')).toHaveCount(0);
  await expect.poll(() => tile(page, "Revenue / h"), { timeout: 10_000 }).not.toBe("$0.00");
  await expect.poll(() => tile(page, "Errors")).toBe("0.0%");
  await page.getByRole("button", { name: "❚❚" }).click();

  // Scale the app through the inspector.
  await node(page, "app-instance-1").click();
  await page.locator(".sb-replicas button", { hasText: "+" }).click();
  await expect(node(page, "app-instance-1")).toContainText("×2");

  // Delete storage with the keyboard: its connection goes too, without errors.
  await node(page, "object-storage-1").click();
  await page.keyboard.press("Delete");
  await expect(node(page, "object-storage-1")).toHaveCount(0);
  await expect(page.locator(".react-flow__edge")).toHaveCount(2);
  if (await page.locator(".sb-toast").isVisible()) toastSeen.push((await page.locator(".sb-toast").textContent()) ?? "");

  // Skip a day, then reload: the game resumes.
  await page.getByRole("button", { name: "+1 day" }).click();
  await expect(page.locator(".sb-clock strong")).toContainText("Day 2");
  await page.reload();
  await expect(node(page, "app-instance-1")).toBeVisible();
  await expect(node(page, "app-instance-1")).toContainText("×2");

  // Reaching the goal completed the learning-path exercise tied to it.
  await page.getByRole("link", { name: "Learning path" }).click();
  const serve = page.locator("tr", { hasText: "Serve your first successful request" });
  await expect(serve).toContainText("✓");
  await expect(serve).toContainText("sandbox game-");

  expect(toastSeen, "unexpected error toast").toEqual([]);
  expect(errors, "browser errors").toEqual([]);
});

test("events arrive and the player responds to them", async ({ page }) => {
  const errors: string[] = [];
  page.on("pageerror", (e) => errors.push(`pageerror: ${e.message}`));
  page.on("console", (m) => {
    if (m.type() === "error") errors.push(`console: ${m.text()}`);
  });

  // A seeded game built through the API, so the deck deals the same cards on
  // every run: Internet → gateway → app → primary, replica, storage. Ruleset
  // sandbox/v2 has the deck but no locked kinds.
  const api = page.request;
  const created = await (await api.post("/api/forgelab/sandbox/games", { data: { seed: 7, ruleset: "sandbox/v2" } })).json();
  const game = `/api/forgelab/sandbox/games/${created.id}`;
  const command = async (c: object) => {
    const res = await api.post(`${game}/commands`, { data: c });
    expect(res.ok(), await res.text()).toBe(true);
    return (await res.json()) as { node?: string; state: { nodes: { id: string; downReplicas?: number }[] } };
  };
  const ids: Record<string, string> = {};
  for (const kind of ["api-gateway", "app-instance", "db-primary", "db-replica", "object-storage"]) {
    ids[kind] = (await command({ type: "place", kind, x: 0, y: 0 })).node!;
  }
  const at: Record<string, [number, number]> = {
    "api-gateway": [250, 0], "app-instance": [500, 0], "db-primary": [800, -150], "db-replica": [800, 0], "object-storage": [800, 150],
  };
  for (const [kind, [x, y]] of Object.entries(at)) await command({ type: "move", node: ids[kind], x, y });
  await command({ type: "connect", from: "internet", to: ids["api-gateway"] });
  await command({ type: "connect", from: ids["api-gateway"], to: ids["app-instance"] });
  for (const to of ["db-primary", "db-replica", "object-storage"]) await command({ type: "connect", from: ids["app-instance"], to: ids[to] });

  await page.goto("/sandbox");
  await page.evaluate((id) => localStorage.setItem("forgelab.sandbox.game", id), created.id);
  await page.reload();
  await expect(node(page, ids["app-instance"])).toBeVisible();
  await expect(page.locator(".sb-events")).toContainText("Quiet for now");

  // Skip days until the deck deals a card; it shows in the event strip.
  const events = page.locator(".sb-event, .sb-event-past li");
  for (let day = 0; day < 10 && (await events.count()) === 0; day++) {
    await page.getByRole("button", { name: "+1 day" }).click();
    await expect(page.locator(".sb-clock strong")).toContainText(`Day ${day + 2}`);
  }
  await expect(events.first()).toBeVisible();

  // Rate-limit the gateway and lift it again.
  await node(page, ids["api-gateway"]).click();
  await page.getByRole("button", { name: "Rate limit", exact: true }).click();
  await expect(node(page, ids["api-gateway"])).toContainText("rate-limited");
  await page.getByRole("button", { name: "Lift rate limit" }).click();
  await expect(node(page, ids["api-gateway"])).not.toContainText("rate-limited");

  // Step an hour at a time until an instance crash takes a component down,
  // then restart it from the inspector.
  type Ev = { card: string; phase: string; target?: string };
  let crashed: string | undefined;
  for (let hour = 0; hour < 24 * 10 && !crashed; hour++) {
    const state = await (await api.post(`${game}/step`, { data: { ticks: 12 } })).json();
    crashed = (state.events as Ev[]).find((e) => e.card === "instance-crash" && e.phase === "active")?.target;
  }
  expect(crashed, "the seeded deck should crash a component within ten days").toBeTruthy();
  await expect(node(page, crashed!)).toContainText("DOWN");
  await expect(page.locator('.sb-event[data-card="instance-crash"]')).toContainText(crashed!);
  await node(page, crashed!).click();
  await page.getByRole("button", { name: /^Restart/ }).click();
  await page.getByRole("button", { name: "+1h" }).click();
  await expect(node(page, crashed!)).not.toContainText("DOWN");

  // Fail the primary over: the replica becomes the primary.
  await node(page, ids["db-primary"]).click();
  await page.getByRole("button", { name: "Fail over to a replica" }).click();
  await expect(node(page, ids["db-replica"]).locator(".sb-node-title")).toHaveText("Database primary");
  await expect(node(page, ids["db-primary"]).locator(".sb-node-title")).toHaveText("Database read replica");

  await expect(page.locator(".sb-toast")).toHaveCount(0);
  expect(errors, "browser errors").toEqual([]);
});
