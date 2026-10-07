# Unsafe second key in one unsplittable segment

> Warning: an inline auth key is visible in the process table.

```sh
(ts-bridge init --auth-key tskey-auth-ONE; ts-bridge connect --auth-key tskey-auth-TWO)
```
