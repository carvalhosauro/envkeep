# `envkeep use <env> --cascade`: switch every worktree of the repo at once.
Output demo/cascade.gif

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

Type "envkeep status"
Sleep 500ms
Enter
Sleep 2.5s

Type "envkeep use staging --cascade"
Sleep 500ms
Enter
Sleep 2.5s

Type "envkeep status"
Sleep 500ms
Enter
Sleep 4s
