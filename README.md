# Nexrender CLI

Render After Effects projects, inspect what happened, and keep work moving from your terminal.

Nexrender CLI is made for people who like a good command line and coding agents that need a safe, predictable way to operate Nexrender. Upload a template, start a render, follow it to completion, or figure out why it failed.

A render can be this simple:

```bash
nexrender job submit --file launch-video.json --wait
```

And a failed render does not need to become an afternoon of tab hunting:

```bash
nexrender job diagnose JOB_ID
```

## Install

macOS, Linux, and WSL:

```bash
curl -fsSL https://raw.githubusercontent.com/nexrender/nexrender-cli/main/scripts/install.sh | sh
```

Windows PowerShell:

```powershell
irm https://raw.githubusercontent.com/nexrender/nexrender-cli/main/scripts/install.ps1 | iex
```

Or install it with Go:

```bash
go install github.com/nexrender/nexrender-cli/cmd/nexrender@latest
```

## Getting started

Run:

```bash
nexrender setup
```

Setup checks your Nexrender API key, saves it in your system keyring when possible, and teaches detected coding agents how to use the CLI. Run it again whenever you want to reconnect an account or install the agent skill somewhere new.

For CI and sandboxed agents that cannot access your system keyring, expose the API token as `NEXRENDER_API_KEY`:

```bash
export NEXRENDER_API_KEY="..."
nexrender auth status
```

Have more than one Nexrender team or server? Give each one a profile:

```bash
nexrender profile add customer-a
nexrender profile use customer-a
nexrender --profile customer-a job list
```

## Put Nexrender to work

See what is rendering:

```bash
nexrender job list
nexrender job show JOB_ID
nexrender job logs JOB_ID
```

Check a job before spending render time, then send it off:

```bash
nexrender job submit --file job.json --dry-run
nexrender job submit --file job.json --wait
```

Bring in a new After Effects template and learn what is inside it:

```bash
nexrender template upload product-video.aep --name "Product video"
nexrender template compositions TEMPLATE_ID
nexrender template layers TEMPLATE_ID
```

Download the project into the current directory, choose its filename, or ask for the temporary URL:

```bash
nexrender template download TEMPLATE_ID
nexrender template download TEMPLATE_ID --output product-video.aep
nexrender template download TEMPLATE_ID --url
```

Work with a whole batch:

```bash
nexrender batch submit --file campaign.json --dry-run
nexrender batch submit --file campaign.json
nexrender batch wait BATCH_ID
```

Fonts and encrypted secrets are along for the ride too:

```bash
nexrender font upload Inter-Bold.ttf
nexrender secret set MAPBOX_TOKEN
```

Every command acts on the Nexrender team connected to the active API key.

## Let an agent drive

The CLI ships with a Nexrender skill for coding agents. `nexrender setup` installs it automatically for detected agents, or you can choose one yourself:

```bash
nexrender skill install --agent codex
nexrender skill install --agent claude
```

Then ask for the outcome you want:

> Upload `intro.aep`, wait until it is ready, and tell me its compositions and layer names.

> Submit `job.json`, follow the render, and show me the output when it finishes.

> Diagnose job `abc123`. Do not change or cancel anything.

The skill teaches the agent when to validate first, when confirmation is required, and how to keep credentials away from logs and presigned upload links. The standalone [nexrender-skill](https://github.com/nexrender/nexrender-skill) remains available for agents that cannot install or run the CLI.

## Made for scripts, too

Ask for JSON explicitly, or pipe a command and get JSON automatically:

```bash
nexrender job list --json
nexrender job list | jq '.data'
```

There is also a built-in jq filter for compact agent workflows:

```bash
nexrender job list --jq '.data[] | {id, status}'
nexrender template list --quiet --jq '.[].id'
```

Explore what the installed version can do without scraping help text:

```bash
nexrender commands --json
nexrender schema createJob --json
```

Create, cancel, delete, and upload operations are never retried after an ambiguous network failure. Destructive commands ask before acting unless you pass `--yes`.

## Help

Start broad, then zoom in:

```bash
nexrender --help
nexrender job --help
nexrender job submit --help
nexrender doctor
```
