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
  // A new world is empty: even the traffic is placed by the player.
  await expect(page.locator(".sb-canvas .react-flow__node")).toHaveCount(0);

  // Only the first kinds are unlocked; a cache waits for the startup tier.
  const cache = page.locator('.sb-kind[data-kind="cache"]');
  await expect(cache).toBeDisabled();
  await expect(cache).toContainText("goal: Startup");

  // Click to place: it must not crash, and the new node becomes the selection.
  await page.locator(".sb-kind", { hasText: "Traffic" }).click();
  await expect(node(page, "traffic-1")).toBeVisible();
  await expect(page.locator(".sb-inspector h3")).toHaveText("Traffic");
  // It asks for nothing until it is connected to an app.
  await expect(page.locator(".sb-inspector")).toContainText("Connect it to an application instance");
  // An application instance starts from a template; cancelling places nothing.
  await page.locator(".sb-kind", { hasText: "Application instance" }).click();
  const chooser = page.getByRole("dialog", { name: "New application instance" });
  await chooser.getByRole("button", { name: "Cancel" }).click();
  await expect(node(page, "app-instance-1")).toHaveCount(0);
  await page.locator(".sb-kind", { hasText: "Application instance" }).click();
  await chooser.getByRole("radio", { name: /^FastAPI/ }).check();
  await chooser.getByRole("button", { name: "Place" }).click();
  await expect(chooser).toBeHidden();
  await expect(node(page, "app-instance-1")).toBeVisible();
  await expect(page.locator(".sb-inspector h3")).toHaveText("Application instance");
  await expect(page.locator(".sb-inspector")).toContainText("Python / FastAPI · async, 2 workers");

  // A click-placed node never lands on top of another.
  const traffic = (await node(page, "traffic-1").boundingBox())!;
  const app = (await node(page, "app-instance-1").boundingBox())!;
  expect(app.x >= traffic.x + traffic.width || app.y >= traffic.y + traffic.height).toBe(true);

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
  await wire(page, "traffic-1", "app-instance-1");
  await wire(page, "app-instance-1", "db-primary-1");
  await wire(page, "app-instance-1", "object-storage-1");
  await expect(page.locator(".react-flow__edge")).toHaveCount(3);

  // A database sends no traffic, so it offers nothing to connect from.
  await expect(node(page, "db-primary-1").locator(".react-flow__handle.source")).toHaveCount(0);

  // Run the clock: revenue appears and errors are zero.
  await page.getByRole("button", { name: "8×" }).click();
  await expect(page.locator(".sb-notice")).toContainText("Goal reached: First request.");
  await expect(page.locator('.sb-goal[data-goal="first-request"]')).toHaveCount(0);
  await expect.poll(() => tile(page, "Revenue / h"), { timeout: 10_000 }).not.toBe("$0.00");
  await expect.poll(() => tile(page, "Errors")).toBe("0.0%");
  await page.getByRole("button", { name: "❚❚" }).click();

  // Scale the app through the inspector.
  await node(page, "app-instance-1").click();
  await page.locator(".sb-replicas button", { hasText: "+" }).click();
  await expect(node(page, "app-instance-1")).toContainText("×2");
  // Clicking the app also opens it; go back to the system.
  await page.keyboard.press("Escape");

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
  // A crashed app is open on the canvas; go back to the system.
  if (await page.locator(".sb-inner").isVisible()) await page.keyboard.press("Escape");

  // Fail the primary over: the replica becomes the primary.
  await node(page, ids["db-primary"]).click();
  await page.getByRole("button", { name: "Fail over to a replica" }).click();
  await expect(node(page, ids["db-replica"]).locator(".sb-node-title")).toHaveText("Database primary");
  await expect(node(page, ids["db-primary"]).locator(".sb-node-title")).toHaveText("Database read replica");

  await expect(page.locator(".sb-toast")).toHaveCount(0);
  expect(errors, "browser errors").toEqual([]);
});

