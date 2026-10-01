# Unsafe compound example

> Warning: an inline auth key is visible in the process table.

```sh
ts-bridge init --auth-key-file /run/secrets/authkey; ts-bridge connect --auth-key tskey-auth-KEYID-SECRET
```
