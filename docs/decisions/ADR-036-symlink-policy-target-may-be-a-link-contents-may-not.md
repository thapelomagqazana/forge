# ADR-036: Symlink Policy: Target May Be a Link, Contents May Not

**Status:** Proposed
**Date:** 2026-10-14
**Deciders:** [@thapelomagqazana]
**Supersedes:** —
**Superseded by:** —

---

## 1. Context

Forge operates on user-supplied project trees. On every supported platform, a user can place symbolic links inside those trees, and the operating system treats them transparently: an `open("a/b.txt")` where `a` is a symlink to `/etc` resolves to `/etc/b.txt` without any visible signal to the caller. That transparency is the source of the problem. A name that looks local can resolve to a path outside the target directory, and the failure mode is silent: the operation succeeds, but on a file the caller did not name. CWE-59 ("Improper Link Resolution Before File Access") describes the class.

Before this decision, the module had two implicit and contradictory policies. On the read path, the pipeline resolved the parent's symlinks under the configured policy; a symlink was followed only if the policy said so, and only if the target stayed inside the root. On the write path, the pipeline did the same for the parent but did nothing for the final component: `os.WriteFile` follows a symlink at the final component, so writing to `link.txt` where `link.txt` points at `/tmp/x` silently wrote to `/tmp/x`. The inconsistency was not visible in code review because the two paths looked the same; it was visible only when a symlink happened to be at the right place in a name.

The inconsistency is not the whole problem. Users legitimately use symlinks in two distinct ways, and a policy that treats them the same is wrong in one direction or the other. A developer on macOS commonly creates `~/projects` as a symlink to `/Volumes/external/projects` and runs Forge against `~/projects`; the tool should operate on the target, not on the symlink. A project that vendors a template via a symlink to a sibling directory expects the vendored content to be readable through the link. But the same project should never write *through* a link whose target is outside the project, because the write would modify a file outside the user's declared scope. The two situations call for different answers, and the code was giving one answer to both.

The situation required a decision because three downstream WBS items — the render plan (Phase 4), the base-state hashing (Phase 14), and the scan/detection subsystem (Phase 15) — each depend on symlink behaviour being predictable. Each would have encoded its own assumptions about what a symlink meant, and the assumptions would have drifted apart. The decision had to be made once, at the abstraction layer, so that the three consumers could rely on a single answer.

Related prior work: WBS 13.2.2 (the boundary wrapper that first applied a symlink policy), WBS 13.3.1 (path normalization, which specified the layers a symlink check belongs to), WBS 13.5.1 (the error taxonomy, which the symlink failures map onto), WBS 13.5.2 (the delivery that implemented the policy this ADR formalises).

---

## 2. Options Considered

### Option A — Follow symlinks everywhere

The abstraction resolves every symlink it encounters, subject only to the root-confinement check. A symlink whose target stays inside the root is followed; one whose target escapes is refused.

**Pros:**

- Simple rule: "the resolved path must stay inside the root." A caller knows what to expect without thinking about which position the link occupies.
- Matches the behavior of most command-line tools (`cat`, `grep`, `sed`) on Unix, where symlinks are transparent.
- No special case for reads versus writes; the same code path handles both.

**Cons:**

- A write through a symlink at the final component silently modifies the target. The caller asked to write `link.txt`; the write landed on the link's target. If the caller did not know `link.txt` was a link, the file they intended to create is not the file that changed.
- A symlink chain whose intermediate components point outside the root cannot be distinguished from a chain that stays inside, if the final target happens to be inside. The resolution must walk every component to know, which is exactly what the "resolve the parent" approach does not do.
- The rule is correct only when the caller knows the whole tree. In a project directory the caller did not author, that knowledge is not available.
- The threat model — a symlink placed by an attacker who wants the operation to escape — is not addressed. The attacker places the link at the target of a write, and the write escapes. The root-confinement check is not enough because the check happens on the resolved path, and the resolved path is exactly what the attacker chose.

### Option B — Do not follow symlinks anywhere

The abstraction refuses any name whose components include a symlink. Reads and writes are treated the same; a symlink anywhere in the name is an error.

**Pros:**

