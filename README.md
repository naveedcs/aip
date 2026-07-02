<h1 align="center">AIP (AI Profiles)</h1>

<p align="center">
  <em>One terminal per company. One AI profile per context. No more account bleed.</em>
</p>

<p align="center">
  <img src="https://img.shields.io/github/stars/naveedcs/aip?style=flat-square&color=111111&label=stars" alt="Stars">
  <img src="https://img.shields.io/badge/release-v0.1.1-111111?style=flat-square" alt="Release v0.1.1">
  <img src="https://img.shields.io/badge/tools-Codex%20%7C%20Claude%20%7C%20Gemini%20%7C%20Copilot-111111?style=flat-square" alt="Supported tools">
  <img src="https://img.shields.io/badge/license-MIT-111111?style=flat-square" alt="MIT license">
</p>

---

You have three clients, three terminals, four AI CLIs, and one very easy
mistake: launching the right agent with the wrong account, memory, MCP server,
or project folder.

AIP gives each work context its own profile. A profile carries the project
directory, AI tool homes, secrets, MCP servers, instructions, and safety label
for that context.

Profiles are persistent. The active profile is terminal-local.

## Before / after

Without AIP, every tool has its own way to isolate state:

```bash
export CODEX_HOME=~/.codex-client-a
export CLAUDE_CONFIG_DIR=~/.claude-client-a
export GEMINI_CLI_HOME=~/.gemini-client-a
export COPILOT_HOME=~/.copilot-client-a
```

With AIP:

```bash
eval "$(aip shell-init zsh)"
aip use client-a
codex
```

Or skip shell switching and run one command directly:

```bash
aip run client-a codex
```

## Install

```bash
go install github.com/naveedcs/aip/cmd/aip@latest
```

From a checkout:

```bash
go build -o aip ./cmd/aip
```

Homebrew, after the tap is published:

```bash
brew tap naveedcs/aip
brew install aip
```

Debian/Ubuntu, from a release `.deb`:

```bash
VERSION=v0.1.1
ARCH=amd64
curl -LO "https://github.com/naveedcs/aip/releases/download/${VERSION}/aip_${VERSION#v}_linux_${ARCH}.deb"
sudo apt install "./aip_${VERSION#v}_linux_${ARCH}.deb"
```

Windows, after the winget manifest is accepted:

```powershell
winget install NaveedCS.AIP
```

## First profile

```bash
aip init
aip profile create \
  --name client-a \
  --display-name "Client A" \
  --project-dir ~/projects/client-a \
  --safety read-only \
  --tools codex,claude,gemini,copilot \
  --yes
```

Log Codex into that profile's isolated Codex home:

```bash
aip login client-a codex
```

Run Codex in the profile:

```bash
aip run client-a codex
```

Check what AIP will load before launching anything:

```bash
aip doctor client-a
aip dry-run client-a codex
```

`doctor` reports names and paths. `dry-run` prints the launch environment with
secret values masked.

## Three terminals, three companies

Load shell integration once per terminal:

```bash
eval "$(aip shell-init zsh)"
```

Then each terminal can keep its own active profile:

```bash
# terminal 1
aip use client-a
codex

# terminal 2
aip use client-b
claude

# terminal 3
aip use client-c
gemini
```

The active selection is just `AIP_PROFILE` in that shell. Switching terminal 1
does not switch terminal 2.

To make the shell integration available in every new zsh terminal, put this in
`~/.zshrc` after `aip` is on your `PATH`:

```bash
eval "$(aip shell-init zsh)"
```

## How it works

When AIP launches a tool, it builds a small activation plan:

```text
1. Load ~/.aip/profiles/<profile>/profile.yaml
2. Set the tool's isolated home/config environment
3. Inject AIP_PROFILE and AIP_PROFILE_SAFETY
4. Resolve declared secrets from the keychain
5. Render supported MCP config for the target tool
6. Start the real AI CLI in the profile's project directory
```

The important bit: profile data lives on disk, but the active profile lives in
the current terminal.

## Supported tools

| Tool | Binary | Isolated by |
|---|---|---|
| Codex | `codex` | `CODEX_HOME` |
| Claude Code | `claude` | `CLAUDE_CONFIG_DIR` |
| Gemini CLI | `gemini` | `GEMINI_CLI_HOME` |
| GitHub Copilot CLI | `copilot` | `COPILOT_HOME` |

Detect installed tools:

```bash
aip tools detect
```

## Secrets

Secrets are profile-scoped and keychain-backed:

```bash
printf '%s\n' "$GITHUB_TOKEN" | aip secret set client-a GITHUB_TOKEN --stdin
aip secret list client-a
```

Secret values are not printed by `secret list`, `doctor`, or `dry-run`.

