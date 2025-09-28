#!/bin/sh
# .bin/commit.sh

# Use gum to select the commit type
TYPE=$(gum choose "feat" "fix" "docs" "style" "refactor" "test" "chore" "revert")
# Use gum to write a one-line scope
SCOPE=$(gum input --placeholder "scope (e.g., TUI, daemon)")
# Use gum to write the summary
SUMMARY=$(gum input --value "$TYPE($SCOPE): " --placeholder "Summary of your changes")
# Use gum to write a detailed body
DESCRIPTION=$(gum write --placeholder "Details of your change (CTRL+D to finish)")

# Format the final commit message
COMMIT_MESSAGE="$SUMMARY\n\n$DESCRIPTION"

# Make the commit
git commit -m "$COMMIT_MESSAGE"
