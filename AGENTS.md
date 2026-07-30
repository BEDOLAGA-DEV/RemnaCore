# Project-Scope Rules

These rules apply to the entire RemnaCore repository.

## Commits

- Every commit must follow the Conventional Commits specification.
- Use a semantic type such as `feat`, `fix`, `refactor`, `test`, `docs`, `build`,
  `ci`, `perf`, `style`, `chore`, or `revert`.
- Keep each commit focused on one coherent change.
- Write commit subjects in English, in the imperative mood, without a trailing
  period.

Example:

```text
feat(billing): add subscription renewal grace period
```

## Code Comments

- Add comments only where the intent, constraint, trade-off, invariant, or
  non-obvious behavior cannot be understood easily from the code itself.
- Explain why the logic exists, not what a plainly readable statement does.
- Keep comments accurate when the related code changes.
- Do not add comments that merely restate names, types, or control flow.

## Language

- Use English throughout the entire project.
- All source code, identifiers, comments, documentation, commit messages,
  configuration descriptions, user-facing copy, logs, and error messages must
  be written in English.
- Localized UI content may use other languages only through the project's
  internationalization resources.

## Tests

- All new or changed code must be covered by tests.
- Add or update unit, integration, architecture, or end-to-end tests according
  to the behavior and risk of the change.
- Bug fixes must include a regression test that fails without the fix.
- Tests must cover expected behavior, relevant edge cases, and failure paths.
- A code change is not complete while its relevant tests are missing or
  failing.