- Simplest possible rule: "no symlinks, ever." No ambiguity, no special cases, no policy options.
- Trivially secure against symlink-based escapes, because the abstraction never follows a link.
- Symmetric between reads and writes, which matches the naive expectation.

**Cons:**

- The common case of `~/projects → /Volumes/external/projects` is rejected, which is a real workflow on macOS and increasingly on Linux. A user who runs `forge` against a symlinked project gets an error for no security benefit.
- Legitimate vendoring via symlinks (a template directory that links to a shared `assets/` directory) is rejected.
- The rule is not the one users expect: `cat ~/projects/file` works, and a tool that refuses the same input is surprising.
- The rule is not the one downstream consumers need: a render plan that must reproduce the symlinks it finds in a source tree cannot do so if the abstraction refuses to acknowledge them.

### Option C — Symmetric policy: follows symlinks, but only under the root

The abstraction follows symlinks whose resolved target stays inside the root, and refuses those that escape, on both reads and writes. This is Option A with the escape check made explicit; the two are often confused but differ in the case of a chain that passes through a link inside the root to a target outside it.

**Pros:**

- The escape check closes the obvious hole in Option A.
- The policy is symmetric and statable in one sentence.
- Reads through symlinks continue to work, which is the common case.

**Cons:**

- The write-through-the-final-component problem remains. A symlink whose target is inside the root is followed on a write, so `Write("link.txt")` where `link.txt` is a link to `real.txt` writes to `real.txt`, not to the file the caller named.
- An attacker who can place a symlink inside the root (which is not the same as a symlink whose target is outside the root) can redirect a write from `link.txt` to any other in-root file. The write escapes the caller's declared scope, even though it stays inside the root's scope.
- The policy cannot distinguish "read this file through the symlink" (legitimate) from "write to whatever the symlink points at" (dangerous), because it treats the two operations identically.

### Option D — Asymmetric policy: follow on read, refuse on write, and resolve the root once

The abstraction treats symlinks differently depending on where they appear and what operation is being performed. The target directory's own symlinks are resolved once at construction, so a Root on `~/projects` operates on `/Volumes/external/projects`. Inside the target, a symlink on the parent of a name is subject to the Root's policy; a symlink at the final component of a write is always refused; a symlink during a Walk is reported but not followed.

**Pros:**

