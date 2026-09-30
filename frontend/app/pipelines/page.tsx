import { Notice } from "@/components/Notice";
import { PipelineSimulator } from "@/components/PipelineSimulator";
import { load, type PipelineInfo } from "@/lib/api";

export default async function PipelinesPage() {
  const pipelines = await load<PipelineInfo[]>("/pipelines");

  return (
    <>
      <h1>Pipelines</h1>
      <p className="lead">
        Simulated build → test → deploy pipelines on a virtual clock. Same seed, same run. A bad release triggers the
        deploy strategy&apos;s rollback.
      </p>
      {!pipelines.ok ? (
        <Notice title="Pipelines unavailable" detail={pipelines.error} />
      ) : pipelines.data.length === 0 ? (
        <p className="lead">No pipelines found under manifests/pipelines/.</p>
      ) : (
        <PipelineSimulator pipelines={pipelines.data} />
      )}
    </>
  );
}
