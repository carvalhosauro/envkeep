# The day-to-day pain envkeep targets: change a var in one worktree,
# push it to the vault, pull it from another worktree.
Output demo/drift.gif

Require envkeep

Set Shell "bash"
Set FontSize 20
Set Width 1100
Set Height 620
Set Padding 20
Set Theme "Catppuccin Mocha"
Set TypingSpeed 55ms

Hide
Type "cd /tmp/envkeep-demo/shop && clear"
Enter
Show

Type "sed -i 's/sk_test_demo_1234/sk_test_demo_ROTATED/' .env"
Sleep 500ms
Enter
Sleep 1s

Type "envkeep status"
Sleep 500ms
Enter
Sleep 2.5s

Type "envkeep push"
Sleep 500ms
Enter
Sleep 2.5s

Type "cd ../shop-payments && envkeep status"
Sleep 500ms
Enter
Sleep 2.5s

Type "envkeep pull"
Sleep 500ms
Enter
Sleep 2.5s

Type "grep STRIPE_KEY .env"
Sleep 500ms
Enter
Sleep 4s
