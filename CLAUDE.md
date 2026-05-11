# CLAUDE.md

## Purpose

Claude Code entry point for this repository.

## Precedence

- Canonical instructions: `AGENTS.md`
- If this file conflicts with `AGENTS.md`, follow `AGENTS.md`.

## Discoverability

- Global agent index: `.cursor/rules/documentation.mdc`
- Local slash commands: `.claude/commands/` (`/squash-bugbot <PR_NUMBER>` links to `.agents/skills/squash-bugbot/SKILL.md`)
- Repository docs: `README.md`, `docs/`
- Package-specific rules: `*/AGENTS.md`
- Review/security rules for Cursor ecosystem: `.cursor/BUGBOT.md`, `.cursor/rules/**`

## Note

This file intentionally stays short to avoid drift. Do not duplicate commands or conventions from `AGENTS.md`.
