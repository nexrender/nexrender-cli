# Calling the API directly

The CLI is a thin wrapper over the Nexrender Cloud REST API. Prefer the CLI. Call the API directly only when the CLI cannot do what is needed, for example when an older CLI does not know a new job setting, and tell the user that you did. The authorization boundaries in SKILL.md still apply: a `POST /jobs` is a paid render, and a create or cancel request is never retried after an ambiguous failure.

## Base URL

Each profile has its own API base URL:

```bash
nexrender profile list --json
```

For example `https://api.nexrender.com/api/v2` (production) or `https://staging.api.nexrender.com/api/v2` (staging). The URL the CLI would actually use, after `--server` and `NEXRENDER_API_URL`, comes from:

```bash
nexrender --profile staging auth status --jq '.data.server'
```

That command also reports `credentialSource` (`environment`, `keyring` or `file`) without printing the token.

## Authentication

Send `Authorization: Bearer <token>` with every API request. Never send it to presigned upload or download URLs, such as `outputUrl` or a template's upload URL.

The CLI looks for the token in this order:

1. `NEXRENDER_API_KEY`. When it is set, the CLI uses it for every profile.
2. The system keyring, service `nexrender-cli`, account = profile name. On macOS that is the login keychain, where go-keyring stores the token base64-encoded after a `go-keyring-base64:` prefix. On Linux it is the Secret Service item with attributes `service=nexrender-cli` and `username=<profile>`. On Windows it is the generic credential `nexrender-cli:<profile>`.
3. `<UserConfigDir>/nexrender/credentials.json`, a JSON object of profile name to token. That is `~/Library/Application Support/nexrender/credentials.json` on macOS, `~/.config/nexrender/credentials.json` on Linux and `%AppData%\nexrender\credentials.json` on Windows.

Never ask the user for the token, and never print it, write it to a file, put it in a URL or pass it as a command-line argument, where other processes can read it. Read it and use it in a single command. On macOS:

```bash
profile=staging
base=$(nexrender --profile "$profile" auth status --jq '.data.server')
token=${NEXRENDER_API_KEY:-}
if [ -z "$token" ]; then
  token=$(security find-generic-password -s nexrender-cli -a "$profile" -w)
  case $token in go-keyring-base64:*) token=$(printf %s "${token#go-keyring-base64:}" | base64 -d) ;; esac
fi
printf 'Authorization: Bearer %s\n' "$token" | curl -sS -H @- "$base/jobs?limit=5"
```

When `credentialSource` is `file`, read the token with `token=$(jq -r --arg p "$profile" '.[$p]' "$HOME/Library/Application Support/nexrender/credentials.json")` instead.

`printf` is a shell builtin, and `curl -H @-` reads the header from stdin, so the token stays out of the process list. Do not run this with `set -x`. Because stdin carries the header, send a request body from a file with `--data-binary @job.json -H 'Content-Type: application/json'`.

## Endpoints

Paths are relative to the base URL.

| Request | Operation ID | CLI equivalent |
| --- | --- | --- |
| `POST /jobs` | `createJob` | `job submit` |
| `GET /jobs/{id}` | `getJob` | `job show` |
| `GET /jobs/{id}/logs` | `getJobLogs` | `job logs` |
| `GET /jobs` | `listJobs` | `job list` |
| `POST /jobs/cancel` with `{"jobs": ["<id>", ...]}` | `cancelJobs` | `job cancel-many` |
| `PATCH /jobs/{id}/cancel` | `cancelJob` | `job cancel` |

For the exact request and response shapes:

```bash
nexrender schema createJob --json
```

The schema is the API version the installed CLI was built against. If the API is newer, the [API docs](https://docs.nexrender.com) are the reference.

Errors come back as JSON with an `error` message, and often an `errorCode` such as `INVALID_SETTINGS`. Validation errors (400) can add a `details` list of `{field, message}`.

## Lottie export

A job with `settings.type: "lottie"` exports the template to Lottie with Bodymovin instead of rendering a video. `settings.lottie` takes Bodymovin's settings object:

```json
{
  "template": { "id": "<template id>" },
  "settings": { "type": "lottie", "lottie": { "should_encode_images": true } }
}
```

- The output is a zip (`outputUrl` ends in `.zip`) holding a `lottie/` folder: `lottie/data.json`, with its images in `lottie/images/` or embedded in `data.json` with `should_encode_images`, plus `lottie/banner/`, `lottie/demo/`, `lottie/standalone/` or `lottie/report/` when `export_modes` turns them on.
- Only some settings can be changed, among them `export_modes`, `should_encode_images` and `rasterize_text`. Every other setting must keep its default. A changed or unknown setting, or `frames`, `codec` or `quality` with a Lottie job, is refused with 400 `INVALID_SETTINGS`. `--dry-run` does not check any of this.
- `rasterize_text: true` turns text into images, so the player needs no fonts. Otherwise text stays live and the player has to load its fonts.
- Lottie is turned on per team. Without it the API answers 403 `LOTTIE_DISABLED`.
- Full list of settings: https://docs.nexrender.com/cloud/jobs/initiating_renders#lottie-export

A CLI older than the Lottie export drops `settings.lottie` without an error. Check before a paid render:

```bash
nexrender job submit --file job.json --dry-run --jq '.data.settings.lottie'
```

If that prints `null`, submit the job with `POST /jobs` as above, after the user has approved the render.
