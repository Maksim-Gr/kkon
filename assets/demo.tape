# kkon demo — render with:
#   docker run --rm -v "$PWD:/vhs" ghcr.io/charmbracelet/vhs assets/demo.tape
# Requires a Kafka Connect cluster at localhost:8083 (see docker-compose.yml)

Output assets/demo.gif

Set Shell "bash"
Set FontSize 19
Set Width 1240
Set Height 800
Set Padding 28
Set Theme "Catppuccin Mocha"
Set TypingSpeed 55ms

# Set up: clean prompt, put binary on PATH, configure kkon to reach Kafka Connect on the host.
Hide
Type 'export PS1="$ " && cp kkon-linux kkon && export PATH="$PWD:$PATH"'
Enter
Type 'mkdir -p ~/.config/kkon && printf "kafkaConnect:\n  url: http://host.docker.internal:8083\n  username: \"\"\n  password: \"\"\n" > ~/.config/kkon/config.yaml'
Enter
Type 'clear'
Enter
Show

Sleep 800ms

# ── List connectors with status badges ────────────────────────────────────────
Type "kkon connector list"
Sleep 400ms
Enter
Sleep 3s

# Select dev-heartbeat (first item) → Show config
Down
Sleep 500ms
Enter
Sleep 600ms
Enter
Sleep 3.5s

Sleep 1s

# ── List again: pick dev-broken (FAILED) → Show config ───────────────────────
Type "kkon connector list"
Sleep 400ms
Enter
Sleep 2.5s

Enter
Sleep 600ms
Enter
Sleep 3s

Sleep 2s
