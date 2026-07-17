# `envkeep use <env>`: switch this worktree's environment; .env is rewritten
# from the target vault (order/comments preserved, override reapplied).
Output demo/use.gif

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

Type "grep DATABASE_URL .env"
Sleep 500ms
Enter
Sleep 2s

Type "envkeep use staging"
Sleep 500ms
Enter
Sleep 2s

Type "grep DATABASE_URL .env"
Sleep 500ms
Enter
Sleep 4s
