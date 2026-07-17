# `envkeep use -c <env>`: create an environment from the current .env and
# switch to it — the `git checkout -b` of env files.
Output demo/create.gif

Require envkeep

Set Shell "bash"
Set FontSize 20
Set Width 1100
Set Height 560
Set Padding 20
Set Theme "Catppuccin Mocha"
Set TypingSpeed 55ms

Hide
Type "cd /tmp/envkeep-demo/shop && clear"
Enter
Show

Type "envkeep envs"
Sleep 500ms
Enter
Sleep 2s

Type "envkeep use -c preview"
Sleep 500ms
Enter
Sleep 2.5s

Type "envkeep envs"
Sleep 500ms
Enter
Sleep 2s

Type "envkeep status"
Sleep 500ms
Enter
Sleep 4s
