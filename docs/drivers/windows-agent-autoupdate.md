# Windows Agent Auto-Update

The Windows agent updates itself: every push to `master` publishes a new agent build on the
server, and each PC installs it within ~5 minutes. After the one-time install below, the agent
never needs to be reinstalled by hand.

The same mechanism is also a **watchdog**: if the agent process is killed (e.g. from Task Manager),
it is started again on the next updater run.

## Architecture

```
GitHub Actions (push to master)
  make build-win-agent            reproducible build, version = last commit touching agent code
  make publish-win-agent-manifest manifest.json = {version, sha256, size, ed25519 signature}
  scp  ->  /opt/metron/agent-updates/{metron-win-agent.exe, manifest.json}

Metron API (agent token auth)
  GET /v1/agent/update             manifest (404 when nothing is published)
  GET /v1/agent/update/download    binary

Windows PC
  MetronAgent   (task, user session, AtLogOn)       enforces sessions; restarts itself when its exe changes
  MetronUpdater (task, SYSTEM, startup + every 5m)  -mode updater: install update, then watchdog
```

### Update flow (`metron-win-agent.exe -mode updater`)

1. `GET /v1/agent/update`. If nothing is published or the installed binary already has the
   published SHA256, stop here.
2. If a public key is compiled into the running binary, verify the manifest signature.
   Unsigned or wrongly signed manifests are rejected **before** downloading.
3. Download to `metron-win-agent.exe.new`, check size and SHA256 against the manifest.
4. Self-check: run `metron-win-agent.exe.new -version`; it must print the manifest version.
   A binary that does not start is never installed.
5. Swap: `exe` becomes `exe.old`, `exe.new` becomes `exe`. On failure the old binary is restored.
   Windows allows renaming a running executable, so nothing has to be stopped.
6. Watchdog: if no other `metron-win-agent.exe` process is running, `schtasks /Run /TN MetronAgent`.

### Agent restart

The running agent checks its own exe every 30 seconds. When the file has changed and stayed stable
for one more check, it starts the new binary with the same arguments and exits. The new process polls
immediately, so enforcement gap is a few seconds at most.

### Versioning

`AGENT_VERSION` is the date and hash of the last commit that touched any Go package the agent
depends on (computed with `go list -deps`), `go.mod`/`go.sum` or the public key file. The build
uses `-trimpath -buildvcs=false`, so it is byte-identical across unrelated pushes. The updater
compares hashes, so PCs only update when agent code actually changed.

## Security model

| Threat | Protection |
|--------|------------|
| Child replaces the binary | `C:\Program Files\Metron` is writable only by Administrators/SYSTEM; the updater runs as SYSTEM |
| Child kills the agent | Watchdog restarts it within 5 minutes (child account must **not** be an administrator) |
| Tampered download / broken transfer | Size + SHA256 from the manifest |
| Compromised server or stolen agent token publishes a malicious binary | ed25519 signature; private key lives only in GitHub secrets |
| Broken build bricks the PC | Self-check (`-version`) before swap; previous binary kept as `.old` |

Without a signing key, updates are still protected by HTTPS, the agent token and SHA256, but
anyone who can write to `/opt/metron/agent-updates` on the server can push code that runs as
SYSTEM on the PC. **Setting up the key is strongly recommended.**

## Implementation plan (status)

- [x] `internal/agentupdate`: manifest, ed25519 sign/verify, hashing
- [x] Server: `agent_updates.dir` config (default `agent-updates` in the working directory,
      i.e. `/opt/metron/agent-updates`), `GET /v1/agent/update`, `GET /v1/agent/update/download`
- [x] Agent: `-version`, `-mode updater`, updater with self-check and rollback, exe watcher + self-restart,
      watchdog
- [x] `cmd/metron-agent-sign`: key generation and manifest signing
- [x] Makefile: version/key ldflags, reproducible build, `publish-win-agent-manifest`
- [x] CI: build, sign, publish agent on every push to `master`
- [x] Installer: `MetronUpdater` SYSTEM task, clean-up of `.old`/`.new`
- [ ] Generate signing key, add GitHub secret, commit public key (step 1 below)
- [ ] One-time reinstall on the PC (step 3 below)
- [ ] Verify on the real PC (checklist below)
- [ ] Later: self-enrollment (one-time code from the bot instead of editing `config.txt`)

## Setup

### 1. Signing key (once, recommended)

```bash
go run ./cmd/metron-agent-sign -genkey
```

- Put the **public** key on its own line at the end of `deploy/win-agent/update-public-key.txt`
  and commit it.
