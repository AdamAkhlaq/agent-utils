# Benchmark: agent-utils versus an agent without it

This directory holds the benchmark that measures what an agent gains from having `agent-utils` on its PATH: **speed** (wall-clock per task), **tokens and cost** (end to end, cache-aware), and **accuracy** (did the task get done right), compared with the same agent doing the same task without it. The method, the tasks, and the harness live here, and so will the per-trial data of every published run.

**What exists today:** the task format, the task suite under `tasks/` (filled in one command category per PR; `bench validate` lists the commands still without a task), the Docker images, the two context paragraphs, and the three subcommands: `validate`, `run`, and `report`. No run has been published yet; `results/` and the README's measured numbers arrive with the first full run.

The tool's authors run the benchmark. The harness is public, every prompt and checker is open to review, raw transcripts are published with each run, and flat or negative results are published too.

## Design rules

Each rule traces to a published failure of a tool benchmark (see [Sources](#sources)).

1. **Measure the whole task, not one layer.** The metric is cost and tokens for the full agent session until the task is done, never "bytes of tool output saved". Fewer bytes per turn can buy more turns and a higher bill.
2. **The baseline is the strongest honest competitor.** The comparison arm is a well-equipped shell (`python3` with PyYAML and Pillow, `jq`, `yq`, `openssl`, ImageMagick, coreutils) with a paragraph telling the agent those tools exist, not a crippled environment.
3. **Paired, same everything.** Same model, same Claude Code version, same image family, same prompt, same trial count; the only delta between arms is the binary and one paragraph of system-prompt context. Per-task deltas are computed on pairs, then averaged across tasks, so one expensive task cannot dominate the headline.
4. **Accuracy is scored alongside cost, deterministically, and fairly.** Every task has a checker that accepts any correct answer, so the baseline is never failed for a formatting choice the tool happens to make. Savings without a pass rate next to them are not reported.
5. **Enough trials, with intervals.** At least 5 trials per task per arm, 10 for the headline cells. Pass rate, pass^k (all k trials pass), median and mean with 95% bootstrap confidence intervals; never a single run.
6. **Cache-aware accounting.** Prompt caching stays on in every arm, because that is what users pay. Tokens are reported in four buckets (uncached input, cache write, cache read, output) and cost uses list prices per bucket.
7. **Interleave arms in time.** The trials of a task's arms run back to back in randomized order, so API latency and cache warmth hit all arms equally.
8. **Pin and publish everything.** Every run writes a manifest (model ID, Claude Code version, agent-utils version, image digests, prompt hashes, date, trial count), and the per-trial data and transcripts are published with the summary.
9. **Validate tasks with an oracle.** Each task ships the exact `agent-utils` invocation that solves it, and the harness runs it through the checker before any trial. A task whose oracle fails is broken, not hard.
10. **Classify infrastructure failures separately.** Rate limits, API errors, and container faults are retried and never counted as task failures.
11. **Disclose the conflict of interest.** See above.

## The four arms

"With agent-utils" and "without it" each mean two different things, so there are four arms. All four use the same image family and the same prompt; they differ only in what is on PATH, which tools Claude Code exposes, and the system-prompt context.

| Arm | What the agent has | Context paragraph | Claude Code flags (added to the common set) |
| --- | --- | --- | --- |
| `named` | The base image plus `agent-utils` on PATH | `context/tool.md` plus one generated sentence: "For this task, use `agent-utils <command>`. Run `agent-utils <command> -h` for its usage." | `--tools Bash,Edit,Glob,Grep,Read,Write --permission-mode bypassPermissions` |
| `tool` | The base image plus `agent-utils` on PATH | [`context/tool.md`](context/tool.md): the README's agent snippet, verbatim | `--tools Bash,Edit,Glob,Grep,Read,Write --permission-mode bypassPermissions` |
| `shell` | The base image: `python3`, `jq`, `yq`, `openssl`, ImageMagick, coreutils | [`context/neutral.md`](context/neutral.md): names the installed tools and says to prefer them | `--tools Bash,Edit,Glob,Grep,Read,Write --permission-mode bypassPermissions` |
| `model` | No command execution: only the `Read` and `Write` tools | `context/neutral.md` | `--tools Read,Write --permission-mode bypassPermissions` |

What an arm may do is fixed by its tool list, and every arm runs with `bypassPermissions` so no tool call is ever held for a permission the agent cannot grant in print mode (`acceptEdits` denies a `Write` to `/out`, which is outside the working directory). The tool and shell arms get every local tool and none that reaches the network: Claude Code's default set under `bypassPermissions` includes `WebSearch` and `WebFetch`, which would be a second channel out of the sandbox and a confound the proxy cannot see.

- `named` versus `shell` is the headline: the agent with the right command in hand against the same agent improvising with general tools, scored for accuracy the same way. The sentence names the command, never the invocation, and comes mechanically from `task.toml`, so nothing per task is hand-tuned; the agent still reads the usage, picks the mode and flags, and gets the output into the file. It is an upper bound on adoption, and the write-up says so.
- `tool` is the realistic deployment: install the binary, add the snippet, see what happens. The pilot showed Sonnet using agent-utils only when it happened to run `agent-utils help` first, and otherwise guessing command names or doing the work by hand, so `tool` measures discovery as much as benefit. The gap between `named` and `tool` is the discoverability cost, and every trial records `used_tool` (whether the shell history invoked the task's command), which the report shows as a usage rate per arm. Making agents adopt the tool (snippet wording, skills) is a separate question from what the tool saves, and this benchmark measures the second.
- `shell` is expected to pass most tasks; the interesting deltas are turns, tokens, cost, and time, plus the tasks where improvised one-liners get it wrong (DST arithmetic, exact rationals, CIDR edge cases, Unicode normalization, JWT with unpadded base64url).
- `model` is the accuracy story: the agent cannot run anything and does the transform "in its head", which is also what a chat answer is. It applies only to text tasks; image and binary tasks list only the other two arms. It keeps `Read` and `Write` so the checker reads a file in every arm and the cost of pulling a fixture into context is measured rather than guessed.
- `tool.md` and `neutral.md` are the only text that differs between the `tool`, `shell`, and `model` arms, and they are held to the same length (within 5 estimated tokens, by two estimators: a piece count that treats each run of letters, each digit, and each punctuation character as one piece, and bytes divided by four). Neither estimator is the model's tokenizer; they are used only to compare two texts measured the same way. `go test ./bench` enforces the limit and checks that `tool.md` is the README snippet verbatim. The `named` arm's context is `tool.md` plus the one sentence, about 20 tokens longer; that sentence is the treatment, and the text each trial saw is kept in the run directory (`context/<task>-named.md`) with the template in the manifest.
- The `tool` arm pays for its own `agent-utils help` and `-h` lookups. That is real cost and stays in the numbers.

## Isolation

- **One container per trial**, from images built once per run from [`image/Dockerfile`](image/Dockerfile) and pinned: the Node base image by digest, Claude Code by npm version, `yq` by version and SHA-256, and `agent-utils` by release tag with the release's checksums. The `tool` image installs agent-utils from a release, never from the working tree, so every published number maps to a release.
- **Non-root user** (`bench`), because `bypassPermissions` refuses to run as root.
- **Nothing from the host loads.** `--bare` cannot be used (it authenticates only with an API key, and the benchmark runs on a subscription token), so isolation is assembled by hand: an empty `CLAUDE_CONFIG_DIR`, `--setting-sources ""`, `--disable-slash-commands`, `--strict-mcp-config` with no MCP configuration, `--no-session-persistence`, and the environment variables set in the image (`CLAUDE_CODE_DISABLE_CLAUDE_MDS`, `CLAUDE_CODE_DISABLE_AUTO_MEMORY`, `CLAUDE_CODE_SKIP_PROMPT_HISTORY`, `DISABLE_AUTOUPDATER`, `CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC`, `DISABLE_TELEMETRY`, `DISABLE_ERROR_REPORTING`). The runner reads the `system`/`init` line of every trial's stream and fails the trial as `isolation_leak` if any MCP server, slash command, skill, or non-built-in plugin loaded, or the tool list is not the arm's.
- **Network**: trial containers sit on an internal Docker network (`--internal`, no route out) whose only egress is a sidecar running [`proxy/`](proxy/), a CONNECT proxy that tunnels to `api.anthropic.com:443` and refuses every other host with a 403 and a log line. The proxy log is kept with the run (`proxy.log`) and the manifest records the number of refusals. Fixtures never come from the network, and `video` is excluded from the suite. `BENCH_DOCKER=1 go test ./bench` includes a test that proves the three properties from inside a container: the API is reachable through the proxy, another host is refused, and a direct connection fails.
- **Credential**: a subscription token from `claude setup-token`, passed as `CLAUDE_CODE_OAUTH_TOKEN` through the environment, stored nowhere in the repository. The runs draw on the plan's rate limits, and `total_cost_usd` is Claude Code's list-price estimate rather than an invoice; it is the same basis in every arm, which is what a comparison needs.
- **Filesystem**: `/task` holds the task directory, mounted read-only; `/out` is an empty volume the checker inspects after the container exits. The checker runs in a separate container from the base image, so the agent cannot see or edit it, and no checker can depend on agent-utils.
- **Budgets**: `--max-turns`, `--max-budget-usd`, and a container timeout (`timeout_seconds` plus 30 seconds of grace for container start), all from `task.toml`. A trial that hits any of them is a failure with a recorded reason (`max_turns`, `max_budget_usd`, `timeout`).
- **Infrastructure failures are not results.** A trial whose result line carries an API error status, whose error text or stderr shows a rate limit, a 5xx, a network error, or an authentication failure, or that produced no result line at all, is retried up to three times with backoff (1, 2, then 4 minutes). Every attempt's transcript is kept (`.retry<n>` suffix); the trial record counts the retries. A trial that fails for infrastructure four times is recorded with `terminal_reason` `infra`. The report leaves `infra` and `isolation_leak` trials out of every pass rate, cost, and ratio, and lists them in their own section, so they are disclosed without being scored.

Common invocation, every arm (the entrypoint adds `-p` with the instruction):

```sh
claude -p "$(cat /task/instruction.md)" \
  --model <full id> --setting-sources "" --disable-slash-commands \
  --strict-mcp-config --no-chrome --no-session-persistence \
  --append-system-prompt-file /context.md \
  --tools <the arm's list> --permission-mode <the arm's mode> \
  --max-turns <max_turns> --max-budget-usd <max_budget_usd> \
  --output-format stream-json --verbose
```

## Task suite

### Coverage rule

Every command in `agent-utils` has at least one task under `tasks/`, except the exclusions in `task.go`, each with a reason: `video` (reaches the network), `commands` and `version` (meta). `go test ./bench` compares the command list with the task directories; a new command cannot merge without a benchmark task. (Until the suite covers every command, that test runs only with `BENCH_COVERAGE=1`, and `bench validate` prints the gaps as warnings.)

### Layout

Each task is a directory, following Terminal-Bench's conventions so a task can be exported to that harness for independent replication:

```
tasks/<command>/<case>/
  task.toml        # command, arms, tags, timeout, turn and budget caps
  instruction.md   # the prompt: an end goal, never a tool name
  fixtures/        # mounted read-only at /task/fixtures
  expected/        # golden outputs the checker compares against (optional)
  oracle.sh        # the agent-utils invocation that solves it; must pass check.sh
  check.sh         # exit 0 pass, 1 fail with the reason on stderr
```

`task.toml`:

```toml
command = "jwt-decode"              # must be an agent-utils command
arms = ["tool", "shell", "model"]   # "model" only when fixtures and outputs are text;
                                    # "named" follows "tool" and is never listed
tags = ["basic"]                    # basic | scale | hard | sweep
timeout_seconds = 300
max_turns = 40
max_budget_usd = 1.00
```

Tags: `basic` is the realistic small case an agent meets; `scale` tasks repeat a text command at roughly 1 KB, 30 KB, and 200 KB of input; `hard` tasks are the cases where improvised code tends to be wrong; `sweep` marks the subset the model sweep runs.

### Prompt rules

- State the goal and the output path under `/out/`: "Decode the JWT in `/task/fixtures/token.jwt` and write its payload claims as JSON to `/out/claims.json`."
- Say what "done" means, and end with the sentence "Do not ask questions; write the file and stop." (or "write the files and stop.").
- Never name a tool, flag, or approach. `bench validate` rejects an instruction that contains `agent-utils`, any command name, `python`, `python3`, `jq`, `yq`, `openssl`, `convert`, or `magick`, matched as whole words.
- The instruction is identical across arms, byte for byte; the arm is the environment.
- **Fixtures are novel.** Inputs are generated or arbitrary values that cannot be recalled: an unremarkable prefix in `172.29.0.0/16` rather than `10.0.0.0/8`, a date in an ordinary year and a zone with unusual rules rather than a famous DST change, random claims in a JWT, a YAML document nobody has seen. The first pilot showed Sonnet writing correct answers to textbook inputs without running anything, which measures recall, not the tool. A model that computes the right answer to a novel input in its head is a true finding, and the checker accepts it; a model that remembers the answer is not. Where a plausible shortcut gives the wrong answer (reasoning from the written address instead of the masked network, assuming a one-hour daylight shift), prefer the input that exposes it.

### Checker fairness

The checker enforces exactly what the instruction asked for and nothing more:

- JSON results are compared after parsing, so key order, whitespace, and number spelling do not matter unless the instruction specified them.
- Text results are compared after trimming surrounding whitespace.
- Images are decoded and checked for format and dimensions; lossless conversions also compare pixels.
- Yes/no tasks ask for a literal `true` or `false` in a file.
- Generators are checked structurally (a UUID's version and variant bits, a password's length and character classes).

Checkers run in the base image, where agent-utils does not exist, with `/task` and `/out` both read-only.

### Validation

`go run ./bench validate` checks every task:

- `task.toml` parses with no unknown keys, names an existing command that matches its directory, lists known arms (including `tool`, where the oracle runs) and tags, and has positive limits.
- `instruction.md` names an output under `/out/`, ends with the closing sentence, names no tool, and refers only to fixtures that exist.
- A task that lists the `model` arm has no binary fixture or output (by extension).
- `oracle.sh` and `check.sh` exist, are executable, and start with `#!`.
- `context/tool.md` is the README snippet verbatim, and `context/neutral.md` is within 5 estimated tokens of it.
- Every command has a task or an exclusion (a warning until the suite is complete; `-coverage` makes it a failure).
- Each oracle's output passes its checker, in Docker: the oracle runs in the `tool` image, offline, and the checker in the base image.

`-static` skips the Docker step, `-tasks 'jwt-decode/*'` restricts every check to the matching tasks, `-root` names the repository (default: found from the working directory), and `-agent-utils-version` chooses the release the tool image installs (default: the latest `v*` tag). Exit codes: 0 every check passed, 1 a check failed or Docker failed, 2 usage error (an unknown flag, a stray argument, a malformed glob, or a glob that matches no task).

`go test ./bench` runs the static checks on every task, and the Docker step when `BENCH_DOCKER=1` is set.

## Metrics

Per trial, one line of `trials.jsonl`, from the stream-json `result` line and the tool events before it: `passed` (the checker's verdict) and `check_reason`, `terminal_reason`, `exit_code`, the four token buckets (`input_tokens`, `cache_creation_input_tokens`, `cache_read_input_tokens`, `output_tokens`), `total_cost_usd`, `num_turns`, `tool_calls`, `bash_calls`, `bash_commands`, `used_tool`, `duration_ms`, `duration_api_ms`, `started_at`, the `transcript` path, `infra_retries`, and `rate_limit_utilization` (the plan's five-hour window as reported during the trial, or -1).

Per task per arm, over trials: pass rate, pass^k, median and mean of each metric.

Headline figures, across tasks:

- **Cost per solved task** (all spend including failures, divided by passes): the one number that cannot be gamed by failing cheaply.
- **Paired per-task ratios** (`named` and `tool`, each over `shell` and over `model`) for cost, tokens, turns, and duration: the median across tasks with a 95% bootstrap confidence interval over tasks.
- **Pass rate, pass^k, and tool usage rate** per arm, with the list of tasks each arm failed.
- **Token composition**: how much of each arm's input is cache reads versus uncached.

## Model matrix

- **Headline**: `claude-sonnet-5-5`, every task, every applicable arm, 10 trials per cell.
- **Sweep**: `claude-haiku-5-5`, `claude-sonnet-5-5`, and `claude-opus-5-5` on the tasks tagged `sweep`, 5 trials per cell, to show how the gap moves with model strength.

## Harness

A Go program in this directory, in the main module, with no new dependencies (Docker through `os/exec`, JSON from the standard library). It is a development tool, never part of the released binary.

- `go run ./bench validate`: the task checks above.
- `go run ./bench run -model <id> -trials N [-arms tool,named,shell,model] [-tasks glob] [-tag sweep] [-run-id id] [-concurrency k] [-agent-utils-version vX.Y.Z]`: validates the suite, builds the three images, writes `manifest.json`, brings up the network and proxy, runs every cell in planned order (tasks in name order, trials in order, and within each trial the task's arms in an order shuffled from a seed derived from the run-id, so the arms of one trial run back to back), appends each finished trial to `trials.jsonl`, and finishes with the report. The run directory is `bench/.runs/<run-id>/` (`-out` changes the parent), with `transcripts/`, `out/` (what every trial wrote), `proxy.log`, `manifest.json`, `trials.jsonl`, `summary.md`, and `summary.json`. The credential is read from `CLAUDE_CODE_OAUTH_TOKEN` and forwarded to containers by name only. Exit codes: 0 the run completed, 1 it stopped (or a precondition failed: no token, no Docker, a suite that does not validate), 2 usage error.
- **Resume**: a second `run` with the same `-run-id` skips every `(task, arm, trial)` already in `trials.jsonl` and refuses to continue if the model, the agent-utils release, or any selected instruction changed since the manifest was written.
- `go run ./bench report <run-dir> [<run-dir>...]`: recomputes `summary.md` and `summary.json` from `trials.jsonl` (and the versions from `manifest.json`). It is deterministic: the bootstrap is seeded from the run-id, so the same file always gives byte-identical output. One run prints its summary; several print the model sweep table, one row per run and arm.

Prerequisites for the Docker step and for trials: a Docker daemon (on macOS, `colima start`; if image pulls fail on DNS lookups inside the VM, `colima start --dns 1.1.1.1`), and for trials a token from `claude setup-token` exported as `CLAUDE_CODE_OAUTH_TOKEN`.

## What the repository keeps

Committed: this document; every task; the Dockerfile and entrypoint; the two context paragraphs; the harness and its tests; and, per published run, `results/<run-id>/manifest.json`, `trials.jsonl` (one line per trial with every metric), `summary.md`, and `summary.json`, plus `results/index.md` with one row per run. A run directory is immutable once published; a rerun is a new run-id. The root README quotes only a run that has a directory here.

Attached to the GitHub release of the measured version, as `bench-<run-id>.tar.gz` with its SHA-256 in the manifest: the full stream-json transcript of every trial and what each trial wrote to `/out`, so any verdict can be re-examined.

Not stored: the Docker images (rebuilt from the Dockerfile; the digest proves equivalence), the token, and pilot or aborted runs (`bench/.runs/`, local only).

## Caveats that go next to every table

- The numbers are for a specific model, Claude Code version, and agent-utils version, on a specific date; all three are printed next to every table.
- `total_cost_usd` is Claude Code's client-side estimate at list prices, not an invoice.
- If the `shell` arm's savings are small, the write-up says so, and the accuracy story rests on the `hard` tasks and the `model` arm.
- Only Claude Code is measured. Another harness may behave differently.
- The authors of the tool ran the benchmark.

## Sources

- Quesma, "Does RTK make AI coding cheaper?" (a 1,740-run Terminal-Bench replication that found a cost increase once whole-task cost was measured): https://quesma.com/blog/does-rtk-make-ai-coding-cheaper/
- A catalogue of token-saving tool claims and third-party checks: https://cc.bruniaux.com/token-savings/
- "An Empirical Cost Attribution of Context-Compression Gateways in Multi-Turn Coding Agents": https://arxiv.org/abs/2609.22114
- MLflow, "Benchmarking AI Agent Performance: A Practical Protocol": https://mlflow.org/articles/benchmarking-ai-agent-performance/
- Harbor task format and lifecycle: https://docs.harborframework.com/tasks/overview
- Claude Code headless mode, CLI reference, environment variables, prompt caching, and permission modes: https://code.claude.com/docs/en/headless, https://code.claude.com/docs/en/cli-reference, https://code.claude.com/docs/en/env-vars, https://code.claude.com/docs/en/prompt-caching, https://code.claude.com/docs/en/permission-modes
