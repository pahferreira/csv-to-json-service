# Issue tracker: GitHub Issues + GitHub Project

Issues and specs for this repo live as **GitHub issues**, and those issues are tracked on a **GitHub Project (Projects v2) board**. Use the `gh` CLI for all operations.

Issues are the source of truth for *content* (title, body, comments, labels). The Project board is the source of truth for *workflow position* (which column / Status an item sits in).

## Prerequisite: project scope

`gh project` commands and the `projectsV2` GraphQL field need a scope beyond `repo`. Without it every Project command below fails with `missing required scopes [read:project]`.

```bash
gh auth refresh -s read:project   # read-only: view boards, items, field values
gh auth refresh -s project        # read + write: also add items and set fields
```

Check what the current token has: `gh auth status`. If the Project commands fail on scopes, **say so and stop** rather than silently falling back to labels-only — the human needs to run the refresh, it's an interactive browser flow.

As of 2026-10-13 the token carries the full `project` scope, so board reads and writes both work.

---

## Issue operations

- **Create an issue**: `gh issue create --title "..." --body "..."`. Use a heredoc for multi-line bodies.
- **Read an issue**: `gh issue view <number> --comments`, filtering comments by `jq` and also fetching labels.
- **List issues**: `gh issue list --state open --json number,title,body,labels,comments --jq '[.[] | {number, title, body, labels: [.labels[].name], comments: [.comments[].body]}]'` with appropriate `--label` and `--state` filters.
- **Comment on an issue**: `gh issue comment <number> --body "..."`
- **Apply / remove labels**: `gh issue edit <number> --add-label "..."` / `--remove-label "..."`
- **Close**: `gh issue close <number> --comment "..."`

Infer the repo from `git remote -v`; `gh` does this automatically when run inside a clone.

---

## Project board operations

The owner for this repo's projects is the repo owner (`pahferreira`). Pass it as `--owner` to every `gh project` command.

### Discover the board

Project numbers and field IDs are not recorded in this file on purpose — they are looked up at runtime, so this doc doesn't rot when the board is renamed or rebuilt.

```bash
# Which projects exist for this owner?
gh project list --owner pahferreira --format json

# Which projects are linked to THIS repo specifically?
gh api graphql -f query='
  query {
    repository(owner: "pahferreira", name: "csv-to-json-service") {
      projectsV2(first: 10) { nodes { number title id } }
    }
  }'
```

If exactly one project is linked to the repo, use it. If several are, **ask which one** rather than guessing.

#### Worked example (verified 2026-10-13)

At the time of writing, exactly one project is linked to this repo. These values are a **convenience cache, not the source of truth** — if a command returns "could not resolve", re-run the discovery above rather than trusting this table.

| What | Value |
| ---- | ----- |
| Title | `csv-to-json-issues` |
| Number | `9` |
| Project ID | `PVT_kwHOAbTN0c4Bm2-A` |
| Status field ID | `PVTSSF_lAHOAbTN0c4Bm2-AzhleOCM` |
| Kind field ID | `PVTSSF_lAHOAbTN0c4Bm2-AzhleY2w` |

`Status` options — workflow position:

| Column | Option ID |
| ------ | --------- |
| Backlog | `f75ad846` |
| Ready | `61e4505c` |
| In progress | `47fc9ee4` |
| In review | `df73e18b` |
| Done | `98236657` |

`Kind` options — triage role, mirroring the labels one-to-one:

| Value | Option ID |
| ----- | --------- |
| `needs-triage` | `0017723c` |
| `needs-info` | `f9c5a0c5` |
| `ready-for-agent` | `044bb58a` |
| `ready-for-human` | `0ff609f9` |

Note `Kind` has **no `wontfix` option** — see the mapping section.

### Read the board

```bash
# Items on the board, with their issue number and field values
gh project item-list <number> --owner pahferreira --format json

# Field definitions — needed to resolve Status and its options
gh project field-list <number> --owner pahferreira --format json
```

`field-list` is what gives you the Status field's `id` and the `id` of each of its single-select options. Read the real option names from the board; don't assume a vocabulary.

