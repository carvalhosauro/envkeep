# Shell hook: cd into a worktree whose .env drifted and get warned — no
# command to remember.
Output demo/hook.gif

Require envkeep

Set Shell "bash"
Set FontSize 20
Set Width 1100
Set Height 560
Set Padding 20
Set Theme "Catppuccin Mocha"
Set TypingSpeed 55ms

Hide
# Source the hook, then create drift: rotate a key in `shop` and push, so
# `shop-payments` is now behind its vault.
Type `eval "$(envkeep hook bash)"`
Enter
Type "cd /tmp/envkeep-demo/shop"
Enter
Type "sed -i 's/sk_test_demo_1234/sk_test_demo_ROTATED/' .env && envkeep push"
Enter
Type "clear"
Enter
Show

Type "cd ../shop-payments"
Sleep 500ms
Enter
Sleep 3s

Type "envkeep pull"
Sleep 500ms
Enter
Sleep 4s
