// A second process arming documents into one session, for the
// concurrent-writer test in state.test.ts.
import { Session } from "../state";

const [base, prefix, count] = process.argv.slice(2);
const session = new Session({ base: base ?? "", root: "/repo", session: "s1" });
for (let i = 0; i < Number(count); i++) {
  session.arm({
    id: `${prefix ?? ""}-${String(i)}`,
    type: "design",
    path: `docs/${prefix ?? ""}/${String(i)}.md`,
    root: "/repo",
    config: "",
    docs_dir: "docs",
  });
}
