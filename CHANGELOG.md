# Changelog

## [1.3.0](https://github.com/mlorentedev/ts-bridge/compare/v1.2.0...v1.3.0) (2026-02-24)


### Features

* add liveness/readiness health probes and pin CI actions ([10bc389](https://github.com/mlorentedev/ts-bridge/commit/10bc3897a20a6a2083341d6dbfff3fb37f64fd16))


### Bug Fixes

* add race detector to release tests and pin CI actions ([51e291e](https://github.com/mlorentedev/ts-bridge/commit/51e291ed56713c4b0557f87b476a3bbbc7468826))
* suppress gosec G117 false positive on Config.AuthKey ([af0ed6e](https://github.com/mlorentedev/ts-bridge/commit/af0ed6e192a2cb14f370ce7f0eea6091b70ada8a))

## [1.2.0](https://github.com/mlorentedev/ts-bridge/compare/v1.1.0...v1.2.0) (2026-02-02)


### Features

* add observability, connection limits, and structured logging ([ef71794](https://github.com/mlorentedev/ts-bridge/commit/ef7179401842fe189d233f9aed5c0a07e960ce95))


### Bug Fixes

* issue with gosec and shellcheck in CI ([276df20](https://github.com/mlorentedev/ts-bridge/commit/276df20e76346eb3273f2a186839fb97bdcbc9c6))
* shellcheck issue in testing suite ([83b433d](https://github.com/mlorentedev/ts-bridge/commit/83b433d8e9c8bf34b9350595451d3785221cb819))

## [1.1.0](https://github.com/mlorentedev/ts-bridge/compare/v1.0.3...v1.1.0) (2026-02-02)


### Features

* preserve state by default in client, add reset flag ([582b7ee](https://github.com/mlorentedev/ts-bridge/commit/582b7ee8609b5161ac267851a59983e2d9d5f796))

## [1.0.3](https://github.com/mlorentedev/ts-bridge/compare/v1.0.2...v1.0.3) (2026-01-28)


### Bug Fixes

* replace unicode box-drawing chars with ASCII for Windows compatibility ([03b7706](https://github.com/mlorentedev/ts-bridge/commit/03b7706ec8f8c10e6adcacf8d7ec2f82ad6cb53e))

## [1.0.2](https://github.com/mlorentedev/ts-bridge/compare/v1.0.1...v1.0.2) (2026-01-28)


### Bug Fixes

* update launch scripts to run binary instead of go run ([af3652b](https://github.com/mlorentedev/ts-bridge/commit/af3652bbdc96da8d8ea0d30a7e57aeaa21acfcef))

## [1.0.1](https://github.com/mlorentedev/ts-bridge/compare/v1.0.0...v1.0.1) (2026-01-28)


### Bug Fixes

* force release please update ([0d5cb92](https://github.com/mlorentedev/ts-bridge/commit/0d5cb928467129b8a54c3182ee4dfb4000302fac))

## 1.0.0 (2026-01-28)


### Features

* initial release ([ce31ad5](https://github.com/mlorentedev/ts-bridge/commit/ce31ad57a781e02cb6e13be843c5e4447ebe82a6))


### Bug Fixes

* add shellcheck directive for dynamic source ([8f01e83](https://github.com/mlorentedev/ts-bridge/commit/8f01e833af45fb89a93c6c9030bba9fe16560294))
* handle unhandled errors and tidy go.mod for CI ([b978834](https://github.com/mlorentedev/ts-bridge/commit/b978834d28ad6ac9da55fbae7ea542ee1538ee13))
* remove govulncheck from ci ([30f4d79](https://github.com/mlorentedev/ts-bridge/commit/30f4d79a0c81bb83e29b90c9152c6f6f51a143ea))
* use goinstall for golangci-lint to support go 1.25 ([cd443b0](https://github.com/mlorentedev/ts-bridge/commit/cd443b061b3c0ebc65424fe61ebf5cec6d9d5e0f))
