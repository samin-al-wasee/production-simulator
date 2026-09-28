import type { ClusterView, Resources } from "@/lib/api";
import { formatBytes, formatCores, formatCount, formatPercent } from "@/lib/format";

function ratio(used: number, total: number): string {
  return total > 0 ? formatPercent(used / total) : "n/a";
}

function Rows({ used, total }: { used: Resources; total: Resources }) {
  return (
    <table>
      <thead>
        <tr>
          <th>Resource</th>
          <th className="num">Used</th>
          <th className="num">Capacity</th>
          <th className="num">Utilization</th>
        </tr>
      </thead>
      <tbody>
        <tr>
          <td>CPU (cores)</td>
          <td className="num">{formatCores(used.cpuCores)}</td>
          <td className="num">{formatCores(total.cpuCores)}</td>
          <td className="num">{ratio(used.cpuCores, total.cpuCores)}</td>
        </tr>
        <tr>
          <td>Memory</td>
          <td className="num">{formatBytes(used.memoryBytes)}</td>
          <td className="num">{formatBytes(total.memoryBytes)}</td>
          <td className="num">{ratio(used.memoryBytes, total.memoryBytes)}</td>
        </tr>
        <tr>
          <td>Disk</td>
          <td className="num">{formatBytes(used.diskBytes)}</td>
          <td className="num">{formatBytes(total.diskBytes)}</td>
          <td className="num">{ratio(used.diskBytes, total.diskBytes)}</td>
        </tr>
      </tbody>
    </table>
  );
}

// Dual Metrics Mode: physical host measurements and simulated virtual values
// side by side, visually distinct, with the scale factor always shown.
export function DualMetrics({ view }: { view: ClusterView }) {
  const { metrics, factor, cluster } = view;
  return (
    <>
      <p className="lead">
        Cluster <strong>{cluster}</strong> at scale <strong>{metrics.scaleFactor}</strong> (binding resource:{" "}
        {factor.binding}).
      </p>
      <div className="grid2">
        <section className="panel physical" aria-label="Physical host metrics">
          <h3>Physical host</h3>
          <div className="tag">Real measurements from this machine ({metrics.physical.kind})</div>
          <Rows used={metrics.physical.used} total={metrics.physical.budget} />
          <p className="legend">Capacity is the allocatable budget after the host reserve.</p>
        </section>
        <section className="panel virtual" aria-label="Virtual production metrics">
          <h3>Virtual production · {metrics.scaleFactor}</h3>
          <div className="tag">
            Simulated capacity, not hardware ({metrics.virtual.kind}) · {formatCount(metrics.virtual.nodes)} virtual nodes
          </div>
          <Rows used={metrics.virtual.used} total={metrics.virtual.capacity} />
          <p className="legend">Each resource keeps its physical utilization: virtual usage applies the matching virtual/physical ratio.</p>
        </section>
      </div>
    </>
  );
}