- The common workflow (`~/projects` as a symlink) is supported: the Root resolves the target once, and the caller never sees the symlink.
- The dangerous case (write-through-the-final-component) is closed: the write primitive refuses a symlink at the final component, regardless of its target.
- Reads through in-root symlinks continue to work under the `FollowInRoot` policy, which is what a project that vendors via symlinks needs.
- The policy distinguishes "the target directory is a symlink" (a caller's choice about where the project lives) from "a symlink inside the target" (a property of the tree, potentially untrusted).
- The Walk behaviour matches the operating-system convention: a directory symlink is a link, not a directory, and the walker does not descend into it.

**Cons:**

- The policy is not a single sentence. It has four rules, and a reader must know all four to predict behaviour.
- The asymmetry between reads and writes requires explanation. A reviewer who expects the two to behave the same will find the difference surprising until they read the rationale.
- The root-resolution-at-construction step changes the observable value of `Root.Path()`: a caller who logged the path passed to `New` sees the resolved target, not the original string.
- A future contributor who wants to add a symlink policy must understand the four rules and how a new rule composes with them.

---

## 3. Decision

Forge adopts the asymmetric symlink policy described in Option D. The policy has four rules, all enforced by the `safefs` package:

1. The target directory may be a symlink. `safefs.New` resolves the target directory's symlinks once, at construction, and stores the resolved absolute path. A Root constructed on `~/projects` (a symlink) operates on the resolved target. `Root.Path()` returns the resolved path.
2. Symlinks inside the target are refused on the write path. A write whose parent contains a symlink is subject to the Root's policy; a write whose final component is a symlink is always refused. The refusal is enforced by opening the file with `O_NOFOLLOW` on Unix and `FILE_FLAG_OPEN_REPARSE_POINT` on Windows, and it produces an error wrapping `ErrSymlinkDenied`.
3. Symlinks during a Walk are reported but not followed. The walker delivers a directory symlink to the callback as a non-directory entry (`Type() == fs.ModeSymlink`) and does not descend into it. A symlink to a file is delivered normally.
4. Broken symlinks are reported, not errored. A symlink whose target does not exist is a valid directory entry. `ReadDir` returns it; `Walk` delivers it; `Stat` reports the OS's error when it tries to follow the link; `Lstat` reports the link's own metadata.

The decision rests on four assumptions:

1. **The four situations are genuinely distinct.** A caller who names a project through a symlink, a caller who reads through an in-root symlink, a walk that encounters a symlink, and a broken symlink are different enough that a single policy would be wrong for at least one of them. The four rules are the smallest set that covers the cases without over-reaching.
2. **The operating system provides the primitive to enforce Rule 2 atomically.** `O_NOFOLLOW` on Unix and `FILE_FLAG_OPEN_REPARSE_POINT` on Windows refuse a symlink at the final component without a separate `Lstat` check. Without the primitive, Rule 2 would be racy against a symlink swap; with it, the check and the open are a single syscall.
3. **A caller who is surprised by Rule 1 can recover.** `Root.Path()` returns the resolved target; a caller who needs the original string kept it before calling `New`. The "surprise" is a change in the value the caller can inspect, not a change in the operation's semantics.
4. **The read/write asymmetry is the correct asymmetry.** The failure modes are asymmetric: a read through a symlink reads a file the caller did not name but does not modify it; a write through a symlink modifies a file the caller did not name. The second is a data-loss risk; the first is not.

If assumption (2) fails on some future platform, Rule 2 degrades to a check-then-open sequence, and the TOCTOU window widens. The degradation is documented in the symlink policy reference, and the mitigation is to keep the write primitive on platforms that provide the primitive.

The decision is not provisional. The four rules are the policy; a change to any of them would need a new ADR superseding this one.

---

## 4. Consequences

### 4.1 Positive

- The common macOS workflow (`~/projects` as a symlink to an external volume) works without a special case at the call site.
- A write never silently modifies a file outside the caller's declared path. The write primitive refuses the final-component symlink, and the caller learns about the misconfiguration via `ErrSymlinkDenied`.
- A project that vendors via symlinks (a common pattern in Go, Node, and Python projects) reads through the links under `FollowInRoot` without the abstraction treating every vendored file as a security event.
- A Walk over a tree that contains a symlink does not descend into the link's target, which bounds the walk's cost and prevents a cycle from defeating the depth bound.
- A broken symlink does not error a Walk or a ReadDir, which matches the operating-system convention and lets a caller that cares about broken links inspect them explicitly.

### 4.2 Negative

- The policy is not a single rule. A reader must know all four rules to predict behaviour, and the four rules' interaction (for example, what happens when a Walk encounters a directory symlink whose target is inside the root) must be understood as the composition of Rule 2 and Rule 3.
- The read/write asymmetry is a legitimate source of surprise. A reviewer who expects a read and a write to behave the same will be confused by the difference until they read the rationale; the rationale belongs in the policy document, and a future contributor who proposes to make the policy symmetric must argue against the asymmetry explicitly.
- `Root.Path()` returns the resolved path, which differs from the string the caller passed to `New`. A caller who compares the two sees a difference; a caller who logs the two sees two paths for the same Root. The change is correct — the resolved path is the one the operations act on — but it is a change, and callers with code that compares paths need to be aware of it.
- A future platform that does not provide the atomic final-component check degrades Rule 2 to a check-then-open sequence, which widens the TOCTOU window. The degradation must be documented and, if possible, avoided by not targeting the platform until the primitive is available.
- The policy is asymmetric in a second sense: the Root's policy (`NoFollow` vs `FollowInRoot`) applies to the parent, but the final component of a write is always refused. A caller who wants a write to follow a symlink at the final component cannot express that intent. The refusal is deliberate — the write-through-the-final-component case is the one the policy exists to prevent — but it is a hard limit.

### 4.3 Neutral

- The policy is documented in `docs/security/symlink-policy.md`. The document is normative; the code implements it. A change to the code that violates the document is a bug, and the document is the reviewer's reference.
- The policy's four rules are named in the code's package doc and in the policy document. A reader who knows the four rules by name (Rule 1 through Rule 4) can refer to them precisely.
- The `FollowAnywhereForbidden` policy is an alias for `NoFollow`. It exists so a caller can express intent explicitly; it does not add a fifth policy.

---

## 5. Related Requirements

- `SEC-FS-017` — the symlink policy requirement; this ADR is the policy.
- `SEC-FS-018` — the target-directory-as-symlink requirement; this ADR's Rule 1.
- `SEC-FS-019` — the write-path symlink refusal requirement; this ADR's Rule 2.
- `SEC-FS-020` — the Walk symlink-reporting requirement; this ADR's Rule 3.
- `SEC-FS-021` — the broken-symlink handling requirement; this ADR's Rule 4.
- `SEC-FS-022` — the TOCTOU mitigation requirement; this ADR's Rule 2 depends on the atomic final-component check.
- `FR-FS-022` — the boundary model requirement; this ADR is one of the two policies the boundary model composes.
- `FR-FS-024` — the error taxonomy requirement; the symlink failures map to `ErrSymlinkDenied` and `ErrSymlinkLoop` in the taxonomy.

> The following requirements must be added to the register when WBS 1.4 is complete: `SEC-FS-023` (the read/write asymmetry is documented and tested), `SEC-FS-024` (a future policy change is refused at the code level, not merely by convention).

---

## 6. Related ADRs

- `ADR-035` — Two-Layer Filesystem Architecture; this ADR is the symlink policy that lives in the `safefs` layer.
- `ADR-011` — Closed Error Category Taxonomy; the symlink failures map to the security category this ADR defines.
- `ADR-013` — Error Wrapping Contract; the symlink failures preserve the underlying error (`ELOOP`, `ERROR_CANT_RESOLVE_FILENAME`) while adding the sentinel.
- `ADR-018` — Exit-Code Taxonomy; the security exit code is what a script aborts on when a symlink policy refuses an operation.

---

## 7. Notes

### Open questions

- **Should the policy be a single setting?** The current design splits the policy into the Root's `SymlinkPolicy` (which governs the parent and the final component of reads) and the write-path's fixed refusal (which governs the final component of writes). A single setting could express both, at the cost of a more complicated option type. The current design was chosen because the write-path refusal is not a policy the caller should be able to disable; making it configurable would invite a caller to re-enable the very escape the policy exists to prevent. Deferred; tracked as a note on WBS 13.5.2.
- **Should the walk follow symlinks under `FollowInRoot`?** The current rule (Rule 3) refuses to descend regardless of policy. A future variant could descend under `FollowInRoot` if the resolved target stays inside the root, at the cost of making the walk's cost dependent on the target's structure and of reintroducing the graph-vs-tree problem the current rule avoids. Deferred; tracked as a note on WBS 13.5.2.
- **Should `Root.Path()` return the unresolved path?** The current behaviour returns the resolved path. A caller who wants the original string kept it. A future ADR could add a `Root.OriginalPath()` accessor if the demand arises. Deferred; tracked as a note on WBS 13.5.2.

### Follow-up decisions anticipated

- **ADR-NNN — Symlink Policy Closure.** The decision to close the set of symlink policies (`NoFollow`, `FollowInRoot`, and the alias `FollowAnywhereForbidden`) is a decision of its own; the current document states the closure but does not ratify it. The follow-up ADR would ratify the closure and state the change process for adding a policy.
- **ADR-NNN — TOCTOU Posture.** The TOCTOU posture that this ADR's Rule 2 depends on is documented in `docs/reference/path-normalization.md`; the ADR would ratify the posture and the "best-effort" guarantee.
- **ADR-NNN — Atomic Write Semantics.** The atomic write primitive that Rule 2's `O_NOFOLLOW` interacts with is the subject of a separate ADR (see WBS 13.5.2).

### External references

- CWE-59 — Improper Link Resolution Before File Access
- CWE-61 — UNIX Symbolic Link (Symlink) Following
- CWE-367 — Time-of-Check Time-of-Use (TOCTOU) Race Condition
- POSIX `open(2)` with `O_NOFOLLOW`
- Windows `CreateFile` with `FILE_FLAG_OPEN_REPARSE_POINT`
- `docs/security/symlink-policy.md` — the policy document this ADR ratifies
- `internal/filesystem/safefs/symlink.go` — the implementation of the four rules
- `internal/filesystem/safefs/atomic.go` — the atomic write primitive that Rule 2 protects
