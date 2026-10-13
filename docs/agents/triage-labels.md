# Triage Labels

The skills speak in terms of five canonical triage roles. This file maps those roles to the actual label strings used in this repo's issue tracker.

| Label in mattpocock/skills | Label in our tracker | Meaning                                  |
| -------------------------- | -------------------- | ---------------------------------------- |
| `needs-triage`             | `needs-triage`       | Maintainer needs to evaluate this issue  |
| `needs-info`               | `needs-info`         | Waiting on reporter for more information |
| `ready-for-agent`          | `ready-for-agent`    | Fully specified, ready for an AFK agent  |
| `ready-for-human`          | `ready-for-human`    | Requires human implementation            |
| `wontfix`                  | `wontfix`            | Will not be actioned                     |

When a skill mentions a role (e.g. "apply the AFK-ready triage label"), use the corresponding label string from this table.

Edit the right-hand column to match whatever vocabulary you actually use.

## Board mirroring

This repo also tracks issues on a GitHub Project board (`csv-to-json-issues`, project 9) carrying two single-select fields:

- **`Kind`** — the triage role. Its options are the four label strings below verbatim, so it mirrors the label one-to-one and can be read back faithfully.
- **`Status`** — workflow position (Backlog / Ready / In progress / In review / Done), which is many-to-one and *cannot* be read back as a triage state.

`wontfix` has no `Kind` option: it maps to `Status: Done` with `Kind` cleared. Setting `Status: Done` auto-closes the issue, so no separate close is needed — and no other triage transition may use `Done` for that reason.

Labels stay canonical — if a label and `Kind` ever disagree, fix the board. Full command reference in `docs/agents/issue-tracker.md`.
