"use client";

import "@xyflow/react/dist/style.css";
import {
  Background,
  Controls,
  MiniMap,
  ReactFlow,
  ReactFlowProvider,
  useNodesState,
  useReactFlow,
  useUpdateNodeInternals,
  type Connection,
  type Edge as FlowEdge,
  type IsValidConnection,
} from "@xyflow/react";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  canConnect,
  clientConfig,
  formatMoney,
  freeSpot,
  internetConfig,
  newest,
  routedStorage,
  sandboxApi,
  type AppConfig,
  type Command,
  type GameState,
  type Ruleset,
} from "@/lib/sandbox";
import { SandboxEvents } from "./SandboxEvents";
import { AppView, NewAppDialog } from "./SandboxApp";
import { EdgePanel } from "./SandboxConn";
import { CacheView } from "./SandboxCache";
import { DbView } from "./SandboxDb";
import { QueueView } from "./SandboxQueue";
import { StorageView } from "./SandboxStorage";
import { StreamView } from "./SandboxStream";
import { EdgeView } from "./SandboxEdge";
import { InternetView } from "./SandboxInternet";
import { TrafficView } from "./SandboxTraffic";
import { SandboxGoals } from "./SandboxGoals";
import { SandboxHud } from "./SandboxHud";
import { SandboxNode, type SandboxFlowNode } from "./SandboxNode";
import { KIND_DRAG_TYPE, SandboxInspector, SandboxPalette } from "./SandboxPanels";

const STORAGE_KEY = "forgelab.sandbox.game";
const nodeTypes = { component: SandboxNode };

function remember(id: string | null) {
  try {
    if (id) localStorage.setItem(STORAGE_KEY, id);
    else localStorage.removeItem(STORAGE_KEY);
  } catch {
    // storage is a convenience only
  }
}

function recall(): string | null {
  try {
    return localStorage.getItem(STORAGE_KEY);
  } catch {
    return null;
  }
}

// useGameStream keeps the game state live: Server-Sent Events first, polling
// if the stream cannot be held open.
function useGameStream(id: string | undefined, onState: (g: GameState) => void) {
  useEffect(() => {
    if (!id) return;
    let failures = 0;
    let poll: ReturnType<typeof setInterval> | undefined;
    const es = new EventSource(sandboxApi.streamUrl(id));
    es.onmessage = (e) => {
      failures = 0;
      onState(JSON.parse(e.data) as GameState);
    };
    es.onerror = () => {
      if (++failures < 3 || poll) return;
      es.close();
      poll = setInterval(() => {
        sandboxApi.get(id).then(onState, () => undefined);
      }, 1000);
    };
    return () => {
      es.close();
      if (poll) clearInterval(poll);
    };
  }, [id, onState]);
}

