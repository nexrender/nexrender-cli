---
name: nexrender
description: Operate Nexrender Cloud from the terminal. Use for submitting, inspecting, waiting for, cancelling, or diagnosing render jobs; uploading and inspecting templates; managing batches, fonts, or secrets; and building Nexrender job payloads.
---

# Nexrender Cloud CLI

Use the `nexrender` CLI for Nexrender Cloud operations. Prefer its structured output and composed workflows over raw HTTP calls.

## Before API work

1. Run `nexrender auth status --json`.
2. If authentication is missing or invalid, ask the user to run `nexrender auth login`. Never request or print their API token.
3. Use `--profile <name>` when the user identifies a non-default team profile.

## Agent output

- Use `--json` for full structured results.
- Use `--jq '<expression>'` for a small result. It implies JSON.
- Run `nexrender commands --json` to discover commands.
- Run `nexrender schema <operation-id> --json` when an exact API request shape is needed.
- Treat stable error codes and exit statuses as the source of truth. Do not scrape human tables.

## Authorization boundaries

Read-only inspection and diagnosis are safe when relevant to the request. Create, cancel, delete, upload, rename, or secret-writing commands require clear user authorization. Do not infer permission to resubmit a failed job.

Use `job submit --dry-run` to validate a payload before a paid render. Never retry a mutation after an ambiguous network failure. Inspect the resulting state first.

## Core workflows

- Submit a job: `nexrender job submit --file job.json --json`
- Validate only: `nexrender job submit --file job.json --dry-run --json`
- Follow a job: `nexrender job wait <id> --json`
- Diagnose a failure: `nexrender job diagnose <id> --json`
- Upload a template: `nexrender template upload project.aep --name "Project" --json`
- Inspect exact composition and layer names: `nexrender template compositions <id> --json` and `nexrender template layers <id> --json`

Read [references/operations.md](references/operations.md) for payload and diagnosis rules.
