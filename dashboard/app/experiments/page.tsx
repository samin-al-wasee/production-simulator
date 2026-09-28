import { ExperimentCard } from "@/components/ExperimentCard";
import { Notice } from "@/components/Notice";
import { load, type AppConfig, type ExperimentInfo } from "@/lib/api";

export default async function ExperimentsPage() {
  const [experiments, config] = await Promise.all([load<ExperimentInfo[]>("/experiments"), load<AppConfig>("/config")]);

  return (
    <>
      <h1>Experiments</h1>
      <p className="lead">
        Declared chaos experiments. Each states a hypothesis, checks steady state, injects one fault, always reverts, and
        verifies recovery. They act only on <code>forgelab-*</code> containers.
      </p>
      {!experiments.ok ? (
        <Notice title="Experiments unavailable" detail={experiments.error} />
      ) : experiments.data.length === 0 ? (
        <p className="lead">No experiments found under scenarios/.</p>
      ) : (
        experiments.data.map((e) => (
          <ExperimentCard key={e.name} experiment={e} runsEnabled={config.ok && config.data.runsEnabled} />
        ))
      )}
    </>
  );
}
