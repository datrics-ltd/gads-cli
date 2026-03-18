#!/bin/bash
# Human-in-the-loop Ralph. Run once, watch what it does, run again.

claude --permission-mode acceptEdits "@PRD.md @progress.txt @SPEC.md \
1. Read the PRD and progress file. \
2. Find the next incomplete task (unchecked [ ] item) and implement it. \
3. Run any relevant tests or checks. \
4. Commit your changes with a conventional commit message (feat:, fix:, chore:, etc). \
5. Check the task off in PRD.md (change [ ] to [x]). \
6. Update progress.txt with what you did. \
ONLY DO ONE TASK AT A TIME. \
Refer to SPEC.md for technical details when needed."
