These fixtures were derived from real sessions supplied by the PR author:

- Pi: Niulai session `01a0fce1-4bdb-7357-909d-1fc983a98dad` (56 records).
- Codex: deer session `01a0fce4-eacb-7890-b42c-f140d51b593d` (43 records).

Record order, event types, timestamps, token counts and timing fields are
retained. Event, response, process and tool-call IDs are consistently replaced
with anonymous IDs, preserving their relationships. Only the two session IDs
above remain as fixture labels.

Prompts, replies, reasoning, tool arguments/results, command output, paths,
model/provider configuration and unknown fields are replaced with placeholders.
Account/user identifiers, instructions, signatures, encrypted reasoning,
environment/permission configuration, tool definitions, headers and URLs are
removed. Remaining string values were checked against an explicit allowlist.

These are sanitized real transcripts, not synthetic event sequences. Pi and
Codex have been tested end to end. Claude Code/Cowork, Oh My Pi, OpenCode and
Gemini CLI have format tests but are **not tested end to end** by the PR author.
