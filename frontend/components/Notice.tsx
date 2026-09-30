export function Notice({ title, detail }: { title: string; detail: string }) {
  return (
    <div className="notice" role="alert">
      <strong>{title}</strong>
      <div>{detail}</div>
      <div>
        Start the API with <code>make serve</code> (or <code>forgelab serve</code>) and reload.
      </div>
    </div>
  );
}
