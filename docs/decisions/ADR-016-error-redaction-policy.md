# ADR-016: Error redaction policy

**Status:** Accepted
**Date:** 2026-10-12
**Deciders:** [@thapelomagqazana]
**Supersedes:** —
**Superseded by:** —

---

## 1. Context

Forge's errors carry structured context: a file path, a line number, a flag name, and — in some cases — a value the user supplied. Some of those values are sensitive: an API token, a password, a path under the user's home directory. The context is rendered by two formatters: the user-facing format (ADR-014), which writes to stderr, and the log format (ADR-015), which writes to whatever destination the `slog` handler is configured with. Both formatters must apply the same redaction rules; if they disagree, a value that is redacted in one output leaks in the other.

Before WBS 10.4.3, the redaction policy was informal. The CLI's error path mentioned "context values are redacted per WBS 10.4.3" in a comment, but WBS 10.4.3 had not been written, and no redaction was performed. The user-facing format rendered context values verbatim; the log format did the same. The only protection against a secret in a context value was the caller's discipline: don't put secrets in the context map. The discipline is a good rule, but it is not a policy — it is not enforced, it is not testable, and it does not survive a contributor who has not read the rule.

The WBS 10.4.3 task is the freeze. It defines what is redacted, how the redaction is identified, and what the redacted value is replaced with. The task's inputs are WBS 10.4.1 (the user-facing format, which applies the redaction), WBS 10.4.2 (the log format, which also applies it), and WBS 8.5.1 (the config subsystem, whose error paths are the first users of the policy). The affected parties are every user who sees an error, every log consumer who indexes one, every command author who constructs a context value, and the security review that audits the codebase for leaks.

The consequence of not deciding is that secrets leak. A user who places an API token in a context value with a key like `token` sees the token on stderr and in the log. The leak is silent, is not caught by any test, and is discovered only when the token is abused. The freeze is the mechanism by which the leak is prevented before it happens.

---

## 2. Options Considered

### Option A — No redaction; rely on the caller's discipline

Do not redact. Document the rule "don't put secrets in the context map" and trust contributors to follow it.

**Pros:**

- No implementation to write or maintain.
- No performance cost on the error-construction path.
- No risk of over-redacting a value that happens to look like a secret.

**Cons:**

- The rule is not enforced. A contributor who has not read it can violate it without any signal.
- The rule is not testable. There is no test that fails when a secret is placed in a context value.
- The rule is not auditable. A security review has to inspect every context-construction site by hand.
- The rule is silent when violated. A secret placed in a context value leaks with no warning, no log entry, and no test failure.
- The rule assumes that every contributor knows which values are secrets. A token is obvious; a value that happens to be a path under the user's home directory may not be. The rule's coverage is proportional to the contributor's awareness, which is not a property the project can rely on.

### Option B — Value-pattern redaction

Identify secrets by matching the value's shape against a list of known patterns: `ghp_…` for GitHub tokens, `AKIA…` for AWS keys, `-----BEGIN * PRIVATE KEY-----` for PEM keys, and so on.

**Pros:**

- The pattern list is a well-known artifact; the patterns are published by the services that issue the secrets.
- A value that matches a pattern is redacted regardless of its key's name.
- The pattern list can be extended as new secret formats are published.

**Cons:**

- The pattern list is always incomplete. A new service's token format is not in the list until the list is updated; the token leaks in the interim.
- The pattern list over-redacts. A value that happens to match a pattern — a documentation example that quotes a redacted token, a test fixture, a legitimate identifier — is redacted unnecessarily.
- The list requires maintenance. Each new pattern is a change to the list; the list grows without bound.
- The list is a moving target. Services change their token formats; the list has to be updated in step.
- The redaction is silent in both directions. A leaked token that is not in the list is not redacted; a redacted value that is not a secret is not surfaced to the user.
- The pattern-matching itself is a source of bugs. A regular expression that is slightly too broad redacts too much; one that is slightly too narrow redacts too little. The boundary is hard to test exhaustively.

