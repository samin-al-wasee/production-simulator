import { LearningPath } from "@/components/LearningPath";
import { Notice } from "@/components/Notice";
import { load, type LearningStatus } from "@/lib/api";

export default async function LearningPage() {
  const status = await load<LearningStatus>("/learning");

  return (
    <>
      <h1>Learning path</h1>
      <p className="lead">
        Ten stages from a local Docker stack to a full production simulation. Exercises tied to an experiment or a
        benchmark complete automatically when it passes; the rest you mark yourself.
      </p>
      {status.ok ? <LearningPath initial={status.data} /> : <Notice title="Learning path unavailable" detail={status.error} />}
    </>
  );
}
