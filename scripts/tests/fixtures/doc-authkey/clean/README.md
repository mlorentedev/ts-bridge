# Clean examples

Use `--auth-key-file` so the key never enters the process table:

```sh
ts-bridge connect --target desktop:3389 --auth-key-file /run/secrets/authkey
```
