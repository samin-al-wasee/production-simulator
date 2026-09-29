"use client";

import { Handle, Position, type Node, type NodeProps } from "@xyflow/react";
import { formatCompact, level, type NodeStats } from "@/lib/sandbox";

export type SandboxNodeData = {
  label: string;
  kind: string;
  size: string;
  replicas: number;
  down: boolean;
  source: boolean;
  target: boolean;
  stats?: NodeStats;
};

export type SandboxFlowNode = Node<SandboxNodeData, "component">;

export function SandboxNode({ id, data, selected }: NodeProps<SandboxFlowNode>) {
  const s = data.stats;
  const internet = data.kind === "internet";
  const util = s?.utilization ?? 0;
  const cls = data.down ? "bad" : level(util);
  return (
    <div className={`sb-node lvl-${cls}${selected ? " selected" : ""}${internet ? " internet" : ""}`}>
      {data.target && <Handle type="target" position={Position.Left} />}
      <div className="sb-node-title">{data.label}</div>
      <div className="sb-node-meta">
        {internet ? "traffic source" : `${id} · ${data.size}${data.replicas > 1 ? ` ×${data.replicas}` : ""}`}
      </div>
      {s && (
        <>
          {!internet && (
            <div className="sb-util" title={`utilization ${(util * 100).toFixed(0)}%`}>
              <span className={cls} style={{ width: `${Math.min(util, 1) * 100}%` }} />
            </div>
          )}
          <div className="sb-node-meta">
            {formatCompact(s.served)}/s
            {!internet && ` · ${s.latencyMs.toFixed(0)} ms`}
            {s.dropped > 0.01 && <span className="bad"> · {formatCompact(s.dropped)}/s dropped</span>}
            {!!s.backlog && ` · ${formatCompact(s.backlog)} queued`}
          </div>
        </>
      )}
      {data.down && <div className="sb-node-meta bad">DOWN</div>}
      {data.source && <Handle type="source" position={Position.Right} />}
    </div>
  );
}
