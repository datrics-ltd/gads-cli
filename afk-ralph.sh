#!/bin/bash
# AFK Ralph loop. Runs Claude Code N times, one task per iteration.
# Usage: ./afk-ralph.sh <iterations>
set -e

if [ -z "$1" ]; then
  echo "Usage: $0 <iterations>"
  exit 1
fi

for ((i=1; i<=$1; i++)); do
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo "Ralph iteration $i/$1 — $(date)"
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

  result=$(claude --permission-mode bypassPermissions -p "@PRD.md @progress.txt @SPEC.md \
  1. Read the PRD and progress file. \
  2. Find the next incomplete task (unchecked [ ] item) and implement it. \
  3. Run any relevant tests or checks. \
  4. Commit your changes with a conventional commit message (feat:, fix:, chore:, etc). \
  5. Check the task off in PRD.md (change [ ] to [x]). \
  6. Update progress.txt with what you did. \
  7. Push to origin main. \
  ONLY WORK ON A SINGLE TASK. \
  Refer to SPEC.md for technical details when needed. \
  If ALL tasks in the PRD are complete, output <promise>COMPLETE</promise>.")

  echo "$result"
  echo ""

  if [[ "$result" == *"<promise>COMPLETE</promise>"* ]]; then
    echo "✅ PRD complete after $i iterations."
    exit 0
  fi

  echo "Iteration $i complete. Sleeping 5s before next..."
  sleep 5
done

echo "Finished $1 iterations. Check progress.txt for status."
