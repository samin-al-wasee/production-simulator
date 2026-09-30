import { LearningPath } from "@/components/LearningPath";
import { Notice } from "@/components/Notice";
import { load, type LearningStatus } from "@/lib/api";

export default async function LearningPage() {
  const status = await load<LearningStatus>("/learning");

  return (
    <>
      <h1>Learning path</h1>
      <p className="lead">
        Six stages played in the Production Sandbox and the pipeline simulator, from a first working system to growth
        at scale. Mark each exercise done when you have played it.
      </p>
      {status.ok ? <LearningPath initial={status.data} /> : <Notice title="Learning path unavailable" detail={status.error} />}
    </>
  );
}