test("configure the Internet's traffic", async ({ page }) => {
  const errors: string[] = [];
  page.on("pageerror", (e) => errors.push(`pageerror: ${e.message}`));
  // The browser logs the one rejected configuration below as a failed request.
  page.on("console", (m) => {
    if (m.type() === "error" && !m.text().includes("status of 422")) errors.push(`console: ${m.text()}`);
  });

  // Internet → app → primary and storage, built through the API. Only
  // rulesets before v6 have the Internet; the dashboard still opens them.
  const api = page.request;
  const created = await (await api.post("/api/forgelab/sandbox/games", { data: { seed: 3, ruleset: "sandbox/v5" } })).json();
  const game = `/api/forgelab/sandbox/games/${created.id}`;
  const command = async (c: object) => {
    const res = await api.post(`${game}/commands`, { data: c });
    expect(res.ok(), await res.text()).toBe(true);
    return (await res.json()) as { node?: string };
  };
  const app = (await command({ type: "place", kind: "app-instance", x: 300, y: 0 })).node!;
  const db = (await command({ type: "place", kind: "db-primary", x: 600, y: -100 })).node!;
  const st = (await command({ type: "place", kind: "object-storage", x: 600, y: 100 })).node!;
  await command({ type: "connect", from: "internet", to: app });
  await command({ type: "connect", from: app, to: db });
  await command({ type: "connect", from: app, to: st });

  await page.goto("/sandbox");
  await page.evaluate((id) => localStorage.setItem("forgelab.sandbox.game", id), created.id);
  await page.reload();
  await node(page, "internet").click();
  await expect(page.locator(".sb-inspector h3")).toHaveText("Internet");
  await expect(page.locator(".sb-inspector")).toContainText("Market (your users)");
  // Clicking the Internet also opens it; go back to the system for now.
  await page.keyboard.press("Escape");

  // Switch to a load test with a mistake in the group shares: the engine
  // rejects it, the form says why, and nothing changes.
  await page.getByRole("button", { name: "Configure traffic" }).click();
  const dialog = page.getByRole("dialog", { name: "Internet traffic" });
  await expect(dialog).toBeVisible();
  await dialog.getByLabel(/^Load test/).check();
  await dialog.getByLabel("Requests/s").fill("30");
  await dialog.getByLabel("Group 1 share %").fill("60");
  await expect(dialog.locator(".sb-total").last()).toHaveText("Groups: 90% (must be 100%)");
  await dialog.getByRole("button", { name: "Apply" }).click();
  await expect(dialog.getByRole("alert")).toContainText("traffic group shares must sum to 100% (now 90%)");
  await expect(page.locator(".badge.load-test")).toHaveCount(0);

  // Fix it and apply: the load test drives the flow and is labelled.
  await dialog.getByLabel("Group 1 share %").fill("70");
  await dialog.getByRole("button", { name: "Apply" }).click();
  await expect(dialog).toBeHidden();
  await expect(page.locator(".badge.load-test")).toBeVisible();
  await expect(node(page, "internet")).toContainText("load test");
  await expect(page.locator(".sb-goals")).toContainText("Paused during the load test");
  await expect.poll(() => tile(page, "RPS")).toBe("30.0");
  await page.getByText("Traffic breakdown").click();
  await expect(page.locator(".sb-breakdown tr", { hasText: "Web users" })).toContainText("21.0/s");
  await expect(page.locator(".sb-breakdown tr", { hasText: "asia" })).toContainText("12.0/s");

  // Clicking the Internet opens it: regions, groups, and endpoints with
  // animated traffic. Esc and the back button return to the system.
  const inside = page.getByLabel("Inside the Internet");
  await node(page, "internet").click();
  await expect(inside).toBeVisible();
  await expect(inside.locator(".react-flow__node", { hasText: "Web users" })).toContainText("21.0/s");
  await expect(inside.locator(".react-flow__node", { hasText: "asia" })).toContainText("12.0/s");
  await expect(inside.locator(".react-flow__edge.animated").first()).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(inside).toHaveCount(0);
  await node(page, "internet").click();
  await inside.getByRole("button", { name: "← System" }).click();
  await expect(inside).toHaveCount(0);

  // Back to the market: users drive the volume again.
  await page.getByRole("button", { name: "Configure traffic" }).click();
  await dialog.getByLabel(/^Market/).check();
  await dialog.getByRole("button", { name: "Apply" }).click();
  await expect(dialog).toBeHidden();
  await expect(page.locator(".badge.load-test")).toHaveCount(0);
  await expect(page.locator(".sb-inspector")).toContainText("Market (your users)");

  await expect(page.locator(".sb-toast")).toHaveCount(0);
  expect(errors, "browser errors").toEqual([]);
});

