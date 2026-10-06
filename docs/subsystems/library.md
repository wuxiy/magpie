# Library: instructions, MCP servers, skills and RTK

The library keeps what every agent should know in one place: the
instructions each agent reads before a conversation, the MCP servers it can
call, and the skills it can load. magpie writes these into each agent's own
files in that agent's format, and takes back only what it wrote. Everything
else in those files stays as it was. Package `internal/library` also gives
agents RTK, a CLI that cuts shell output down before the model reads it.

## Responsibilities and sources of truth

| Part | Responsibility | Source |
| --- | --- | --- |
| library.json | `Library` holds the instruction sets, the MCP servers, the skills, the projects, skill groups and copy settings. `Applied` records what magpie last wrote into each agent, so it only ever takes away its own. The instruction texts and skill folders live under `Dir` (`library/` in magpie's config folder). | [`library.go`](../../internal/library/library.go) |
| One change | `change` locks the library, loads it, applies one edit, saves, syncs every agent and saves again. Every exported mutation (`SaveServer`, `InstallSkills`, `SetSkillHow` and the rest) goes through it. One change runs at a time, whether from the page or the CLI. | [`library.go`](../../internal/library/library.go) |
| Targets | `Targets` lists the detected agents (`agent.Detected`), plus apps like Claude Desktop, with where each keeps its instructions file (and an `Override` it reads instead), its MCP file and its skills folder. | [`targets.go`](../../internal/library/targets.go) |
| Sync | `sync` writes each target's instructions and MCP servers. It then takes edits made in agents' skill copies into the library (`takeEdits`) and links or copies the skills. `~/.agents/skills` goes first, so an agent that reads it gets no second link. Then it syncs projects' skills. | [`sync.go`](../../internal/library/sync.go) |
| Instructions | magpie's part of an agent's instructions file sits between two marker lines. The rest of the file is the user's. Several named sets can be switched between (#106). | [`instructions.go`](../../internal/library/instructions.go) |
| MCP servers | A `Server` is a command or a URL (`http`, `sse`). `mcpFile` encodes it in each agent's own format (one `mcpFormat` per agent family) and merges it with the user's own fields for that entry. `Also` files get the same servers; `Extra` files are only read when importing. `CheckServers` connects as an agent would and keeps the result in memory only. A server that needs OAuth is signed in once in magpie, and agents reach it through the gateway's `/mcp/<name>` (#615). | [`mcp.go`](../../internal/library/mcp.go), [`mcp_check.go`](../../internal/library/mcp_check.go), [`mcp_signin.go`](../../internal/library/mcp_signin.go) |
| Skills | A skill is a folder with a `SKILL.md`. It comes from GitHub, from a folder on this machine (linked, so editing the folder edits the skill) or from the market. Each agent gets a link or a marked copy (#896, `SkillHow`). An agent in a WSL distro always gets a copy. | [`skills.go`](../../internal/library/skills.go), [`skill_how.go`](../../internal/library/skill_how.go), [`skill_edits.go`](../../internal/library/skill_edits.go), [`skill_from.go`](../../internal/library/skill_from.go) |
| Backups | Before a change first writes an agent file, `backups` copies it to `BackupDir`, one folder per change. The last `keepBackups` (30) changes are kept. | [`backup.go`](../../internal/library/backup.go) |
| RTK | `SetRTK` runs rtk's own installer (`rtk init -g …`) for an agent being switched on. For one being switched off, it removes exactly what the installer wrote, using magpie's own code, which works even with rtk gone. `ReadRTK` reads each agent's files to say which have the hook. `rtkSpec.blocked` stops the installer where it would break an agent. A detected agent RTK has no hook for at all is still listed, from `rtkNoHook`, with the reason and a switch that can't be turned on (dsh: its hooks can allow or deny a command but not rewrite it, and `rtk init` has no `--agent dsh`). | [`rtk.go`](../../internal/library/rtk.go) |

## Runtime path

1. **Edit.** The Library page or `magpie library` calls an exported function such as `SaveServer`, `ServerAgents`, `InstallSkills`, `SkillAgents` or `SetSkillHow`.
2. **Change.** `change` edits `library.json` under the lock and calls `sync`.
3. **Write.** For each target, `syncInstructions` and `syncMCP` write magpie's part in the agent's format and record it in `Applied`. Skills are linked or copied into the agent's skills folder. A copy edited since it was made is taken into the library first.
4. **Result.** The `Result` lists the agents changed, a `Problem` for each thing that couldn't be given, and the backup folder.

## Constraints and failure behavior

- magpie takes away only what `Applied` or its mark says it wrote. A folder of the user's with a skill's name is never touched. The user's own fields in an MCP entry are kept.
- An edit never silently loses. When an agent's copy of a skill and the library both changed, the newest edit wins, and the losing one is kept with the backups.
- A failure for one agent is a `Problem` in the result. It doesn't stop the others.
- A server an agent can't reach is not written there, and `mcpFile.supports` says why. For example, Claude Desktop takes no remote server, and Codex, Goose, dsh, Grok and Command Code take no SSE.
- Removing RTK never uses `rtk init --uninstall`. That command deletes Gemini's `GEMINI.md` whole and removes Claude Code, OpenCode and Cursor together.
- CC Switch's own skills folder is only ever read.

## Verification

```sh
go test -tags nogui ./internal/library -run 'TestTargets|TestServer|TestImportServer|TestCheck|TestInstructions|TestInstructionSets|TestSkill|TestEverySkillAgents|TestRTK'
go test -tags nogui ./internal/library
node --test internal/gui/tests/library-items.test.cjs internal/gui/tests/library-rtk-gain.test.cjs internal/gui/tests/library-rtk-nohook.test.cjs
```

The package's `TestMain` runs in a home of its own (`testenv`), never the
user's real agent files.