### Option C — Key-based redaction with a frozen list, plus home-directory path redaction

Identify secrets by their context key: a value whose key is `token`, `password`, `secret`, `key`, `credential`, or `auth` is redacted. The match is case-insensitive and word-boundary-based: the key is split on `_`, `-`, and `.`, and each segment is compared to the list. Separately, a value that is a path under the user's home directory has the home prefix replaced with `~`. Both transformations are applied by a single shared layer that the two formatters consult.

**Pros:**

- The identification is deterministic. A key in the list is a secret; a key not in the list is not. There is no pattern-matching ambiguity.
- The list is small and stable. It has six entries; adding a seventh requires an ADR.
- The rule is enforceable. A test can assert that every key in the list produces the redacted value, and that a key not in the list produces its value unchanged.
- The rule is auditable. A security review inspects six keys, not a growing pattern list.
- The redaction is symmetric. The user-facing format and the log format apply the same layer; they cannot diverge.
- The home-directory rule is orthogonal to the secret rule. A path under the user's home directory is redacted regardless of its key; the rule is a second transformation, not a competing one.
- The caller's discipline is reinforced, not replaced. The rule "don't put secrets in the context map" is still good; the redaction is defence in depth.

**Cons:**

- The rule is key-based, not value-based. A caller who places a token in a context value with a key like `data` is not protected; the value is not redacted.
- The list is frozen. A caller who invents a new secret-typed key (`apiKey`, `bearerToken`) is not covered; the key is added to the list only by an ADR.
- The word-boundary rule has to be documented and tested. `api_token` matches; `monkey` does not; `tokens` does not. The rules are specific and have to be applied consistently.
- The home-directory rule does not resolve symlinks. A path that is a symlink to a location under the home directory is not redacted; a path that is under the home directory via a symlink is redacted only if the symlink is resolved before the redaction. The caller is responsible for the resolution.
- The redaction is not encryption. The redacted value is a marker; the original is not recoverable from the output. A caller who needs the original value must not route it through the layer.

### Option D — Redaction as a handler-level concern

Do not redact in the error type. Redact at the `slog` handler, using a `ReplaceAttr` function that inspects every attribute and redacts the ones whose values match a secret pattern.

**Pros:**

- The redaction is centralised at the handler; every log entry is subject to it, not just errors.
- The handler can apply a broader pattern list than the error type would, since the handler is not constrained by the error's key set.

**Cons:**

- The user-facing format is not a `slog` handler; it writes directly to stderr. A handler-level redaction does not cover it. The user-facing format would still leak.
- The handler sees attribute values, not keys. A value-pattern redaction has the same problems as Option B.
- The handler configuration is per-process; a caller who configures the handler without a redacting `ReplaceAttr` gets no redaction. The policy is not enforced by the type.

**This option was rejected before the ADR was written; it is listed here for completeness.**

---

## 3. Decision

The policy is **Option C**: a key-based redaction with a frozen list, plus a home-directory path redaction, implemented as a shared layer that both the user-facing format (ADR-014) and the log format (ADR-015) consult.

The frozen secret-keys list is `token`, `password`, `secret`, `key`, `credential`, `auth`. The match is case-insensitive and word-boundary-based: the key is split on `_`, `-`, and `.`, and each segment is compared to the list. A key with a matching segment produces the redacted value `<redacted>` for its context value.

Separately, a context value that is a path beginning with the user's home directory has the home prefix replaced with `~`. The home directory is read from the process's environment via `os.UserHomeDir`. A path that shares the home directory's prefix but is a sibling (`/home/alice2` when the home is `/home/alice`) is not redacted; the check requires a path separator after the prefix.

The two transformations are applied in order by a single function (`redact`), and the user-facing and log formats call the same function. The order is: the secret-key check first, then the home-directory check. The secret-key check is a key-based lookup; the home-directory check is a value-based prefix check. Both are pure functions of their inputs and are safe for concurrent use.