test("an application instance shows why it is slow and can be reconfigured", async ({ page }) => {
  const errors: string[] = [];
  page.on("pageerror", (e) => errors.push(`pageerror: ${e.message}`));
  // The browser logs the one rejected configuration below as a failed request.
  page.on("console", (m) => {
    if (m.type() === "error" && !m.text().includes("status of 422")) errors.push(`console: ${m.text()}`);
  });

  const api = page.request;
  const created = await (await api.post("/api/forgelab/sandbox/games", { data: { seed: 3 } })).json();
  const game = `/api/forgelab/sandbox/games/${created.id}`;
  const command = async (c: object) => {
    const res = await api.post(`${game}/commands`, { data: c });
    expect(res.ok(), await res.text()).toBe(true);
    return (await res.json()) as { node?: string };
  };
  const src = (await command({ type: "place", kind: "traffic", x: 0, y: 0 })).node!;
  const app = (await command({ type: "place", kind: "app-instance", x: 300, y: 0 })).node!;
  const db = (await command({ type: "place", kind: "db-primary", x: 600, y: -100 })).node!;
  const st = (await command({ type: "place", kind: "object-storage", x: 600, y: 100 })).node!;
  await command({ type: "connect", from: src, to: app });
  await command({ type: "connect", from: app, to: db });
  await command({ type: "connect", from: app, to: st });
  await api.post(`${game}/step`, { data: { ticks: 2 } });

  await page.goto("/sandbox");
  await page.evaluate((id) => localStorage.setItem("forgelab.sandbox.game", id), created.id);
  await page.reload();
  await node(page, app).click();
  const inspector = page.locator(".sb-inspector");
  await expect(inspector.locator("tr", { hasText: "Health" })).toContainText("healthy");
  await expect(inspector.locator("tr", { hasText: "Bottleneck" })).toContainText("CPU");

  // Zero workers is rejected with the reason; one sync worker moves the bottleneck.
  await inspector.getByRole("button", { name: "Configure app" }).click();
  const dialog = page.getByRole("dialog", { name: "Application instance" });
  await dialog.getByLabel("Workers").fill("0");
  await dialog.getByRole("button", { name: "Apply" }).click();
  await expect(dialog.getByRole("alert")).toContainText("workers must be between 1 and 64");
  await dialog.getByLabel("Workers").fill("1");
  await dialog.getByRole("button", { name: "Apply" }).click();
  await expect(dialog).toBeHidden();
  await expect(inspector.locator("tr", { hasText: "Bottleneck" })).toContainText("workers / concurrency slots");
  await page.getByText("Routes", { exact: true }).click();
  await expect(inspector.locator(".sb-breakdown tr", { hasText: "GET /products" }).first()).toBeVisible();

  // The app is open on the canvas: a request's path through it, with the
  // engine's values and what each connection carried (the database over
  // SQL, storage over S3). Esc and the back button return to the system.
  const inside = page.getByLabel(`Inside ${app}`);
  await expect(inside).toBeVisible();
  await expect(inside.locator(".react-flow__node", { hasText: "1 sync workers" })).toContainText("in flight");
  await expect(inside.locator(".react-flow__node", { hasText: "Backlog" })).toContainText("/ 100 queued");
  await expect(inside.locator(".react-flow__node", { hasText: "GET /products" }).first()).toContainText("/s");
  await expect(inside.locator(".react-flow__node", { hasText: db })).toContainText("SQL");
  await expect(inside.locator(".react-flow__node", { hasText: st })).toContainText("S3");
  await expect(inside.locator(".react-flow__node", { hasText: "Middleware" })).toContainText("CPU-ms per request");
  await expect(inside.locator(".react-flow__edge.animated")).not.toHaveCount(0);
  await page.keyboard.press("Escape");
  await expect(inside).toHaveCount(0);
  await expect(inspector.locator("tr", { hasText: "Health" })).toBeVisible();
  await node(page, app).click();
  await inside.getByRole("button", { name: "← System" }).click();
  await expect(inside).toHaveCount(0);

  await expect(page.locator(".sb-toast")).toHaveCount(0);
  expect(errors, "browser errors").toEqual([]);
});

