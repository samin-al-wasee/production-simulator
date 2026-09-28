import { DualMetrics } from "@/components/DualMetrics";
import { Notice } from "@/components/Notice";
import { load, type ClusterView, type StackView } from "@/lib/api";

export default async function OverviewPage() {
  const [cluster, stack] = await Promise.all([load<ClusterView>("/cluster"), load<StackView>("/stack")]);

  return (
    <>
      <h1>Overview</h1>
      <p className="lead">Physical host and simulated production, side by side.</p>

      {cluster.ok ? (
        <DualMetrics view={cluster.data} />
      ) : (
        <Notice title="Cluster metrics unavailable" detail={cluster.error} />
      )}

      <h2>Local stack</h2>
      {!stack.ok ? (
        <Notice title="Stack status unavailable" detail={stack.error} />
      ) : stack.data.error ? (
        <div className="notice">{stack.data.error}</div>
      ) : stack.data.containers.length === 0 ? (
        <p className="lead">
          No ForgeLab containers found. Start the stack with <code>make up</code>.
        </p>
      ) : (
        <div className="panel">
          <table>
            <thead>
              <tr>
                <th>Container</th>
                <th>Image</th>
                <th>State</th>
                <th>Status</th>
              </tr>
            </thead>
            <tbody>
              {stack.data.containers.map((c) => (
                <tr key={c.name}>
                  <td>{c.name}</td>
                  <td>{c.image}</td>
                  <td>
                    <span className={`status ${c.state === "running" ? "ok" : "bad"}`}>{c.state}</span>
                  </td>
                  <td>{c.status}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </>
  );
}