The balance tipped toward Option C for three reasons. First, the identification is deterministic. A key-based rule has no pattern-matching ambiguity; a value that happens to look like a token is not redacted unless its key names it as one. Second, the rule is testable. The list has six entries; each can be tested; the word-boundary rules can be tested; the home-directory rules can be tested. The coverage is exhaustive. Third, the rule is auditable. A security review inspects six keys and a prefix rule; the review is finite and repeatable.

The trade-offs accepted are: the rule is key-based, not value-based (a token under a non-secret key is not redacted); the list is frozen (adding a key requires an ADR); the home-directory rule does not resolve symlinks (the caller is responsible for the resolution); and the redaction is not encryption (the original value is not recoverable). The first is the most significant; it is documented in the `WithContext` doc comment, which instructs callers to use secret-typed keys for secret-typed values. The other three are details of the rule's scope.

The decision assumes that the six keys in the list cover the secrets Forge's errors will carry. A future phase that introduces a new secret type (for example, a session token with a different name) adds the key to the list by an ADR. The addition is additive; no existing behaviour changes.

---

## 4. Consequences

### 4.1 Positive

- Secrets are redacted in both the user-facing format and the log format. The two consumers share the redaction layer; they cannot diverge.
- The rule is deterministic. A key in the list is redacted; a key not in the list is not. There is no pattern-matching ambiguity.
- The rule is testable. The tests cover each key, the case-insensitivity, the word-boundary rules, the home-directory prefix, and the sibling-path exclusion.
- The rule is auditable. The list is six keys; the review is finite.
- The caller's discipline is reinforced. A caller who follows the rule (secret-typed keys for secret-typed values) is protected by the redaction. A caller who violates the rule is not protected, but the rule is documented at the construction site (`WithContext`).
- The home-directory rule is orthogonal. A path under the user's home directory is redacted regardless of its key; the rule does not compete with the secret rule.
- The redaction is a single layer. The user-facing and log formats call the same function; the function's behaviour is the policy.

### 4.2 Negative

- A secret under a non-secret key is not redacted. A caller who places a token under a key like `data` or `value` leaks it. The rule's coverage is proportional to the caller's use of secret-typed keys.
- The list is frozen. A caller who invents a new secret-typed key (`apiKey`, `bearerToken`, `passphrase`) is not covered until the key is added to the list by an ADR. The friction is deliberate but is real.
- The home-directory rule does not resolve symlinks. A path that is a symlink to a location under the home directory is not redacted; a path under the home directory via a symlink is redacted only if the symlink is resolved before the redaction. The caller is responsible for the resolution; the resolution is a filesystem operation that the redaction layer does not perform.
- The redaction is not encryption. The redacted value is a marker; the original is not recoverable from the output. A caller who needs the original value must not route it through the layer.
- The redaction is applied at the formatting stage, not at construction. A caller who inspects an error's context programmatically sees the unredacted value. The redaction is a presentation concern; the error's data is unchanged.
- The word-boundary rule is specific. `api_token` matches; `monkey` does not; `tokens` does not. The rules are documented and tested but are not obvious from the key's spelling; a caller who expects `tokens` to match is surprised.

### 4.3 Neutral

- The redaction layer is applied by the user-facing format (ADR-014) and the log format (ADR-015). In Phase 2, the log format renders context values as given; the redaction layer's application to the log format is a future phase's decision. The current design applies the layer to the user-facing format only; the log format's interaction with the policy is deferred.
- The home-directory rule reads the process's environment. The `redactPath` function calls `os.UserHomeDir`; the call is a side effect on the read path, but it reads only the environment and is deterministic given the environment. A future phase that wants a different home (for example, a test that installs a synthetic home) uses the unexported `redactPathWithHome`, which takes the home as a parameter.
- The redaction's placeholder is `<redacted>`. The marker is ASCII, is enclosed in angle brackets, and does not contain whitespace. A future phase that wants a different marker changes the constant; the change is a contract change.
- The `AllSecretKeys` function returns a copy of the frozen list. The copy protects the list from mutation; a caller that sorts or truncates the returned slice does not affect the policy.