test("traffic components connect under a contract and add up", async ({ page }) => {
  const errors: string[] = [];
  page.on("pageerror", (e) => errors.push(`pageerror: ${e.message}`));
  page.on("console", (m) => {
    if (m.type() === "error" && !m.text().includes("status of 422")) errors.push(`console: ${m.text()}`);
  });

  // Two traffic components → one app → primary and storage.
  const api = page.request;
  const created = await (await api.post("/api/forgelab/sandbox/games", { data: { seed: 5 } })).json();
  const game = `/api/forgelab/sandbox/games/${created.id}`;
  const command = async (c: object) => {
    const res = await api.post(`${game}/commands`, { data: c });
    expect(res.ok(), await res.text()).toBe(true);
    return (await res.json()) as { node?: string };
  };
  const web = (await command({ type: "place", kind: "traffic", x: 0, y: -100 })).node!;
  const mobile = (await command({ type: "place", kind: "traffic", x: 0, y: 100 })).node!;
  const app = (await command({ type: "place", kind: "app-instance", x: 300, y: 0 })).node!;
  const db = (await command({ type: "place", kind: "db-primary", x: 600, y: -100 })).node!;
  const st = (await command({ type: "place", kind: "object-storage", x: 600, y: 100 })).node!;
  for (const [from, to] of [[web, app], [mobile, app], [app, db], [app, st]]) await command({ type: "connect", from, to });
  await api.post(`${game}/step`, { data: { ticks: 2 } });

  await page.goto("/sandbox");
  await page.evaluate((id) => localStorage.setItem("forgelab.sandbox.game", id), created.id);
  await page.reload();
  await expect(page.locator(".react-flow__edge")).toHaveCount(4);

  // Point the mobile clients at the wrong port: every request is refused,
  // and the node and its edge say why.
  await node(page, mobile).click();
  await page.keyboard.press("Escape");
  const inspector = page.locator(".sb-inspector");
  await expect(inspector.locator("h3")).toHaveText("Traffic");
  await inspector.getByRole("button", { name: "Configure traffic" }).click();
  const dialog = page.getByRole("dialog", { name: "Traffic component" });
  // Connecting adopted the app's connection and one endpoint per route.
  await expect(dialog.getByLabel("Port")).toHaveValue("8000");
  await expect(dialog.getByLabel("Scheme")).toHaveValue("https");
  await expect(dialog.getByLabel("Endpoint 1 path")).toHaveValue("/products");
  await expect(dialog.getByLabel("Endpoint 6 path")).toHaveValue("/login");
  await expect(dialog.locator(".sb-total")).toHaveText("Requests: 100%");
  await expect(dialog.getByLabel("Endpoint 1 %")).toHaveValue("35");
  await dialog.getByLabel("Name").fill("Mobile users");
  await dialog.getByLabel("Client type").selectOption("mobile");
  await dialog.getByLabel("Port").fill("8080");
  await dialog.getByLabel("Endpoint 1 %").fill("10");
  await dialog.getByRole("button", { name: "Apply" }).click();
  await expect(dialog.getByRole("alert")).toContainText("endpoint shares must sum to 100%");
  await dialog.getByLabel("Endpoint 1 %").fill("35");
  await dialog.getByRole("button", { name: "Apply" }).click();
  await expect(dialog).toBeHidden();
  const refused = "connection refused: port 8080, the app listens on 8000";
  await expect(node(page, mobile)).toContainText(refused);
  await expect(inspector).toContainText(refused);
  await expect(page.locator(".react-flow__edge-text", { hasText: refused })).toBeVisible();

  // The app lists its inputs, and its inside view shows the refused one.
  await node(page, app).click();
  const inside = page.getByLabel(`Inside ${app}`);
  await expect(inside.locator(".react-flow__node", { hasText: mobile })).toContainText("connection refused");
  await expect(inside.locator(".react-flow__node", { hasText: web })).toContainText("/s");
  await page.keyboard.press("Escape");
  await page.getByText("Inputs", { exact: true }).click();
  await expect(inspector.locator(".sb-breakdown tr", { hasText: mobile })).toContainText("refused");

  // Fix the port: both components send, and the meters add them up.
  await node(page, mobile).click();
  await page.keyboard.press("Escape");
  await inspector.getByRole("button", { name: "Configure traffic" }).click();
  await dialog.getByLabel("Port").fill("8000");
  await dialog.getByRole("button", { name: "Apply" }).click();
  await expect(dialog).toBeHidden();
  await expect(node(page, mobile)).not.toContainText("refused");
  const state = await (await api.get(game)).json();
  const sum = state.flow.traffic.components.reduce((a: number, c: { rps: number }) => a + c.rps, 0);
  expect(state.flow.traffic.components).toHaveLength(2);
  expect(Math.abs(sum - state.flow.rps)).toBeLessThan(1e-9);

  // Inside a traffic component: its clients, requests, and connection.
  await node(page, web).click();
  const view = page.getByLabel(`Inside ${web}`);
  await expect(view.locator(".sb-inner-bar")).toContainText("HTTP/1.1 · https:8000");
  await expect(view.locator(".react-flow__node", { hasText: app })).toContainText("ok");
  await expect(view.locator(".react-flow__edge-text")).toHaveCount(0);
  await view.getByRole("button", { name: "← System" }).click();

  // A third app from a domain template: its routes and their typical shares
  // are what a traffic component adopts.
  await page.locator(".sb-kind", { hasText: "Application instance" }).click();
  const types = page.getByRole("dialog", { name: "New application instance" });
  await types.getByRole("radio", { name: /^Ride sharing/ }).check();
  await types.getByRole("radio", { name: /^Go/ }).check();
  await expect(types).toContainText("POST /drivers/location 50%");
  await types.getByRole("button", { name: "Place" }).click();
  await expect(types).toBeHidden();
  await expect(inspector).toContainText("Ride sharing API · Go / net/http · async");

  // A second app, defined by hand: the engine checks the form before it is placed.
  await page.keyboard.press("Escape");
  await page.locator(".sb-kind", { hasText: "Application instance" }).click();
  const chooser = page.getByRole("dialog", { name: "New application instance" });
  await chooser.getByRole("button", { name: "define everything manually" }).click();
  await expect(chooser.getByLabel("Workers")).toBeVisible();
  await chooser.getByLabel("Workers").fill("0");
  await chooser.getByRole("button", { name: "Apply" }).click();
  await expect(chooser.getByRole("alert")).toContainText("workers must be between 1 and 64");
  await expect(page.locator(".sb-canvas .react-flow__node")).toHaveCount(6);
  await chooser.getByLabel("Workers").fill("2");
  await chooser.getByRole("button", { name: "Apply" }).click();
  await expect(chooser).toBeHidden();
  await expect(page.locator(".sb-canvas .react-flow__node")).toHaveCount(7);
  await expect(inspector).toContainText("sync, 2 workers");

  // Disconnecting returns it to asking for nothing.
  await node(page, web).click();
  await page.keyboard.press("Escape");
  await inspector.locator(".sb-links li", { hasText: app }).getByRole("button", { name: "disconnect" }).click();
  await expect(node(page, web)).toContainText("not connected");
  await expect(inspector).toContainText("Connect it to an application instance");

  await expect(page.locator(".sb-toast")).toHaveCount(0);
  expect(errors, "browser errors").toEqual([]);
});

