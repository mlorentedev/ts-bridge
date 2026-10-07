# Safe piped example

```sh
ts-bridge status --json | jq .ready
TS_VERBOSE=1 sudo ts-bridge connect --target desktop:3389 --auth-key-file ~/.ts-bridge/authkey
```
