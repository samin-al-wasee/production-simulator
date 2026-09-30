import { redirect } from "next/navigation";

// The Production Sandbox is the dashboard's home (ADR-0014).
export default function Home() {
  redirect("/sandbox");
}