**`item-list` is not trustworthy on this project.** Verified 2026-10-13: an item that provably existed on the board (correct project, unarchived, both fields set) was reported by `gh project item-list 9` and by the `ProjectV2.items` GraphQL connection as `totalCount: 0`, repeatably rather than as a transient lag. Read membership and field values **from the issue side** instead, which returned the item correctly:

```bash
gh api graphql -f query='
  query {
    repository(owner: "pahferreira", name: "csv-to-json-service") {
      issue(number: <n>) {
        projectItems(first: 10) {
          totalCount
          nodes {
            id
            project { number title }
            fieldValues(first: 20) {
              nodes {
                ... on ProjectV2ItemFieldSingleSelectValue {
                  name
                  field { ... on ProjectV2SingleSelectField { name } }
                }
              }
            }
          }
        }
      }
    }
  }'
```

So: **never conclude from an empty `item-list` that the board is empty or that a write failed.** Confirm against the issue-side query, or against the `PVTI_…` id that `item-add` returns.

### Add an issue to the board

Canonical two-step — create the issue, then add it by URL:

```bash
url=$(gh issue create --title "..." --body "..." | tail -1)
gh project item-add <number> --owner pahferreira --url "$url"
```

`gh issue create --project "<title>"` can do this in one call, but it is version-sensitive; prefer the two-step when correctness matters more than brevity.

### Move an item (set its Status)

Prefer the **by-name** form. It needs no node IDs: give the project number, point at the item by issue URL, and name the field and option:

```bash
gh project item-edit 9 --owner pahferreira \
  --url https://github.com/pahferreira/csv-to-json-service/issues/<n> \
  --field Status --value Ready
```

The **node-ID** form exists for scripts, and is the only option if a field or option name is ambiguous:

```bash
gh project item-edit \
  --project-id "<PVT_…>" \
  --id "<PVTI_…>" \
  --field-id "<PVTSSF_…>" \
  --single-select-option-id "<option id>"
```

Other field types use `--text`, `--number`, `--date` in place of `--value` / `--single-select-option-id`. Remove a value with `--clear`.

**One field per invocation.** For non-draft issues `item-edit` updates a single field per call, so setting both `Status` and `Kind` is always two commands.

---

## Labels vs. board Status

**Labels are canonical for triage.** The five triage roles in `docs/agents/triage-labels.md` are applied as issue labels, and `/triage` reads and writes those labels.

The board is a **mirror for human visibility**, not a second source of truth. When a skill changes an issue's triage label it should update the board to match — but the label is what `/triage` reads and writes, so on any disagreement the label wins and the board gets corrected.

### Label → board mapping

The board carries two single-select fields. `Status` is workflow position; `Kind` is the triage role, and its options mirror the labels one-to-one.

| Triage label | `Status` | `Kind` |
| ------------ | -------- | ------ |
| `needs-triage` | Backlog | `needs-triage` |
| `needs-info` | Backlog | `needs-info` |
| `ready-for-agent` | Ready | `ready-for-agent` |
| `ready-for-human` | Ready | `ready-for-human` |
| `wontfix` | Done | *cleared* (Done auto-closes the issue) |

Applying a triage label means **two** `item-edit` calls, one per field:

```bash
url=https://github.com/pahferreira/csv-to-json-service/issues/<n>
gh project item-edit 9 --owner pahferreira --url "$url" --field Status --value Ready
gh project item-edit 9 --owner pahferreira --url "$url" --field Kind   --value ready-for-agent
```

For `wontfix`, clear `Kind` rather than leaving the previous value behind — a closed-as-wontfix issue still reading `Kind: needs-triage` is stale and misleading:

```bash
gh project item-edit 9 --owner pahferreira --url "$url" --field Status --value Done
gh project item-edit 9 --owner pahferreira --url "$url" --field Kind --clear
```

> **`Status: Done` closes the issue by itself.** Verified 2026-10-13: setting `Status` to `Done` on this board closed the linked issue with no `gh issue close` call — the project has GitHub's built-in auto-close workflow enabled. Two consequences:
>
> - For `wontfix`, setting `Status: Done` is sufficient; a following `gh issue close` is redundant and will report *"already closed"*. Use `gh issue close --comment` only when you want the comment, and expect that warning.
> - **Never set `Status: Done` to mean "finished triaging".** It closes the issue. Triage moves items to `Backlog` or `Ready` only.

