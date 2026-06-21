# Gaia demo tape — generate with: vhs demo/gaia-demo.tape  (run from repo root)
# Requires: charmbracelet/vhs and a built binary at ./bin/gaia (run `make build`).
#
# Showcases: the CLI command palette, the unsafe local dev quickstart
# (auto init + unlock + seeded secrets), and the spring-animated TUI splash.

Output "demo/gaia-demo.gif"

Set Shell "bash"
Set FontSize 16
Set Width 1200
Set Height 720
Set Theme "Catppuccin Mocha"
Set TypingSpeed 50ms
Set Padding 20

# --- Hidden setup: put gaia on PATH, work in a throwaway dir, seed file ---
Hide
Type "export PATH=$PWD/bin:$PATH" Enter
Type "export GAIA_PASSPHRASE=devpass" Enter
Type "rm -rf /tmp/gaia-demo && mkdir -p /tmp/gaia-demo && cd /tmp/gaia-demo" Enter
Type "printf 'common:\n  app_env: production\nstripe:\n  api_key: sk_test_demo_1234\ndatabase:\n  password: s3cr3t-pw\n' > seed.yaml" Enter
Type "clear" Enter
Show

# --- 1. The command palette ---
Type "gaia --help" Enter
Sleep 3s
Type "clear" Enter
Sleep 400ms

# --- 2. Local dev quickstart: auto init + unlock + seeded secrets ---
Type "# One command to spin up a local dev daemon with seeded secrets" Enter
Sleep 300ms
Type "gaia dev --seed seed.yaml" Enter
Sleep 4s
Ctrl+C
Sleep 800ms
Type "clear" Enter
Sleep 400ms

# --- 3. TUI peek: the spring-animated splash (bare `gaia` launches the TUI) ---
Type "# Launch the interactive TUI" Enter
Sleep 300ms
Type "gaia" Enter
# Let the harmonica splash drop, reveal, and settle into the main menu.
Sleep 5s
Type "q"
Sleep 1.2s
