# ADR-037: TOCTOU Is Narrowed, Not Closed

**Status:** Proposed
**Date:** 2026-10-14
**Deciders:** [@thapelomagqazana]
**Supersedes:** —
**Superseded by:** —

---

## 1. Context

Forge validates a path before it uses it. It cleans the name, checks that the cleaned form is local, checks that the name is not on the deny-list, resolves the parent's symlinks under the configured policy, and re-checks that the resolved parent is inside the target directory. Only then does the operation call the underlying filesystem. Between the last check and the syscall, an attacker who can modify the filesystem can change the meaning of the name: replace a directory with a symlink, swap a file for another, create a path that did not exist during the check. The window is small, but it is real, and it is not a hypothetical: CWE-367 ("Time-of-Check to Time-of-Use Race Condition") describes exactly this class, and it appears in every program that validates a path and then opens it.

The problem has no perfect solution on the platforms Forge supports. Linux provides `openat2(2)` with `RESOLVE_BENEATH`, which resolves a path relative to a directory descriptor and refuses any component that escapes; the resolve and the open are one syscall, and the window is closed. macOS does not provide an equivalent. Windows provides `FILE_FLAG_OPEN_REPARSE_POINT` for a single component but not for a whole path. A cross-platform abstraction cannot rely on a primitive that exists on only one of its three targets, and Forge targets Linux, macOS, and Windows.

Forge needs a stated posture, not a promise it cannot keep. If the abstraction claims "no TOCTOU is possible," a reviewer will trust it, and the code will be wrong on macOS and Windows. If the abstraction claims nothing, a caller will assume the worst and either duplicate checks the abstraction already performs or invent coordination the abstraction does not need. The honest answer is between the two: state what the abstraction narrows, state what it does not close, and name the residual risk so a caller who needs a stronger guarantee knows where to look.

The decision had to be made before Phase 4 (render plan), Phase 14 (base-state hashing), and Phase 15 (scan/detection) built on the abstraction, because each of those phases has a different tolerance for TOCTOU. The render plan writes into a directory the user owns; it does not need a stronger guarantee than "best effort." The base-state hasher reads a tree and compares snapshots; a race that changes a file between the two reads produces a spurious diff, which the user can rerun. The scanner reads a tree and may need to promise that a file it reported has not changed under it; whether that promise is needed is a Phase 15 concern. Stating the abstraction's posture up front lets each phase decide its own additional guarantee.

Related prior work: WBS 13.2.2 (the pipeline that does the checks), WBS 13.3.1 (the path normalization document that first named the TOCTOU posture), WBS 13.5.2 (the symlink policy that depends on the same narrowed-window discipline).

---

## 2. Options Considered

### Option A — Close the window with a platform-specific primitive

Use `openat2(2)` with `RESOLVE_BENEATH` on Linux, and the closest equivalent on each other platform, to make the check and the open a single atomic operation. Document a strict "no TOCTOU" guarantee.

**Pros:**

- The guarantee is strong. A caller can rely on it without qualification.
- The Linux implementation is straightforward: `openat2` is a single syscall, and the Go `syscall` package or `golang.org/x/sys/unix` provides the wrapper.
- The abstraction is a strict improvement over what it does today, at least on Linux.

**Cons:**

- macOS does not provide `openat2` or any equivalent. The abstraction would have to fall back to a check-then-open sequence on macOS, and the guarantee would be "strong on Linux, best-effort on macOS," which is not the guarantee the ADR claims.
- Windows's `FILE_FLAG_OPEN_REPARSE_POINT` is per-component, not per-path. Making the abstraction use it for every component is possible but expensive: each component requires its own `CreateFile` and `Close`, and the whole sequence is not atomic even then.
- The Linux-only implementation would tempt a future contributor to conditionalise the abstraction on `runtime.GOOS`, which the module's design avoids in favour of build tags and interface splits.
- The guarantee is stronger than any of the three consumers need. The render plan, the hasher, and the scanner all operate on directories the user owns; they do not face a concurrent attacker. The extra complexity buys a guarantee that no current use case requires.

### Option B — Document the posture as "best effort" and provide no additional guarantee

State in the abstraction's package doc and in the path-normalization reference that TOCTOU is possible, that the window is narrowed to the gap between the parent resolution and the syscall, and that callers who need a stronger guarantee must provide their own coordination. Provide no mechanism.

**Pros:**