- Add the **private** key as GitHub secret `AGENT_SIGNING_KEY`
  (repo Settings, Secrets and variables, Actions). Never commit it.

Once the public key is committed, CI refuses to publish unsigned builds and agents built with that
key reject unsigned updates.

The key has to be in place **before** the one-time install (step 3), because the public key is
compiled into the installed binary. An agent installed without a key accepts the next signed build
by hash, and from then on requires signatures.

### 2. Merge to master

Merging this branch deploys the new API endpoints and publishes the first agent build.
Check on the server:

```bash
ssh metron@<server> 'cat /opt/metron/agent-updates/manifest.json'
```

No server config change is needed: `agent_updates.dir` defaults to `agent-updates` relative to the
working directory (`/opt/metron`). To use another path, add to `config.json` (the
`METRON_CONFIG_JSON` secret):

```json
"agent_updates": { "dir": "/opt/metron/agent-updates" }
```

### 3. One-time install on the PC

The currently installed agent has no updater, so it has to be reinstalled once.

1. On the dev machine (with `config.json` and `bot-config.json` present, so `config.txt` is filled in):
   ```bash
   make release-win-agent
   ```
   This builds `bin/metron-win-agent.zip`. For the build to match the one CI publishes, run it from an
   up-to-date `master` after the merge. Otherwise the updater simply replaces it with the CI build
   within 5 minutes.
2. Copy the zip to the PC, unpack, double-click `INSTALL.bat`, confirm UAC.
3. The installer registers both tasks (`MetronAgent` and `MetronUpdater`) and starts them.

Make sure the child's Windows account is a **standard user**, not an administrator. Otherwise the
child can stop the tasks or replace the binary.

### 4. Verify on the PC (PowerShell as Administrator)

```powershell
# Both tasks exist
Get-ScheduledTask -TaskName MetronAgent, MetronUpdater

# Installed version equals the published one
& "C:\Program Files\Metron\metron-win-agent.exe" -version

# Updater ran without errors
Start-ScheduledTask -TaskName MetronUpdater
Start-Sleep 10
Get-Content "C:\ProgramData\Metron\updater.log" -Tail 20
```

Checklist for the first real run:

- [ ] `updater.log` shows no errors; with nothing new published it only logs at debug level
- [ ] After a push that changes agent code: `update installed` in `updater.log`, then
      `agent binary replaced on disk, restarting` in `agent.log`, and `-version` shows the new version
- [ ] Watchdog: log in as the child, kill `metron-win-agent.exe` in Task Manager, wait up to 5 minutes.
      The agent must come back (`agent.log` shows a new start). If `updater.log` shows
      `schtasks /Run MetronAgent` errors, the task principal (Users group) does not support on-demand
      start from SYSTEM on this Windows edition. Report it: the fallback is launching the agent into the
      user session from the updater.
- [ ] After the self-restart the new agent process keeps running (`Get-Process metron-win-agent`).
      Task Scheduler may put task processes in a job object; if the restarted child dies together with
      the old process, the watchdog brings it back within 5 minutes, but the self-restart should then be
      reworked to go through `schtasks /Run`
- [ ] Lock still works right after logon (startup grace period from the previous fix)

## Rollback

- **Automatic:** a build that fails the `-version` self-check is never installed.
- **Bad build that starts but misbehaves:** revert the commit on `master`. CI publishes the previous
  agent code with a new version and PCs install it within 5 minutes (the updater installs whatever is
  published; it does not require a newer version).
- **Manual on the PC** (Administrator):
  ```powershell
  Stop-ScheduledTask -TaskName MetronUpdater
  cd "C:\Program Files\Metron"
  Get-Process metron-win-agent -ErrorAction SilentlyContinue | Stop-Process -Force
  Move-Item metron-win-agent.exe metron-win-agent.exe.bad -Force
  Move-Item metron-win-agent.exe.old metron-win-agent.exe -Force
  Start-ScheduledTask -TaskName MetronAgent
  ```
  Keep `MetronUpdater` stopped until a fixed build is published, otherwise it reinstalls the bad one.

## Troubleshooting

| Symptom | Where to look |
|---------|---------------|
| No updates arrive | `updater.log`; `GET /v1/agent/update` with the agent token must return the manifest |
| `reject update ... invalid update signature` | Public key in the binary does not match `AGENT_SIGNING_KEY`, or CI published unsigned |
| `new binary failed self-check` | Build is broken; it was not installed. Check the CI build |
| `remove previous backup` error | An old agent process still runs from `.old`; resolves after it restarts, or reboot |
| Agent not restarted after kill | `schtasks /Run` errors in `updater.log` (see checklist) |