function Board({ rules, initial, onNewGame }: { rules: Ruleset; initial: GameState; onNewGame: () => void }) {
  const [game, setGame] = useState<GameState>(initial);
  const [toast, setToast] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  // The connection the inspector shows, as "from->to".
  const [edgeSel, setEdgeSel] = useState<string | null>(null);
  const selectedEdge = game.edges.find((e) => `${e.from}->${e.to}` === edgeSel);
  // Where an application instance waits to be placed while its template is chosen.
  const [newApp, setNewApp] = useState<{ x: number; y: number } | null>(null);
  // The component whose inside the canvas shows instead of the system.
  const [inside, setInside] = useState<string | null>(null);
  const back = useCallback(() => setInside(null), []);
  const opened = game.nodes.find((n) => n.id === inside);
  // Goals already reached when the board opened, or announced since.
  const announced = useRef<Set<string> | null>(null);
  const [nodes, setNodes, onNodesChange] = useNodesState<SandboxFlowNode>([]);
  const canvas = useRef<HTMLDivElement>(null);
  const { screenToFlowPosition, getInternalNode } = useReactFlow();
  const updateNodeInternals = useUpdateNodeInternals();
  // A node just placed by the player becomes the selection once it arrives.
  const pendingSelect = useRef<string | null>(null);
  // React Flow owns selection; the inspector follows it.
  const selected = nodes.find((n) => n.selected)?.id ?? null;
  const traffic = internetConfig(game, rules);

  const onState = useCallback((g: GameState) => setGame((cur) => newest(cur, g)), []);
  useGameStream(game.id, onState);

  useEffect(() => {
    if (!toast) return;
    const t = setTimeout(() => setToast(null), 4000);
    return () => clearTimeout(t);
  }, [toast]);

  useEffect(() => {
    if (!notice) return;
    const t = setTimeout(() => setNotice(null), 6000);
    return () => clearTimeout(t);
  }, [notice]);

  // Announce goals reached while the player watches.
  useEffect(() => {
    const reached = (game.goals ?? []).filter((g) => g.achievedAt !== undefined);
    if (!announced.current) {
      announced.current = new Set(reached.map((g) => g.id));
      return;
    }
    const fresh = reached.filter((g) => !announced.current!.has(g.id));
    if (fresh.length === 0) return;
    fresh.forEach((g) => announced.current!.add(g.id));
    const label = (kind: string) => rules.kinds.find((k) => k.name === kind)?.label ?? kind;
    const unlocked = fresh.flatMap((g) => g.unlocks ?? []).map(label);
    setNotice(
      `Goal reached: ${fresh.map((g) => g.title).join(", ")}.${unlocked.length > 0 ? ` Unlocked ${unlocked.join(", ")}.` : ""}`,
    );
  }, [game.goals, rules.kinds]);

  // Mirror server nodes into React Flow, keeping the selection and the
  // position of a node the player is dragging.
  useEffect(() => {
    setNodes((prev) => {
      const pending = pendingSelect.current;
      return game.nodes.map((n) => {
        const old = prev.find((p) => p.id === n.id);
        const kind = rules.kinds.find((k) => k.name === n.kind);
        return {
          // Keep React Flow's own fields (measured size, dragging); our copy
          // may not have the measurement yet right after placement.
          ...old,
          measured: old?.measured ?? getInternalNode(n.id)?.measured,
          id: n.id,
          type: "component",
          position: old?.dragging ? old.position : { x: n.x, y: n.y },
          selected: pending ? n.id === pending : (old?.selected ?? false),
          deletable: n.kind !== "internet",
          data: {
            // A traffic component is titled by the population it is, and an
            // application by its service name once it has one of its own.
            label: (n.kind === "traffic" && clientConfig(game, rules, n)?.name) || (n.kind === "app-instance" && n.app?.name) || (kind?.label ?? n.kind),
            kind: n.kind,
            size: n.size,
            replicas: n.replicas,
            downReplicas: n.downReplicas ?? 0,
            down: !!n.down,
            rateLimited: !!n.rateLimited,
            loadTest: (n.traffic ?? clientConfig(game, rules, n))?.source === "configured",
            source: (kind?.connectsTo?.length ?? 0) > 0,
            // Only kinds something can send to take connections.
            target: rules.kinds.some((k) => k.connectsTo?.includes(n.kind)),
            stats: game.flow.nodes.find((s) => s.id === n.id),
          },
        };
      });
    });
  }, [game, rules, setNodes, getInternalNode]);

  // React Flow drops a node's handle positions whenever it receives the node
  // without a measured size, and never re-measures a node whose size did not
  // change; such a node draws no edges and cannot be wired. Heal it.
  useEffect(() => {
    const lost = nodes.filter((n) => n.measured && !getInternalNode(n.id)?.internals.handleBounds).map((n) => n.id);
    if (lost.length > 0) updateNodeInternals(lost);
  }, [nodes, getInternalNode, updateNodeInternals]);

  useEffect(() => {
    if (pendingSelect.current && selected === pendingSelect.current) pendingSelect.current = null;
  }, [selected]);

  const edges: FlowEdge[] = useMemo(
    () =>
      game.edges.map((e) => {
        const from = game.flow.nodes.find((s) => s.id === e.from);
        const to = game.flow.nodes.find((s) => s.id === e.to);
        const saturated = (to?.utilization ?? 0) >= 1 || (to?.capacity === 0 && (to?.offered ?? 0) > 0);
        // A connection that breaks its contract says why on the edge; one
        // whose calls fail is marked (v7 reports every connection).
        const es = game.flow.edges?.find((x) => x.from === e.from && x.to === e.to);
        const problem = es?.problem ?? from?.traffic?.problem;
        const failing = !!es && es.rps > 0 && es.errors / es.rps > 0.01;
        const id = `${e.from}->${e.to}`;
        return {
          id,
          source: e.from,
          target: e.to,
          animated: !problem && (from?.served ?? 0) > 0 && (to?.offered ?? 0) > 0,
          className: [saturated || problem || failing ? "sb-edge-bad" : "", edgeSel === id ? "sb-edge-selected" : ""].join(" ").trim() || undefined,
          label: problem,
          interactionWidth: 16,
        };
      }),
    [game, edgeSel],
  );

  const send = useCallback(
    async (c: Command) => {
      try {
        const res = await sandboxApi.command(game.id, c);
        onState(res.state);
        return res.node;
      } catch (err) {
        setToast(err instanceof Error ? err.message : String(err));
      }
    },
    [game.id, onState],
  );

  // Configuring the Internet reports a rejection to its form, not as a toast:
  // a configuration can have several problems to fix at once.
  const configure = useCallback(
    async (c: Command) => {
      try {
        const res = await sandboxApi.command(game.id, c);
        onState(res.state);
        return null;
      } catch (err) {
        return err instanceof Error ? err.message : String(err);
      }
    },
    [game.id, onState],
  );

  const select = useCallback(
    (id: string) => {
      pendingSelect.current = id;
      setNodes((prev) => prev.map((p) => ({ ...p, selected: p.id === id })));
    },
    [setNodes],
  );

  const place = useCallback(
    async (kind: string, position?: { x: number; y: number }) => {
      let pos = position;
      if (!pos) {
        const r = canvas.current?.getBoundingClientRect();
        const centre = screenToFlowPosition({ x: (r?.left ?? 0) + (r?.width ?? 0) / 2, y: (r?.top ?? 0) + (r?.height ?? 0) / 2 });
        pos = freeSpot(centre, game.nodes);
      }
      const at = { x: Math.round(pos.x), y: Math.round(pos.y) };
      // An application instance starts from a template or a hand-made
      // configuration, chosen before it is placed.
      if (kind === "app-instance" && rules.appStacks?.length) {
        setNewApp(at);
        return;
      }
      const id = await send({ type: "place", kind, ...at });
      if (id) select(id);
    },
    [screenToFlowPosition, send, game.nodes, rules.appStacks, select],
  );

  const placeApp = useCallback(
    async (app: AppConfig) => {
      if (!newApp) return null;
      try {
        const res = await sandboxApi.command(game.id, { type: "place", kind: "app-instance", ...newApp, app });
        onState(res.state);
        if (res.node) select(res.node);
        return null;
      } catch (err) {
        return err instanceof Error ? err.message : String(err);
      }
    },
    [game.id, newApp, onState, select],
  );

  const isValidConnection: IsValidConnection = useCallback(
    (c) => {
      const from = game.nodes.find((n) => n.id === c.source);
      const to = game.nodes.find((n) => n.id === c.target);
      return !!from && !!to && canConnect(rules.kinds, from.kind, to.kind);
    },
    [game.nodes, rules.kinds],
  );

  async function control(action: () => Promise<GameState>) {
    try {
      onState(await action());
    } catch (err) {
      setToast(err instanceof Error ? err.message : String(err));
    }
  }

  return (
    <div className="sandbox">
      <SandboxHud
        game={game}
        onSpeed={(s) => control(() => sandboxApi.speed(game.id, s))}
        onSkip={(t) => control(() => sandboxApi.step(game.id, t))}
      />
      <div className="sb-strips">
        <SandboxEvents game={game} rules={rules} />
        <SandboxGoals game={game} rules={rules} />
      </div>
      {game.status === "bankrupt" && (
        <div className="notice sb-over">
          <strong>Bankrupt.</strong> Cash stayed negative for a full day. <button onClick={onNewGame}>New game</button>
        </div>
      )}
      <div className="sb-layout">
        <SandboxPalette rules={rules} cash={game.meters.cash} goals={game.goals ?? []} onPlace={(k) => place(k)} />
        <div
          className="sb-canvas"
          ref={canvas}
          onDragOver={(e) => {
            e.preventDefault();
            e.dataTransfer.dropEffect = "move";
          }}
          onDrop={(e) => {
            e.preventDefault();
            const kind = e.dataTransfer.getData(KIND_DRAG_TYPE);
            if (kind) place(kind, screenToFlowPosition({ x: e.clientX, y: e.clientY }));
          }}
        >
          <ReactFlow
            nodes={nodes}
            edges={edges}
            nodeTypes={nodeTypes}
            onNodesChange={onNodesChange}
            onNodeDragStop={(_, n) => send({ type: "move", node: n.id, x: Math.round(n.position.x), y: Math.round(n.position.y) })}
            onConnect={(c: Connection) => send({ type: "connect", from: c.source, to: c.target })}
            isValidConnection={isValidConnection}
            onDelete={({ nodes: removed, edges: cut }) => {
              // Removing a node removes its connections on the server too.
              const gone = new Set(removed.map((n) => n.id));
              removed.forEach((n) => send({ type: "remove", node: n.id }));
              cut
                .filter((e) => !gone.has(e.source) && !gone.has(e.target))
                .forEach((e) => send({ type: "disconnect", from: e.source, to: e.target }));
            }}
            onNodeClick={(_, n) => {
              setEdgeSel(null);
              if (["internet", "traffic", "app-instance"].includes(n.data.kind) || n.data.stats?.db || n.data.stats?.cache || n.data.stats?.storage || n.data.stats?.queue || n.data.stats?.stream || n.data.stats?.edge) setInside(n.id);
            }}
            onEdgeClick={(_, e) => {
              setNodes((prev) => prev.map((p) => ({ ...p, selected: false })));
              setEdgeSel(e.id);
            }}
            onPaneClick={() => setEdgeSel(null)}
            deleteKeyCode={["Backspace", "Delete"]}
            colorMode="system"
            fitView
            fitViewOptions={{ maxZoom: 1, padding: 0.4 }}
          >
            <Background />
            <Controls showInteractive={false} fitViewOptions={{ maxZoom: 1, padding: 0.4 }} />
            <MiniMap pannable zoomable />
          </ReactFlow>
          {opened?.kind === "app-instance" && <AppView game={game} rules={rules} node={opened} onBack={back} />}
          {opened && game.flow.nodes.find((s) => s.id === opened.id)?.db && <DbView game={game} node={opened} onBack={back} />}
          {opened && game.flow.nodes.find((s) => s.id === opened.id)?.cache && <CacheView game={game} node={opened} onBack={back} />}
          {opened && game.flow.nodes.find((s) => s.id === opened.id)?.storage && <StorageView game={game} node={opened} onBack={back} />}
          {opened && game.flow.nodes.find((s) => s.id === opened.id)?.queue && <QueueView game={game} node={opened} onBack={back} />}
          {opened && game.flow.nodes.find((s) => s.id === opened.id)?.stream && <StreamView game={game} node={opened} onBack={back} />}
          {opened && game.flow.nodes.find((s) => s.id === opened.id)?.edge && <EdgeView game={game} node={opened} onBack={back} />}
          {opened?.kind === "traffic" && <TrafficView game={game} rules={rules} node={opened} onBack={back} />}
          {inside === "internet" && traffic && <InternetView config={traffic} game={game} routed={routedStorage(game, rules)} onBack={back} />}
          {newApp && <NewAppDialog rules={rules} onPlace={placeApp} onClose={() => setNewApp(null)} />}
          {toast && (
            <div className="sb-toast" role="alert">
              {toast}
            </div>
          )}
          {notice && (
            <div className="sb-notice" role="status">
              {notice}
            </div>
          )}
        </div>
        {selectedEdge && !selected ? (
          <EdgePanel game={game} rules={rules} from={selectedEdge.from} to={selectedEdge.to} onConfigure={configure} onCommand={send} />
        ) : (
          <SandboxInspector game={game} rules={rules} selected={selected} onCommand={send} onConfigure={configure} />
        )}
      </div>
      <div className="sb-footer">
        <span className="legend">
          {game.id} · seed {game.seed} · ruleset {game.ruleset}
          {game.freeBuild ? " · free build" : ""} · tick {game.tick}
        </span>
        <button
          className="secondary"
          onClick={() =>
            sandboxApi.save(game.id).then(
              (r) => setToast(`Saved to ${r.path}`),
              (err: Error) => setToast(err.message),
            )
          }
        >
          Save
        </button>
        <button className="secondary" onClick={onNewGame}>
          New game
        </button>
      </div>
    </div>
  );
}

