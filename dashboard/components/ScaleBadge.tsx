import { load, type ClusterView } from "@/lib/api";

// The active scale factor must always be visible (Dual Metrics Mode).
export async function ScaleBadge() {
  const res = await load<ClusterView>("/cluster");
  if (!res.ok) {
    return (
      <span className="badge unavailable" title={res.error}>
        scale unavailable
      </span>
    );
  }
  return (
    <span className="badge" title="Virtual values are simulated capacity, not hardware measurements">
      simulation scale {res.data.metrics.scaleFactor}
    </span>
  );
}