test("services call each other over configured connections", async ({ page }) => {
  const errors: string[] = [];
  page.on("pageerror", (e) => errors.push(`pageerror: ${e.message}`));
  page.on("console", (m) => {
    if (m.type() === "error" && !m.text().includes("status of 422")) errors.push(`console: ${m.text()}`);
  });

  // A storefront calling a catalog service, each with its own database,
  // built from the templates through the API.
  const api = page.request;
  const rules = await (await api.get("/api/forgelab/sandbox/ruleset")).json();
  const created = await (await api.post("/api/forgelab/sandbox/games", { data: { seed: 9 } })).json();
  const game = `/api/forgelab/sandbox/games/${created.id}`;
  const command = async (c: object) => {
    const res = await api.post(`${game}/commands`, { data: c });
    expect(res.ok(), await res.text()).toBe(true);
    return (await res.json()) as { node?: string };
  };
  const type = (name: string) => rules.appTypes.find((t: { name: string }) => t.name === name);
  const app = (name: string, x: number, y: number) =>
    command({ type: "place", kind: "app-instance", x, y, app: { ...rules.appStacks[4].app, name: `${name} API`, routes: type(name).routes } });
  const front = (await app("Storefront", 300, 0)).node!;
  const catalog = (await app("Catalog", 600, -100)).node!;
  const db = (await command({ type: "place", kind: "db-primary", x: 900, y: -100 })).node!;
  const src = (await command({ type: "place", kind: "traffic", x: 0, y: 0 })).node!;
  for (const [from, to] of [[src, front], [front, catalog], [catalog, db]]) await command({ type: "connect", from, to });
  await api.post(`${game}/step`, { data: { ticks: 2 } });

  await page.goto("/sandbox");
  await page.evaluate((id) => localStorage.setItem("forgelab.sandbox.game", id), created.id);
  await page.reload();
  await expect(page.locator(".react-flow__edge")).toHaveCount(3);

  // The app's inside view lists what each connection carried.
  await node(page, front).click();
  const inside = page.getByLabel(`Inside ${front}`);
  await expect(inside.locator(".react-flow__node", { hasText: `${catalog} (Catalog API)` })).toContainText("HTTP/1.1");
  await page.keyboard.press("Escape");

  // Select the catalog's database connection: SQL on 5432, adopted. Break
  // its port: the edge says why, and the catalog's calls fail.
  await page.locator(`.react-flow__edge[data-id="${catalog}->${db}"]`).click({ force: true });
  const inspector = page.locator(".sb-inspector");
  await expect(inspector.locator("h3")).toHaveText("Connection");
  await expect(inspector).toContainText("SQL · port 5432 · no TLS · pool 20");
  await inspector.getByRole("button", { name: "Configure connection" }).click();
  const dialog = page.getByRole("dialog", { name: "Connection" });
  await dialog.getByLabel("Pool (per replica)").fill("0");
  await dialog.getByRole("button", { name: "Apply" }).click();
  await expect(dialog.getByRole("alert")).toContainText("pool must be between 1 and 1000");
  await dialog.getByLabel("Pool (per replica)").fill("10");
  await dialog.getByLabel("Port").fill("5433");
  await dialog.getByRole("button", { name: "Apply" }).click();
  await expect(dialog).toBeHidden();
  const refused = `connection refused: port 5433, ${db} listens on 5432`;
  await expect(inspector).toContainText(refused);
  await expect(page.locator(".react-flow__edge-text", { hasText: refused })).toBeVisible();

  // Moving the database's listener makes the connection follow it again.
  await node(page, db).click();
  await page.keyboard.press("Escape");
  await inspector.getByRole("button", { name: "change" }).click();
  await inspector.getByLabel("Listener port").fill("5433");
  await inspector.getByRole("button", { name: "Apply" }).click();
  await expect(inspector).toContainText("Listens on SQL · port 5433");
  await expect(page.locator(".react-flow__edge-text", { hasText: refused })).toHaveCount(0);

  await expect(page.locator(".sb-toast")).toHaveCount(0);
  expect(errors, "browser errors").toEqual([]);
});

