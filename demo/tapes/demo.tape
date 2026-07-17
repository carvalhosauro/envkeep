# Hero gif: the 20-second tour — status, switch env, .env rewritten.
Output demo/demo.gif

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

Type "envkeep use staging"
Sleep 500ms
Enter
Sleep 2s

Type "cat .env"
Sleep 500ms
Enter
Sleep 3s

Type "envkeep use dev"
Sleep 500ms
Enter
Sleep 2s

Type "envkeep status"
Sleep 500ms
Enter
Sleep 4s