MCP servers can reference secrets with `${secret:NAME}`:

```bash
aip mcp add client-a github \
  --command npx \
  --arg -y \
  --arg @modelcontextprotocol/server-github \
  --env GITHUB_PERSONAL_ACCESS_TOKEN='${secret:GITHUB_TOKEN}'
```

## MCP

Add, inspect, test, and sync MCP servers per profile:

```bash
aip mcp add client-a docs --command npx --arg -y --arg some-mcp-server
aip mcp list client-a
aip mcp test client-a docs --offline
aip mcp sync client-a
```

`aip mcp sync` writes profile-specific MCP configs for Codex, Claude, Gemini,
and GitHub Copilot CLI. Copilot cannot expand `${VAR}` references in its MCP
config, so secret-bearing servers are skipped for Copilot unless you explicitly
allow plaintext secret rendering:

```bash
aip mcp sync client-a --allow-plaintext-secrets
```

Only use that flag when you accept that resolved secrets will be written into
Copilot's MCP config on disk.

## Honcho memory

Honcho can be attached to a profile as a managed MCP server:

```bash
aip secret set client-a HONCHO_API_KEY
aip honcho enable client-a --workspace-id client-a --user-name naveed
aip honcho show client-a
aip mcp test client-a honcho --offline
```

When enabled, AIP adds the reserved `honcho` MCP server and injects non-secret
Honcho SDK environment variables for launches. The API key stays behind the
profile secret path for Codex, Claude, and Gemini.

## Templates and sharing

Profiles can start from templates, be cloned for a higher-risk mode, or be
exported for team handoff:

```bash
aip profile templates
aip profile create --name client-a --project-dir ~/projects/client-a --template software-readonly
aip profile clone client-a client-a-admin
aip profile export client-a --out client-a.yaml
aip profile import client-a.yaml --name client-a --project-dir ~/projects/client-a
aip project init --profile client-a
```

## Commands

| Command | What it does |
|---|---|
| `aip init` | Create AIP storage under `~/.aip` |
| `aip profile create` | Create a profile, with flags or an interactive wizard |
| `aip profile list` | List saved profiles |
| `aip profile show <name>` | Show profile details |
| `aip use <profile>` | Make a profile active in the current shell |
| `aip deactivate` | Clear the current shell's active profile |
| `aip run <profile> <tool>` | Launch one tool inside one profile |
| `aip login <profile> <tool>` | Run a tool's login flow inside one profile |
| `aip doctor <profile>` | Check profile configuration and warnings |
| `aip dry-run <profile> <tool>` | Preview the launch plan with secrets masked |
| `aip secret set/list/rm` | Manage profile secrets |
| `aip mcp add/list/rm/sync/test` | Manage profile MCP servers |
| `aip honcho enable/disable/show` | Manage Honcho memory for a profile |
| `aip tools detect` | Show which supported CLIs are installed |
| `aip project init/show` | Manage project-local AIP config |

## Safety model

AIP is an isolation manager, not a sandbox.

It separates tool homes, profile files, secrets, MCP config, and launch
environment. It does not prevent a launched AI CLI from writing files if that
CLI has permission to write them.

Safety levels are profile metadata:

| Level | Intended use |
|---|---|
| `read-only` | Analysis, debugging, inspection |
| `standard` | Normal local development |
| `admin` | Production credentials or high-risk operations |

Launching an `admin` profile with `aip run` requires typing the profile name
before execution.

Treat production credentials accordingly.

## FAQ

**Does `aip use` switch the whole computer?**
No. It only sets `AIP_PROFILE` in the current terminal. Other terminals keep
their own active profile.

**Where are profiles stored?**
By default, under `~/.aip/profiles/<profile>`. Use `--home` or `AIP_HOME` for a
different AIP root.

**Can I run three profiles at the same time?**
Yes. Open three terminals, run `aip use <profile>` in each one, and launch the
tool you need from that terminal.

**How do I check whether Codex is logged into a profile?**
Run the tool inside the profile:

```bash
aip run client-a codex -- login status
```

Or check directly:

```bash
CODEX_HOME="$HOME/.aip/profiles/client-a/tools/codex" codex login status
```

**Does `read-only` block writes?**
No. It is launch metadata exposed as `AIP_PROFILE_SAFETY`. Use it to label and
steer the session; do not treat it as an OS sandbox.

## Development

```bash
go test -count=1 ./...
go vet ./...
go build -o aip ./cmd/aip
```

Publish a release by pushing a version tag:

```bash
git tag -a v0.1.1 -m "v0.1.1"
git push origin v0.1.1
```

The release workflow builds macOS, Linux, and Windows archives plus `.deb` and
`.rpm` packages.

## License

[MIT](LICENSE).