- Honest. The abstraction does not claim a guarantee it cannot deliver.
- Simple. No platform-specific code, no conditional implementations, no per-platform tests.
- The docs and the code agree: both say "best effort."
- The choice of additional coordination is left to the caller, who has the context (the phase, the workload, the threat model) the abstraction does not.

**Cons:**

- A caller who needs a stronger guarantee has to build it themselves, and the abstraction offers no help. The caller might not know that `openat2` exists or how to use it relative to a directory descriptor.
- The docs do not say what "best effort" means quantitatively. A caller cannot tell whether the window is one syscall wide or much wider, so the phrase is not actionable.
- A reviewer who is satisfied by "best effort" might not think about the case at all, and the abstraction provides no signal that a stronger guarantee is possible.

### Option C — Narrow the window, document the residual risk, and provide a mechanism for callers who need more

Apply the mitigations that the abstraction can apply on every platform — resolve the parent immediately before the syscall, use `O_NOFOLLOW` and `FILE_FLAG_OPEN_REPARSE_POINT` for the final component, order the checks so a rejection never triggers a syscall — and document the residual risk precisely: the window is one syscall wide, and closing it would require a platform primitive that not every target provides. Provide a documented path for callers who need more: open the parent directory, obtain a descriptor, and operate relative to the descriptor, which the abstraction does not do for them but which it does not prevent.

**Pros:**

- The abstraction does what it can on every platform, without a conditional implementation.
- The documentation is honest: it says what the window is, why it exists, and what would close it.
- A caller who needs a stronger guarantee has a documented path: the abstraction's boundary check is not the last word, and a caller can layer a stricter guarantee on top.
- The window is narrow enough that the residual risk is negligible for the consumers Forge has. One syscall is a window that requires the attacker to have already achieved filesystem access; the abstraction's job is to refuse the attack, not to assume the attacker cannot.

**Cons:**

- The abstraction still does not close the window. A caller who wants a strong guarantee has to build it themselves.
- The documentation must be precise enough to be useful. A vague "one syscall wide" is not a number; a reader needs to know which syscall and why the window cannot be narrowed further.
- The mitigations require the abstraction to know about platform-specific primitives (`O_NOFOLLOW`, `FILE_FLAG_OPEN_REPARSE_POINT`), which is the layer's job, but it means the platform-specific behaviour is spread across the pipeline, the symlink resolver, and the atomic-write primitive.

### Option D — Adopt a coordinate system that eliminates the window by construction

Require all operations to go through a file descriptor for the parent directory, obtained once at Root construction and held for the Root's lifetime. Every operation uses `openat(dirfd, name)` or the platform equivalent. The window is eliminated because the dirfd is fixed and the operations are relative to it.

**Pros:**

- The window is genuinely closed on every platform that provides `openat` or an equivalent. Linux and macOS both do. Windows has `NtCreateFile` with a `RootDirectory` parameter, which is the equivalent.
- The abstraction's API is unchanged: callers still pass names, and the abstraction resolves them relative to the dirfd.
- The guarantee is strong and portable, if the platform-specific code is written carefully.

**Cons:**

- The Root holds a file descriptor for its lifetime, which is a resource that must be closed and a resource that the abstraction has so far not held. The `Root` type's doc comment says "holds no OS resources"; this option would require changing that.
- The abstraction would have to hold one dirfd per Root and, for sub-roots, a dirfd per Handle. Sub-roots are currently cheap (a pointer and a string); they would become resources with lifetimes.
- A Root constructed on a directory that does not exist yet cannot hold a dirfd for it. The current abstraction supports "construct before create," and this option would remove that support.
- The platform-specific code is substantial: `openat`, `fstatat`, `unlinkat`, `renameat`, `mkdirat`, and their Windows equivalents. Every operation would need its own path-relative implementation.
- The abstraction's tests would need to exercise the dirfd path on every platform, which is more work than the current tests.

---

## 3. Decision

Forge adopts Option C. The abstraction applies every mitigation that is portable across the three targets, documents the residual risk precisely, and provides a documented path for callers who need a stronger guarantee.

The mitigations the abstraction applies, in order:

1. **Resolve the parent immediately before the syscall.** The pipeline's `resolveForIO` resolves the parent and returns the resolved name; the operation uses that name immediately. The gap between the resolution and the syscall is minimised but not eliminated.
2. **Use the final-component primitive on the write path.** `O_NOFOLLOW` on Unix and `FILE_FLAG_OPEN_REPARSE_POINT` on Windows make the kernel refuse a symlink at the final component in the same syscall that opens the file. The check and the open are atomic for the final component.
3. **Order the checks so a rejection never triggers a syscall.** The lexical checks (validateName, deny-list, allow-list) run before any I/O. A name that fails them never reaches the filesystem.
4. **Re-verify the parent immediately before the write.** The `verifyWritePath` function re-checks the parent's non-symlink status just before the write primitive, narrowing the window to the gap between that check and the write's open.

The residual risk is stated precisely: the window between the pipeline's parent resolution and the operation's syscall is one syscall wide. An attacker who can modify the filesystem during that window can change the meaning of the name. The window cannot be closed on every supported platform because the primitive that closes it (`openat2` with `RESOLVE_BENEATH` on Linux) does not exist on macOS or Windows.

The documented path for callers who need more: open the parent directory, obtain a descriptor, and operate relative to the descriptor. The abstraction does not do this for the caller, but it does not prevent it. A caller who needs the guarantee can bypass the abstraction's operations for the specific case, use `os.Open` on the parent directory directly (which the layering rule permits for a package that is not a domain package), and use `unix.Openat` or the equivalent for the file operation.

The decision rests on four assumptions:

1. **The residual window is small enough that no current consumer needs it closed.** The render plan, the base-state hasher, and the scanner all operate on directories the user owns and do not face a concurrent attacker. If a future consumer needs the window closed, this ADR is the place to record the change.
2. **The platform primitives for the final-component check are available on every target.** `O_NOFOLLOW` is on POSIX; `FILE_FLAG_OPEN_REPARSE_POINT` is on Windows. Both are stable and documented.
3. **A caller who needs a stronger guarantee can build one.** The abstraction's operations are not the only way to reach the filesystem for a package that is permitted to import `os`; a caller with a strict requirement can use the primitive directly.
4. **The documentation is precise enough that a reviewer can evaluate the risk.** The window is named (one syscall), the platform limits are named (Linux has the primitive, macOS and Windows do not), and the mitigation is named (open the parent, operate relative).

If assumption (1) fails and a consumer needs the window closed, the fix is a new ADR proposing the dirfd-based coordinate system (Option D) for the specific consumer. If assumption (2) fails on a future platform, the platform-specific final-component check degrades to a check-then-open sequence, and the window widens by that difference.

The decision is not provisional. The posture is "narrowed, not closed"; a change to "closed" would be a new ADR with its own consequences.

---

## 4. Consequences

### 4.1 Positive

- The abstraction is honest. A reviewer reading the path-normalization reference sees the exact residual risk and the reason it exists. The reviewer does not have to infer the risk from the code.
- The abstraction applies every mitigation that is portable. On Linux, the final-component check is atomic; on macOS and Windows, the check-then-open window is narrowed to one syscall. The three platforms behave as closely as the primitives allow.
- A caller who needs a stronger guarantee has a documented path. The abstraction does not pretend to be the last word on security; it says what it does and what a caller can do beyond it.
- The abstraction's boundary model remains simple. No dirfd lifetimes, no per-Root resources, no "construct before create" removal. The Root is still a value; the Handle is still a pointer and a string.
- The tests remain portable. The same test suite runs on every platform; the platform-specific code is confined to the symlink resolver and the atomic-write primitive, and it is tested by build-tagged files.

### 4.2 Negative

- A caller who needs the window closed has to build the dirfd-based coordinate system themselves. The abstraction does not provide it. The caller must know `openat2` or `unix.Openat` or the Windows equivalent; the abstraction's docs name the path but do not walk it.
- The residual risk is real and cannot be argued away. On a system with a concurrent attacker who can modify the target directory, the abstraction's guarantee is "the window is narrow," not "the window is closed." A caller who reads the docs and trusts the abstraction for a security-sensitive operation is making a mistake the abstraction's docs warn about, but the warning is easy to miss.
- The mitigations are spread across several files. The pipeline's parent resolution, the symlink resolver's final-component check, the atomic-write primitive's re-verification, and the operation's use of the resolved name are all part of the posture. A reviewer who wants to audit the posture must read four files.
- The abstraction does not provide a mechanism for the caller to obtain the resolved parent's descriptor. A caller who wants to build the dirfd-based system must re-derive the parent path, which means either re-doing the pipeline's checks or trusting the abstraction's result without being able to inspect the descriptor it used.

### 4.3 Neutral

