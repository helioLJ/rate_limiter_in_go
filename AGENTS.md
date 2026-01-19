# AGENTS.md

## Project Rules
- For each pack of changes: review the diff, run the smallest relevant validations (format/lint/tests/integration if applicable), then stage, commit with a clear message, and push to origin.
- Keep diffs minimal and scoped to the requested work; avoid refactors unless required.
- Do not add new production dependencies unless strictly necessary; justify and document any additions.
- Prefer idiomatic Go: explicit error handling, context usage, concurrency safety, and clear APIs.
- Document behavior changes and public usage in README or docs when applicable.
- If you cannot run validation, state why and provide exact commands to run.
