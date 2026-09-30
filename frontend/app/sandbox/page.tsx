import { SandboxGame } from "@/components/SandboxGame";

export default function SandboxPage() {
  return (
    <>
      <h1>Production Sandbox</h1>
      <p className="lead">
        Build a production system from nothing and run it as a business. Every value here is modelled by the core
        engine; nothing is started on your machine.
      </p>
      <SandboxGame />
    </>
  );
}