export function SandboxGame() {
  const [rules, setRules] = useState<Ruleset | null>(null);
  const [game, setGame] = useState<GameState | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [freeBuild, setFreeBuild] = useState(false);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const latest = await sandboxApi.ruleset();
        const id = recall();
        const g = id ? await sandboxApi.get(id).catch(() => null) : null;
        // An older game is shown with its own ruleset's catalog and defaults.
        const r = g && g.ruleset !== latest.version ? await sandboxApi.ruleset(g.ruleset) : latest;
        if (cancelled) return;
        setRules(r);
        setGame(g);
      } catch (err) {
        if (!cancelled) setError(err instanceof Error ? err.message : String(err));
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  async function newGame() {
    try {
      const previous = game?.id;
      const g = await sandboxApi.create(game ? !!game.freeBuild : freeBuild);
      if (g.ruleset !== rules?.version) setRules(await sandboxApi.ruleset(g.ruleset));
      remember(g.id);
      setGame(g);
      if (previous) sandboxApi.remove(previous).catch(() => undefined);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    }
  }

  if (error) {
    return (
      <div className="notice">
        <strong>Sandbox unavailable.</strong> {error}. Start the core with <code>make serve</code>.
      </div>
    );
  }
  if (loading || !rules) return <p className="lead">Loading…</p>;
  if (!game) {
    return (
      <div className="card sb-start">
        <h3>Start from nothing</h3>
        <p>
          Your production is empty: no traffic, no servers, and {formatMoney(rules.startingCash)} in the bank. Place traffic for the users
          you want to reach and the components that serve them, wire them up, and keep the system healthy and profitable as traffic
          grows, surges, and breaks things.
        </p>
        <label className="sb-free">
          <input type="checkbox" checked={freeBuild} onChange={(e) => setFreeBuild(e.target.checked)} /> Free build: every component
          unlocked from the start (goals are still tracked)
        </label>
        <button onClick={newGame}>New game</button>
      </div>
    );
  }
  return (
    <ReactFlowProvider key={game.id}>
      <Board rules={rules} initial={game} onNewGame={newGame} />
    </ReactFlowProvider>
  );
}