- The path-normalization reference document gains a "TOCTOU Posture" section that is normative. The section is the abstraction's guarantee; a change to the code that violates the section is a bug.
- The abstraction's four mitigations are named in the code and in the doc. A reader who knows the four mitigations by their function names (resolve the parent, `O_NOFOLLOW`, lexical-first ordering, verify-write-path) can refer to them precisely.
- The `openat2` primitive is mentioned in the docs as the mechanism that would close the window on Linux. The mention is informational; the abstraction does not use it. A future contributor who proposes to use it must open an ADR, because using it would change the abstraction's platform behaviour.

---

## 5. Related Requirements

- `SEC-FS-022` — the TOCTOU mitigation requirement; this ADR is the posture that satisfies it.
- `SEC-FS-017` — the symlink policy requirement; the final-component check this ADR names is the same primitive the symlink policy uses.
- `SEC-FS-019` — the write-path symlink refusal requirement; this ADR's mitigation (2) is the mechanism.
- `NFR-PLATFORM-001` — the cross-platform requirement; this ADR's posture is the same on every platform, modulo the primitives that differ.
- `FR-FS-023` — the path-normalization requirement; the TOCTOU posture is documented in the same reference.
- `FR-FS-024` — the error taxonomy requirement; the abstract TOCTOU-adjacent errors (`ErrSymlinkDenied`, `ErrNotFound`) are in the taxonomy.

> The following requirements must be added to the register when WBS 1.4 is complete: `SEC-FS-025` (the TOCTOU posture is documented and normative), `SEC-FS-026` (a caller who needs a stronger guarantee has a documented path), `NFR-TOCTOU-001` (the abstraction does not claim a guarantee it cannot deliver on all targets).

---

## 6. Related ADRs

- `ADR-035` — Two-Layer Filesystem Architecture; this ADR is a posture of the `safefs` layer.
- `ADR-036` — Symlink Policy: Target May Be a Link, Contents May Not; the final-component check this ADR relies on is the same primitive Rule 2 of the symlink policy uses.
- `ADR-013` — Error Wrapping Contract; the TOCTOU-adjacent failures are wrapped the same way as every other `safefs` failure.
- `ADR-018` — Exit-Code Taxonomy; a TOCTOU-triggered failure maps to the same exit codes as the underlying operation.

---

## 7. Notes

### Open questions

- **Should the abstraction offer an opt-in dirfd mode?** A future version could accept `WithDirfd()` and hold a descriptor for the Root's lifetime, offering a stronger guarantee to callers who need it. The cost is the resource lifetime and the "construct before create" removal. Deferred; tracked as a note on WBS 13.5.2.
- **Should the abstraction use `openat2` on Linux and fall back elsewhere?** The platform split would be a build-tagged file with the Linux implementation and a fallback. The gain is a strong guarantee on Linux; the cost is a conditional behaviour that the docs would have to explain. Deferred; tracked as a note on WBS 13.2.1.
- **Should the abstraction expose the resolved parent's descriptor?** A `Root.OpenParent(name)` method that returns a descriptor for the parent would let a caller build the dirfd-based system without re-doing the pipeline's checks. The cost is a new method with its own lifetime rules. Deferred; tracked as a note on WBS 13.5.2.

### Follow-up decisions anticipated

- **ADR-NNN — Symlink Policy Closure.** The closure of the symlink policy set is the subject of a separate ADR (see ADR-036's notes).
- **ADR-NNN — Atomic Write Semantics.** The temp-file-rename-fsync sequence and its durability guarantees are the subject of a separate ADR (see WBS 13.5.2).
- **ADR-NNN — Dirfd-Based Coordinate System.** If a future consumer needs the window closed, this ADR would propose the dirfd-based coordinate system and its consequences.

### External references

- CWE-367 — Time-of-Check to Time-of-Use (TOCTOU) Race Condition
- Linux `openat2(2)` with `RESOLVE_BENEATH`
- macOS `openat(2)` and the absence of a `RESOLVE_BENEATH` equivalent
- Windows `CreateFile` with `FILE_FLAG_OPEN_REPARSE_POINT`
- `docs/reference/path-normalization.md` — the TOCTOU Posture section this ADR ratifies
- `internal/filesystem/safefs/pipeline.go` — the pipeline that applies the mitigations
- `internal/filesystem/safefs/symlink.go` — the symlink resolver that uses the final-component primitive
- `internal/filesystem/safefs/atomic.go` — the atomic-write primitive that re-verifies the parent
