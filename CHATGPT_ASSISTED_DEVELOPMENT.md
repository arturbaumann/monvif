# ChatGPT-assisted development notes

This project used ChatGPT 5.5 Thinking as a planning and debugging assistant alongside Claude Code.

## What ChatGPT helped with

- Designed version-by-version development scopes.
- Created structured Claude Code prompts.
- Interpreted ONVIF test output.
- Helped debug WS-Discovery behavior.
- Helped debug camera-specific diagnostic timing issues.
- Created Git commit and tag commands.
- Helped fix Go module path and GitHub install issues.
- Suggested release process documentation.
- Helped turn session lessons into reusable project practices.

## Most useful pattern

Use ChatGPT to create a precise Claude Code prompt, then use Claude Code to implement.

Flow:

1. Explain current repo state and failing output to ChatGPT.
2. Ask for a Claude Code prompt.
3. Paste prompt into Claude Code.
4. Run generated tests and real-world commands.
5. Paste results back into ChatGPT.
6. Iterate.
7. Commit/tag only after tests pass.

## Why this worked

ChatGPT was useful for reasoning, planning, and process.
Claude Code was useful for editing and implementation.

The combination worked better than asking either tool to do everything alone.

## Reusable rule

For future projects:

- Use ChatGPT for architecture, prompts, debugging interpretation, and release process.
- Use Claude Code for repo editing, tests, implementation, and refactoring.
- Keep changes small enough to review.
- Commit working milestones often.