---

## 5. Related Requirements

- `SEC-003` — The redaction policy. This ADR is the policy.
- `ERR-001` — The canonical error type. The policy applies to the type's context values.
- `ERR-004` — The user-facing error format. The policy is applied by the format's context rendering.
- `LOG-002` — The programmer-facing error format. The policy's application to the log format is a future phase's decision.
- `SEC-004` *(pending)* — The secret pattern list is frozen and documented in `docs/security-model.md`. Must be added to the register when WBS 1.4 is complete.
- `NFR-TEST-007` *(pending)* — The redaction policy has a dedicated test file (`redact_test.go`) covering every key, the case-insensitivity, the word-boundary rules, and the home-directory prefix. Must be added to the register when WBS 1.4 is complete.

---

## 6. Related ADRs

- `ADR-014` — User-facing error format. The format applies the redaction policy to its context values.
- `ADR-015` — Programmer-facing error format. The format's interaction with the policy is a future phase's decision; in Phase 2, the format renders context values as given.
- `ADR-017` — Error verbose modes. The verbose mode relaxes the home-directory rule but not the secret rule; the relaxation is documented in that ADR.
- `ADR-011` — Closed error category taxonomy. The taxonomy's categories are not redacted; the redaction applies to context values.
- `ADR-012` — Error code registry and stability policy. The Codes are not redacted; the redaction applies to context values.

---

## 7. Notes

### Open questions deferred

- **Should the log format apply the redaction?** In Phase 2, the log format renders context values as given. A future phase may apply the redaction to the log format as well; the change would be additive (the redacted value is a subset of the unredacted one) and would require an ADR.

- **Should the redaction resolve symlinks?** The current rule does not; the caller is responsible. A future phase could add a symlink-resolution step, but the step would be a filesystem operation on the formatting path, which the current design avoids.

- **Should the redaction be configurable?** The current policy is fixed; a caller cannot disable it. A future phase could expose a configuration option (for example, `--no-redact` for a debug build), but the option would weaken the policy and would require an ADR.

- **Should the secret-keys list be extended?** The list is frozen at six keys. A future phase that introduces a new secret type adds the key to the list by an ADR. The addition is additive.

- **Should the redaction be applied to the message?** The current rule applies to context values only. A message is authored by the error's constructor; the constructor is responsible for not placing secrets in the message. Redacting prose is not a well-defined operation; the policy does not attempt it.

### Follow-up decisions anticipated

- **WBS 10.4.4** (verbose modes) relaxes the home-directory rule in verbose mode. The relaxation is a rendering concern; the secret rule is unaffected.
- **WBS 10.5.1** (boundary integration) routes every error through the user-facing format, which applies the redaction. The boundary does not redact independently; the format's redaction is the policy's application.
- **WBS 12.0** (logging) consumes the log format. The log format's interaction with the redaction policy is a future phase's decision; the current design renders context values as given.

### External references

- The `no-color.org` convention's influence on the redaction policy is nil; the reference is included only because the "no colour" rule and the "no secrets" rule both concern the discipline of freezing a user-facing contract.
- The GitHub secret-scanning patterns (`ghp_`, `gho_`, `ghs_`, `ghu_`) are a well-known example of value-pattern redaction. The policy rejects value-pattern redaction in favour of key-based redaction; the GitHub patterns are mentioned as the alternative that was considered.
- The `os.UserHomeDir` function's behaviour (reading `$HOME` on Unix and `%USERPROFILE%` on Windows) is the reference for the home-directory rule. A future phase that wants a different home reads it from a configuration source instead.