test("a database shows where its time goes and can be reconfigured", async ({ page }) => {
  const errors: string[] = [];
  page.on("pageerror", (e) => errors.push(`pageerror: ${e.message}`));
  page.on("console", (m) => {
    if (m.type() === "error" && !m.text().includes("status of 422")) errors.push(`console: ${m.text()}`);
  });

  const api = page.request;
  const created = await (await api.post("/api/forgelab/sandbox/games", { data: { seed: 4 } })).json();
  const game = `/api/forgelab/sandbox/games/${created.id}`;
  const command = async (c: object) => {
    const res = await api.post(`${game}/commands`, { data: c });
    expect(res.ok(), await res.text()).toBe(true);
    return (await res.json()) as { node?: string };
  };
  const src = (await command({ type: "place", kind: "traffic", x: 0, y: 0 })).node!;
  const app = (await command({ type: "place", kind: "app-instance", x: 300, y: 0 })).node!;
  const db = (await command({ type: "place", kind: "db-primary", x: 600, y: -100 })).node!;
  const st = (await command({ type: "place", kind: "object-storage", x: 600, y: 100 })).node!;
  for (const [from, to] of [[src, app], [app, db], [app, st]]) await command({ type: "connect", from, to });
  await command({ type: "scale", node: app, replicas: 3 });
  await api.post(`${game}/step`, { data: { ticks: 2 } });

  await page.goto("/sandbox");
  await page.evaluate((id) => localStorage.setItem("forgelab.sandbox.game", id), created.id);
  await page.reload();
  await node(page, db).click();

  // Inside: connections, queue, CPU, buffer cache, disk, and response.
  const inside = page.getByLabel(`Inside ${db}`);
  await expect(inside.locator(".react-flow__node", { hasText: "Buffer cache" })).toContainText("100% hits");
  await expect(inside.locator(".react-flow__node", { hasText: "Connections" })).toContainText("60 / 100 open");
  await page.keyboard.press("Escape");

  // The inspector shows the engine's values; fewer connections than the
  // callers' pools open refuses the rest.
  const inspector = page.locator(".sb-inspector");
  await expect(inspector.locator("tr", { hasText: "Bottleneck" })).toContainText("CPU");
  await inspector.getByRole("button", { name: "Configure database" }).click();
  const dialog = page.getByRole("dialog", { name: "Database" });
  await dialog.getByLabel("Max connections").fill("0");
  await dialog.getByRole("button", { name: "Apply" }).click();
  await expect(dialog.getByRole("alert")).toContainText("max connections must be between 1 and 10000");
  await dialog.getByLabel("Max connections").fill("30");
  await dialog.getByRole("button", { name: "Apply" }).click();
  await expect(dialog).toBeHidden();
  await expect(inspector.locator("tr", { hasText: "Connections" })).toContainText("60 / 30");
  await expect(inspector.locator("tr", { hasText: "Refused" })).toBeVisible();

  await expect(page.locator(".sb-toast")).toHaveCount(0);
  expect(errors, "browser errors").toEqual([]);
});
