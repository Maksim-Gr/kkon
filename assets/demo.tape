# kkon demo — render with:  vhs assets/demo.tape
# Requires a Kafka Connect cluster at localhost:8083 (see docker-compose.yml)
# and kkon configured to point at it.

Output assets/demo.gif

Set Shell "bash"
Set FontSize 19
Set Width 1240
Set Height 800
Set Padding 28
Set Theme "Catppuccin Mocha"
Set TypingSpeed 55ms

# Put the local kkon binary on PATH so the prompt reads cleanly.
Hide
Type 'export PS1="$ " && export PATH="$PWD:$PATH" && clear'
Enter
Show

Sleep 800ms
Type "kkon connector health-check"
Sleep 600ms
Enter
Sleep 4s

Type "kkon connector list"
Sleep 600ms
Enter
Sleep 2s
Down
Sleep 800ms
Enter
Sleep 4s

Sleep 1s
