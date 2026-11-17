# Working rules

## Repository scope

- This repository holds shared Go packages for Buildkite. Keep application configuration types, defaults, and semantic validation in the consuming applications.
- Support Go 1.23. Do not use newer language features or standard-library APIs without agreement to raise the minimum version.
- Add shared behavior for concrete consumer needs. Do not add packages or extension points for hypothetical consumers.

## Subagents

- Delegate clearly bounded work and run independent work in parallel when it does not conflict.
- Only the coordinating agent delegates unless nested delegation is explicitly requested. Workers and reviewers must not recursively spawn more subagents.
- Keep responsibility for integration and verification with the coordinating agent. Delegation does not replace checking the results.

## Incremental implementation

- Break larger implementation slices into logical, self-contained subslices. Each subslice should have a clear outcome and its own validation. A small, coherent change may be a single subslice.
- Complete and validate one subslice before moving to the next. When committing is authorized, give each subslice an independent commit rather than bundling multiple subslices into one.

## Independent review

- At the end of every implementation subslice, use two fresh, read-only review subagents that did not implement it. Both must review the same production-code snapshot. This gate does not apply to routine editorial changes outside implementation subslices.
- Give the first reviewer repository and task context, tests, and code diffs. Ask it to identify problems, mistakes, regressions, and design issues across the changes and surrounding code, and assess the implementation against the agreed scope.
- Focus the second reviewer only on code quality, logic, bugs, and design. Provide a filtered workspace or input bundle containing only production source files under review and relevant surrounding production code. Exclude tests, fixtures, build/module files, code diffs, documentation, Git history, commit messages, plans, logs, design explanations, and the first reviewer's findings.
- Supply the second reviewer only minimal review instructions, without task rationale, intended behavior, or other explanatory prose. Do not supply normal repository context that automatically exposes `AGENTS.md` or any excluded materials. Instruct it not to read those materials or infer requirements that the code does not establish. If tooling cannot provide this separation, disclose the limitation rather than claiming a blind review.
- The code-only review complements, rather than replaces, the first review's assessment of intended behavior and requirements.
- Resolve feedback from both reviewers before completing the subslice or moving on. Fix actionable findings and briefly explain rejected or inapplicable findings. Do not silently discard feedback.
- Re-run affected checks after fixes. Material behavior or design changes made during review must receive follow-up independent review under the same context rules before completing the subslice or moving on.
- Follow-up reviews may use the original reviewers. Both must review the same updated production-code snapshot, with their original context boundaries preserved.

## Simple, idiomatic Go

- Prefer the standard library. Add dependencies, interfaces, wrappers, and configuration only when they solve a concrete need, not hypothetical future requirements.
- Prefer small, focused Go files organized by coherent responsibility. Keep closely related code together.
- Use one blank line between logical code blocks when the purpose changes. Keep closely related statements together, including an operation and its immediate error check, resource acquisition and its cleanup `defer`, and a lock with its protected updates and unlock. Keep comments attached to the code they describe. Do not separate every statement or brace-delimited block.
- After `gofmt`, explicitly review every added or edited Go function for these logical-block separators before considering the change ready. `gofmt` and `git diff --check` do not enforce this rule. Include this spacing check in review requests, and inspect final code rather than only diff hunks.
- Name test files `<owner>[_<topic>]_test.go` after the source file or responsibility they verify, not the fixture or entry point they call. Keep shared fixtures in `test_helpers_test.go`, and keep helpers used by a single suite in that suite's file.
- Order declarations within a file in newspaper style: the most important, highest-level declarations first, with supporting detail beneath. Types and entry points lead, callers precede callees, and private helpers sink below the code that uses them.

## Executable verification

- Format Go changes with `gofmt`. Before completing an implementation subslice, run `go test ./...`, `go test -race ./...`, and `go vet ./...` from each affected module root.
- Validate compatibility with the minimum supported Go version. Passing checks only on a newer toolchain does not establish Go 1.23 compatibility.
- Add regression tests for bugs. Report blocked or failing checks honestly.
- If a required check fails or cannot run, report the blocker to the user and keep the subslice incomplete. Do not advance past it until the blocker is resolved or the user explicitly grants an exception.

## Behavioral tests

- Use deterministic local fixtures. Test exported package behavior rather than private helper structure or incidental call order.
- Test shared behavior through the package API and retain application-specific contract tests in consumers. When extracting code, verify the affected consumer as well as the shared package.
- For every implementation subslice, review affected tests for redundancy, necessity, and clarity. Remove tests that add no distinct regression protection. Reduce duplication and unnecessary machinery without weakening assertions or obscuring intent. Remove fixtures and helpers that are no longer needed. Preserve coverage of distinct behavior and failure modes.

## Compatibility and safe failure

- Treat exported APIs and documented parsing rules as consumer contracts. Ask before materially changing agreed scope, accepted inputs, precedence, side effects, or security guarantees.
- For configuration loading, keep diagnostic messages free of configuration values. Name settings or file line numbers instead.
- Keep file discovery and process-environment mutation out of configuration loading. Read only an explicitly selected file and use process variables as overrides, including explicitly empty values.
- Give goroutines, open files, and other resources clear ownership and cleanup paths. Test cancellation and concurrency when the package exposes those behaviors.

## Documentation alignment

- Keep documentation about current code, not its delivery history. Update API documentation, examples, and design rationale when behavior changes.
- Keep reproducible verification commands and current limitations. Do not maintain implementation/decision logs, completed-slice narratives, or superseded alternatives.
- Keep each fact in one place and link to it elsewhere. Use the README for orientation and package documentation for API contracts and examples.
- Before deleting a duplicate, check whether it holds details the other copy lacks. Move those details to the canonical place instead of dropping them.
- Keep internal implementation detail out of consumer documentation unless it changes what the caller sees or must do. State the effect on the caller instead.
- When moving, renaming, or removing a heading, update incoming links and check that relative links and anchors resolve.

## Writing style

- Do not use semicolons in documentation, code comments, user-facing messages, or other prose. Split the sentence, or join the clauses with a conjunction or colon. Semicolons required by code syntax are fine.
- In user-facing documentation, address the reader as "you" and refer to library consumers as "callers" in code comments.
- Use American spelling and plain words. Prefer "who created it" over "provenance" and "isn't" over "is not" in user-facing prose.

## Documentation formatting

- Keep each prose paragraph and list item on one source line so edits do not reflow unrelated text. Keep paragraphs short and use subheadings when the topic changes.
- Merge bullets that make the same point, and give each bullet one idea. Lead with the instruction or outcome, then the detail.
- Use compact table syntax (`| --- |`) without column padding. Use real API and environment-variable names. Drop a column whose value is the same in every row, and state it once in prose.
- Make code blocks copy-pasteable. Give each platform its own block rather than one block with platform variables. Separate logical steps inside a block with blank lines, and keep a comment attached to the command it describes.
- Never place a line of `=` or `-` directly under prose, because Markdown turns the preceding line into a heading.

## Documentation accuracy

- Verify behavior claims against code before writing or rewording them, including supported types, limits, precedence, side effects, and error behavior. Test commands and examples when you can.
- Do not overstate guarantees, especially security ones. Scope each claim to the component and entry point it applies to.
- Mark third-party behavior you can't verify as tentative or link to its documentation.

## Commit messages

- When committing and pushing are authorized, push each completed commit to `origin` immediately after creating it. Otherwise, keep changes local and report what remains uncommitted or unpushed.
- Give every commit a readable summary of the changes it implements in the commit body, and summarize those changes when reporting the commit to the user.
- Do not add Amp branding or attribution to commit subjects, bodies, or trailers.
