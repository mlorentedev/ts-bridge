# Changelog

## [1.11.1](https://github.com/mlorentedev/ts-bridge/compare/v1.11.0...v1.11.1) (2026-06-11)


### Bug Fixes

* remove stale proxy tests and harden integration tests ([#68](https://github.com/mlorentedev/ts-bridge/issues/68)) ([bb6553e](https://github.com/mlorentedev/ts-bridge/commit/bb6553ea67348bc3ebac2d72a23f258dca292990))

## [1.11.0](https://github.com/mlorentedev/ts-bridge/compare/v1.10.0...v1.11.0) (2026-06-11)


### Features

* implement ts-bridge init interactive setup wizard ([#66](https://github.com/mlorentedev/ts-bridge/issues/66)) ([f3de260](https://github.com/mlorentedev/ts-bridge/commit/f3de260863aede2c155acff25c4368d972fe8c6b))

## [1.10.0](https://github.com/mlorentedev/ts-bridge/compare/v1.9.0...v1.10.0) (2026-06-11)


### Features

* **cli:** add ts-bridge status subcommand ([#64](https://github.com/mlorentedev/ts-bridge/issues/64)) ([5215045](https://github.com/mlorentedev/ts-bridge/commit/52150455e1ee3ce43851604498e4acafa59ad7f3))

## [1.9.0](https://github.com/mlorentedev/ts-bridge/compare/v1.8.0...v1.9.0) (2026-06-11)


### Features

* **cli:** add ts-bridge connect subcommand with YAML config support ([#62](https://github.com/mlorentedev/ts-bridge/issues/62)) ([650d252](https://github.com/mlorentedev/ts-bridge/commit/650d2528ecded9e822ee337f53d84c2695930fd1))

## [1.8.0](https://github.com/mlorentedev/ts-bridge/compare/v1.7.2...v1.8.0) (2026-06-11)


### Features

* **cli:** scaffold Cobra CLI structure with version subcommand ([#59](https://github.com/mlorentedev/ts-bridge/issues/59)) ([d20c28a](https://github.com/mlorentedev/ts-bridge/commit/d20c28a9e5b748221d8f62dcba9c0990074676d9))

## [1.7.2](https://github.com/mlorentedev/ts-bridge/compare/v1.7.1...v1.7.2) (2026-05-18)


### Bug Fixes

* harden proxy under load — TOCTOU, dial timeout, half-close ([#34](https://github.com/mlorentedev/ts-bridge/issues/34)) ([c769153](https://github.com/mlorentedev/ts-bridge/commit/c7691538a8901160d769660a3291de76e1a99a8d))

## [1.7.1](https://github.com/mlorentedev/ts-bridge/compare/v1.7.0...v1.7.1) (2026-05-18)


### Bug Fixes

* case-insensitive truthy parsing in run.sh for Linux/Windows parity ([#29](https://github.com/mlorentedev/ts-bridge/issues/29)) ([ad0c090](https://github.com/mlorentedev/ts-bridge/commit/ad0c090df665b9a4f41b5a583e99aff3d951bf45))

## [1.7.0](https://github.com/mlorentedev/ts-bridge/compare/v1.6.0...v1.7.0) (2026-05-18)


### Features

* ReconnectDialer with exponential backoff for transient dial failures ([#24](https://github.com/mlorentedev/ts-bridge/issues/24)) ([d56ff7e](https://github.com/mlorentedev/ts-bridge/commit/d56ff7eb74690d56a171d0664a1e64be3e059547))

## [1.6.0](https://github.com/mlorentedev/ts-bridge/compare/v1.5.1...v1.6.0) (2026-05-18)


### Features

* TS_IDLE_TIMEOUT to close abandoned connections ([#21](https://github.com/mlorentedev/ts-bridge/issues/21)) ([e7a595e](https://github.com/mlorentedev/ts-bridge/commit/e7a595e5ce10b525b2edfca447ff92929c2ec462))

## [1.5.1](https://github.com/mlorentedev/ts-bridge/compare/v1.5.0...v1.5.1) (2026-05-18)


### Bug Fixes

* clean up tsnet server on init failure and surface auth hints ([#18](https://github.com/mlorentedev/ts-bridge/issues/18)) ([769fc9f](https://github.com/mlorentedev/ts-bridge/commit/769fc9f7ecf09cf193998e2025fc9b75e6f54340))

## [1.5.0](https://github.com/mlorentedev/ts-bridge/compare/v1.4.0...v1.5.0) (2026-03-08)


### Features

* graceful drain and multi-package refactoring ([#14](https://github.com/mlorentedev/ts-bridge/issues/14)) ([14355b7](https://github.com/mlorentedev/ts-bridge/commit/14355b7ba7824fcca79e10c7d120ebe649734f51))

## [1.4.0](https://github.com/mlorentedev/ts-bridge/compare/v1.3.1...v1.4.0) (2026-02-28)


### Features

* add TS_CONTROL_URL for custom control plane support ([1150e69](https://github.com/mlorentedev/ts-bridge/commit/1150e6945f9756ab340e0411ed5873d95cc41e50))
* upgrade tsnet v1.60.0 to v1.80.0 ([8da247b](https://github.com/mlorentedev/ts-bridge/commit/8da247b1abaa3150f8a269f817d07eb9fc934c3a))


### Bug Fixes

* accept hskey- auth key prefix for Headscale compatibility ([2135da3](https://github.com/mlorentedev/ts-bridge/commit/2135da3f2a44a69587696e4fc7e89a1b5ba1ff01))

## [1.3.1](https://github.com/mlorentedev/ts-bridge/compare/v1.3.0...v1.3.1) (2026-02-24)


### Bug Fixes

* harden Windows runtime and reduce noisy close logs ([8ee6f13](https://github.com/mlorentedev/ts-bridge/commit/8ee6f13d66ca411982111073e9e7c19382ad5104))
* harden Windows runtime behavior and refresh project documentation ([9026550](https://github.com/mlorentedev/ts-bridge/commit/9026550626a3d4552bbc33961f63f23f734e083a))

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
