---
name: nexrender
description: Operate Nexrender Cloud from the terminal. Use for submitting, inspecting, waiting for, cancelling, or diagnosing render jobs; uploading and inspecting templates; managing batches, fonts, or secrets; exporting Lottie animations; building Nexrender job payloads; and calling the Nexrender API directly when the CLI lacks something.
---

# Nexrender Cloud CLI

Use the `nexrender` CLI for Nexrender Cloud operations. Prefer its structured output and composed workflows over raw HTTP calls. The CLI is a thin wrapper over the REST API: when it cannot do what is needed, read [references/api.md](references/api.md) to call the API directly.

## Before API work

1. Run `nexrender auth status --json`.
2. If authentication is missing or invalid, ask the user to run `nexrender auth login`. Never request or print their API token.
3. Use `--profile <name>` when the user identifies a non-default team profile.
4. Run `nexrender team show --json` when it matters which team the token acts on, or how much of a trial is left.

## Agent output

- Use `--json` for full structured results.
- Use `--jq '<expression>'` for a small result. It implies JSON.
- Run `nexrender commands --json` to discover commands.
- Run `nexrender schema <operation-id> --json` when an exact API request shape is needed.
- Treat stable error codes and exit statuses as the source of truth. Do not scrape human tables.

## Authorization boundaries

Read-only inspection and diagnosis are safe when relevant to the request. Create, cancel, delete, upload, rename, or secret-writing commands require clear user authorization. Do not infer permission to resubmit a failed job.

Use `job submit --dry-run` to check a payload before a paid render. It only validates locally: the API runs its own checks on submit and can still refuse a payload that passed. Never retry a mutation after an ambiguous network failure. Inspect the resulting state first.

## Core workflows

- Submit a job: `nexrender job submit --file job.json --json`
- Validate only: `nexrender job submit --file job.json --dry-run --json`
- Follow a job: `nexrender job wait <id> --json`
- Diagnose a failure: `nexrender job diagnose <id> --json`
- Upload a template: `nexrender template upload project.aep --name "Project" --json`
- Inspect exact composition and layer names: `nexrender template compositions <id> --json` and `nexrender template layers <id> --json`
- Export a Lottie animation: submit a job with `settings.type: "lottie"` and Bodymovin's settings in `settings.lottie`. See [references/api.md](references/api.md#lottie-export).

Read [references/operations.md](references/operations.md) for payload and diagnosis rules, and [references/api.md](references/api.md) for base URLs, authentication, endpoints and Lottie jobs.
