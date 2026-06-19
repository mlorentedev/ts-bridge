# Changelog

## [1.15.1](https://github.com/mlorentedev/ts-bridge/compare/v1.15.0...v1.15.1) (2026-06-19)


### Bug Fixes

* **host:** honor bool-flag precedence, report actual RDP port, add Windows CI ([#194](https://github.com/mlorentedev/ts-bridge/issues/194)) ([65e5c41](https://github.com/mlorentedev/ts-bridge/commit/65e5c41bc5fc99bf54d4230df14f0ce606d16be4))

## [1.15.0](https://github.com/mlorentedev/ts-bridge/compare/v1.14.0...v1.15.0) (2026-06-19)


### Features

* **host:** parity host with client architecture ([#187](https://github.com/mlorentedev/ts-bridge/issues/187)) ([4aa13ec](https://github.com/mlorentedev/ts-bridge/commit/4aa13ec4a7588bcfb0a5f776ba87c37e7b2e7b64))

## [1.14.0](https://github.com/mlorentedev/ts-bridge/compare/v1.13.0...v1.14.0) (2026-06-19)


### Features

* add 'ts-bridge discover' subcommand for tailnet host auto-discovery ([#171](https://github.com/mlorentedev/ts-bridge/issues/171)) ([e6f41b6](https://github.com/mlorentedev/ts-bridge/commit/e6f41b619d3992fc0e88e357feae1bb516639d51))

## [1.13.0](https://github.com/mlorentedev/ts-bridge/compare/v1.12.11...v1.13.0) (2026-06-18)


### Features

* SRE-grade structured logging with file output and rotation ([#169](https://github.com/mlorentedev/ts-bridge/issues/169)) ([0dfc030](https://github.com/mlorentedev/ts-bridge/commit/0dfc030130e8072fa2b838798e8c512753398d99))

## [1.12.11](https://github.com/mlorentedev/ts-bridge/compare/v1.12.10...v1.12.11) (2026-06-18)


### Bug Fixes

* **config:** derive LocalAddr in auto-mode — fixes BUG-021 ([#165](https://github.com/mlorentedev/ts-bridge/issues/165)) ([73c4699](https://github.com/mlorentedev/ts-bridge/commit/73c46995ad99ff30e306b7336c245531e83681a7))

## [1.12.10](https://github.com/mlorentedev/ts-bridge/compare/v1.12.9...v1.12.10) (2026-06-18)


### Bug Fixes

* **ci:** fix release build — use correct package path ([#162](https://github.com/mlorentedev/ts-bridge/issues/162)) ([6591ae8](https://github.com/mlorentedev/ts-bridge/commit/6591ae8217506fa02eca8a9fe0197156cd015dd2))

## [1.12.9](https://github.com/mlorentedev/ts-bridge/compare/v1.12.8...v1.12.9) (2026-06-17)


### Bug Fixes

* **ci:** fix build-matrix cross-compile package path ([#156](https://github.com/mlorentedev/ts-bridge/issues/156)) ([e2cb699](https://github.com/mlorentedev/ts-bridge/commit/e2cb699485a9340f42e7baa3d77220618c7097bc))
* **config:** resolve AutoInstance from full precedence chain (BUG-020) ([#151](https://github.com/mlorentedev/ts-bridge/issues/151)) ([fdc5cf4](https://github.com/mlorentedev/ts-bridge/commit/fdc5cf4a4f900cadfb6fbcd6895f0236b301a6aa))

## [1.12.8](https://github.com/mlorentedev/ts-bridge/compare/v1.12.7...v1.12.8) (2026-06-15)


### Bug Fixes

* TECH-009 (nolint comments) + TECH-006 (sanitizeHostnameLabel dedup) ([#145](https://github.com/mlorentedev/ts-bridge/issues/145)) ([97e813f](https://github.com/mlorentedev/ts-bridge/commit/97e813f197874e6f34d23a06eef8172f01cd5ee0))

## [1.12.7](https://github.com/mlorentedev/ts-bridge/compare/v1.12.6...v1.12.7) (2026-06-15)


### Bug Fixes

* use structured logger for YAML warnings (BUG-016) ([2493093](https://github.com/mlorentedev/ts-bridge/commit/24930932f827bfebc14c370393f9ea58dfc4e7a3))
* use structured slog logger for YAML warnings (BUG-016) ([#141](https://github.com/mlorentedev/ts-bridge/issues/141)) ([cdf6959](https://github.com/mlorentedev/ts-bridge/commit/cdf6959d4f25b483ca46d8be63d09eecc9fd407c))

## [1.12.6](https://github.com/mlorentedev/ts-bridge/compare/v1.12.5...v1.12.6) (2026-06-15)


### Bug Fixes

* batch init/connect bug fixes (BUG-003, BUG-017) ([#134](https://github.com/mlorentedev/ts-bridge/issues/134)) ([cfb2460](https://github.com/mlorentedev/ts-bridge/commit/cfb24605b52588469fe9adc17f2a26fdfdfd3d2e))
* default hostname fallback + banner order (BUG-009, BUG-010) ([#139](https://github.com/mlorentedev/ts-bridge/issues/139)) ([d8186df](https://github.com/mlorentedev/ts-bridge/commit/d8186df9e1f6c41b0cd9dab475343c8c1b1a51d4))
* validate target format before auth key in Merge() ([#137](https://github.com/mlorentedev/ts-bridge/issues/137)) ([284647f](https://github.com/mlorentedev/ts-bridge/commit/284647f042279d7b84cf623020f4a4119dc12958))

## [1.12.5](https://github.com/mlorentedev/ts-bridge/compare/v1.12.4...v1.12.5) (2026-06-12)


### Bug Fixes

* validate dial-retries rejects negative values (BUG-006) ([#130](https://github.com/mlorentedev/ts-bridge/issues/130)) ([d1f2104](https://github.com/mlorentedev/ts-bridge/commit/d1f21045e928a38343751a0f08cb174c35864241))

## [1.12.4](https://github.com/mlorentedev/ts-bridge/compare/v1.12.3...v1.12.4) (2026-06-12)


### Bug Fixes

* read --auth-key-file before Merge validation ([#128](https://github.com/mlorentedev/ts-bridge/issues/128)) ([0a5b9b2](https://github.com/mlorentedev/ts-bridge/commit/0a5b9b2b11758516b398f647117c3fdf34e1d84e))

## [1.12.3](https://github.com/mlorentedev/ts-bridge/compare/v1.12.2...v1.12.3) (2026-06-12)


### Bug Fixes

* batch bug fixes — BUG-011 through BUG-019 ([#122](https://github.com/mlorentedev/ts-bridge/issues/122)) ([932805d](https://github.com/mlorentedev/ts-bridge/commit/932805d3daa5d3f5f4160c92db984af142887d32))

## [1.12.2](https://github.com/mlorentedev/ts-bridge/compare/v1.12.1...v1.12.2) (2026-06-12)


### Bug Fixes

* sanitize firewall rule name to prevent PowerShell injection (BUG-012) ([#120](https://github.com/mlorentedev/ts-bridge/issues/120)) ([29549ba](https://github.com/mlorentedev/ts-bridge/commit/29549ba1e22351c85465093c665dbd8861828c4d))

## [1.12.1](https://github.com/mlorentedev/ts-bridge/compare/v1.12.0...v1.12.1) (2026-06-12)


### Bug Fixes

* HTTP body leak in fetchHealth + .env merge in writeYAMLConfig ([1983122](https://github.com/mlorentedev/ts-bridge/commit/1983122d3e3bdb651ed824359681d68cf4458d92))

## [1.12.0](https://github.com/mlorentedev/ts-bridge/compare/v1.11.1...v1.12.0) (2026-06-11)


### Features

* add 'ts-bridge host' subcommand for RDP host configuration ([73fbef8](https://github.com/mlorentedev/ts-bridge/commit/73fbef83785f6804ebb39851609b66ead8fc1396))


### Bug Fixes

* add #nosec G204 annotations to Linux subprocess calls ([6c95e70](https://github.com/mlorentedev/ts-bridge/commit/6c95e7065a66897229ecd0c21f94e80964f3222c))
* resolve lint and security CI failures ([59dc510](https://github.com/mlorentedev/ts-bridge/commit/59dc51031ff3118f49b74bfcc7f2dc2dce8cae39))

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