Three properties of this mapping:

- **`Kind` inverts; `Status` does not.** Because `Kind`'s options match the label strings exactly, a board item's `Kind` is a faithful read of its triage role — `Ready` alone is still ambiguous between agent and human, but `Ready` + `Kind: ready-for-agent` is not. Reading `Kind` to learn triage state is legitimate.
- **Labels remain canonical on conflict.** `Kind` and the label can still drift — a hand-edit on the board, or a failed second `item-edit` after the first succeeded. When they disagree, the label wins and the board should be corrected to match, not the other way round.
- **`In progress` and `In review` are not triage states.** No label maps to them; they track implementation and PR progress, moved by whoever is doing the work. A triage skill must never move an item *out* of `In progress` or `In review` because a label changed — update `Kind` and leave `Status` alone in that case.

## PRs as a triage surface

**PRs as a request surface: no.** _(Set to `yes` if this repo treats external PRs as feature requests; `/triage` reads this flag.)_

When set to `yes`, PRs run through the same labels and states as issues, using the `gh pr` equivalents:

- **Read a PR**: `gh pr view <number> --comments` and `gh pr diff <number>` for the diff.
- **List external PRs for triage**: `gh pr list --state open --json number,title,body,labels,author,authorAssociation,comments` then keep only `authorAssociation` of `CONTRIBUTOR`, `FIRST_TIME_CONTRIBUTOR`, or `NONE` (drop `OWNER`/`MEMBER`/`COLLABORATOR`).
- **Comment / label / close**: `gh pr comment`, `gh pr edit --add-label`/`--remove-label`, `gh pr close`.

GitHub shares one number space across issues and PRs, so a bare `#42` may be either: resolve with `gh pr view 42` and fall back to `gh issue view 42`.

## When a skill says "publish to the issue tracker"

Create a GitHub issue, then add it to the Project board.

## When a skill says "fetch the relevant ticket"

Run `gh issue view <number> --comments`. Pull the board Status too (`gh project item-list`) only when workflow position actually matters to the task.

## Wayfinding operations

Used by `/wayfinder`. The **map** is a single issue with **child** issues as tickets.

- **Map**: a single issue labelled `wayfinder:map`, holding the Notes / Decisions-so-far / Fog body. `gh issue create --label wayfinder:map`.
- **Child ticket**: an issue linked to the map as a GitHub sub-issue (`gh api` on the sub-issues endpoint). Where sub-issues aren't enabled, add the child to a task list in the map body and put `Part of #<map>` at the top of the child body. Labels: `wayfinder:<type>` (`research`/`prototype`/`grilling`/`task`). Once claimed, the ticket is assigned to the driving dev. Add children to the Project board as well.
- **Blocking**: GitHub's **native issue dependencies**, the canonical, UI-visible representation. Add an edge with `gh api --method POST repos/<owner>/<repo>/issues/<child>/dependencies/blocked_by -F issue_id=<blocker-db-id>`, where `<blocker-db-id>` is the blocker's numeric **database id** (`gh api repos/<owner>/<repo>/issues/<n> --jq .id`, _not_ the `#number` or `node_id`). GitHub reports `issue_dependencies_summary.blocked_by` (open blockers only, the live gate). Where dependencies aren't available, fall back to a `Blocked by: #<n>, #<n>` line at the top of the child body. A ticket is unblocked when every blocker is closed.
- **Frontier query**: list the map's open children (`gh issue list --state open`, scoped to the map's sub-issues / task list), drop any with an open blocker (`issue_dependencies_summary.blocked_by > 0`, or an open issue in the `Blocked by` line) or an assignee; first in map order wins.
- **Claim**: `gh issue edit <n> --add-assignee @me`, the session's first write.
- **Resolve**: `gh issue comment <n> --body "<answer>"`, then `gh issue close <n>`, then append a context pointer (gist + link) to the map's Decisions-so-far.
