# Copilot Instructions for this repository

## Project constitution

This repository is governed by the project constitution in [openspec/project.md](../openspec/project.md).

Treat that file as the single source of truth for:
- product scope and goals
- non-negotiable technical constraints
- architecture choices
- product principles
- assumptions and out-of-scope items
- capability areas covered by the OpenSpec documents

All prompts, code generation, spec updates, and implementation decisions must align with this constitution.

## Mandatory behavior for every prompt

1. Read [openspec/project.md](../openspec/project.md) before making implementation or planning decisions.
2. Use it as the default authority when requirements are unclear or appear to conflict.
3. If a request conflicts with the constitution, state the conflict clearly and either:
   - propose a compliant alternative, or
   - ask for explicit approval to deviate.
4. Do not silently override the project constitution with ad hoc assumptions.
5. If requirements are ambiguous, incomplete, or conflicting, do not guess; ask for clarification before proceeding.
6. Keep scope and wording aligned with the v1 product goals of the kids study app.

## Project guardrails

- Backend: Go, Chi, slog, TOML configuration.
- Frontend: static HTML/CSS/JS served by the Go binary; no Node/npm build step.
- Storage: JSON files on disk; no database for v1.
- Build rule: CGO must remain disabled for all dependencies.
- Deployment: single Go binary, Dockerized, deployed with the existing compose stack.
- UI language: Dutch for all kid- and parent-facing content.
- Keep all user-facing strings centralized and reviewable.
- All code must be tested with appropriate unit and integration tests.
- All code must be documented with clear and concise comments.

## Spec flow

Detailed implementation guidance lives in the OpenSpec specs under [openspec/specs](../openspec/specs/). Use the constitution as the top-level policy and the specific spec files for execution details.

If a prompt asks for a feature that is not defined in the constitution or specs, do not assume it is in scope unless it is explicitly approved.
