# Changelog

## [2.0.1](https://github.com/mlorentedev/ts-bridge/compare/v2.0.0...v2.0.1) (2026-11-14)


### Bug Fixes

* **host:** stop reporting the firewall as failed when no service is declared ([#798](https://github.com/mlorentedev/ts-bridge/issues/798)) ([1693014](https://github.com/mlorentedev/ts-bridge/commit/169301444135ac42c6e5606833e9ce130807de22))

## [2.0.0](https://github.com/mlorentedev/ts-bridge/compare/v1.20.0...v2.0.0) (2026-11-14)


### ⚠ BREAKING CHANGES

* **cli:** the root persistent `--config` flag is removed. It was documented as reserved and nothing read it; a command line that passes it to `status`, `auth`, `import` or `discover` now fails with `unknown flag: --config`. `init`, `host`, `connect` and `browser` keep their own `--config`.
* **host:** `ts-bridge host init` requires `--config` and writes the ADR-017 YAML host file instead of `TS_HOST_*` lines in `.env`. On Windows and Linux, scripts that run `host init` without `--config` fail. `host setup` and `host check` still read `TS_HOST_*` from a `.env` in the working directory.
* **host:** `host init` now requires `--config <file>` and writes the ADR-017 YAML host file that `host setup`, `host check` and `host serve` read. It no longer writes `TS_HOST_*` lines to `.env`. Re-run `host init --config <file>`, or write the host file by hand, and pass the same `--config` to setup, check and serve.

### Features

* **agent:** serve an identity-checked local API from a per-user agent ([#793](https://github.com/mlorentedev/ts-bridge/issues/793)) ([0b3eff4](https://github.com/mlorentedev/ts-bridge/commit/0b3eff4593da7534f1fc3ff16a623112275b02a7))
* **bootstrapssh:** log the destination of every bootstrap SOCKS dial ([#669](https://github.com/mlorentedev/ts-bridge/issues/669)) ([3090051](https://github.com/mlorentedev/ts-bridge/commit/30900515d8d532ab2aa663da7dfbd28ae07bd680))
* **discover:** save the chosen host as a profile with --profile ([#665](https://github.com/mlorentedev/ts-bridge/issues/665)) ([7165991](https://github.com/mlorentedev/ts-bridge/commit/716599109ff11c64a2622b142659ca93de7e1222))
* **host:** converge an ssh service on Windows hosts ([#781](https://github.com/mlorentedev/ts-bridge/issues/781)) ([4428ecb](https://github.com/mlorentedev/ts-bridge/commit/4428ecb6e87c58d84367d433b6cf79272f84c99e))
* **host:** converge and check the ssh service on Linux ([#765](https://github.com/mlorentedev/ts-bridge/issues/765)) ([9e6ba8d](https://github.com/mlorentedev/ts-bridge/commit/9e6ba8d54ee962d6a8f22cb671eb180df71a3e08)), closes [#521](https://github.com/mlorentedev/ts-bridge/issues/521)
* **host:** host init writes the YAML host file named by --config ([3e1a3e2](https://github.com/mlorentedev/ts-bridge/commit/3e1a3e248d37fcba1aaa24bdc79b8a020980e77f))
* **host:** host serve, a persistent tailnet node publishing the declared services (HOST-008) ([#560](https://github.com/mlorentedev/ts-bridge/issues/560)) ([f85eab3](https://github.com/mlorentedev/ts-bridge/commit/f85eab3cc697f9084513fd34733518aa47741f00))
* **host:** load and validate the declared host configuration file ([#519](https://github.com/mlorentedev/ts-bridge/issues/519)) ([4363338](https://github.com/mlorentedev/ts-bridge/commit/436333857c079d5d3b80578b54507045941ca6f5))
* **host:** open the Linux service port on tailscale0 only (iptables) ([#687](https://github.com/mlorentedev/ts-bridge/issues/687)) ([605e5d2](https://github.com/mlorentedev/ts-bridge/commit/605e5d233af2ba4e192382887e4e802627931691))
* **host:** read a --config host file in host setup and host check ([#554](https://github.com/mlorentedev/ts-bridge/issues/554)) ([ecace2e](https://github.com/mlorentedev/ts-bridge/commit/ecace2e369ecc2730fee05c84ba4a11a11983012))
* **host:** refuse a Linux ancestor whose POSIX ACL grants another principal write ([#679](https://github.com/mlorentedev/ts-bridge/issues/679)) ([635945a](https://github.com/mlorentedev/ts-bridge/commit/635945afc590fbd1b1313752212db55ae1bcf07f))


### Bug Fixes

* **auth:** refuse to share a credential name across control planes ([#664](https://github.com/mlorentedev/ts-bridge/issues/664)) ([398ead6](https://github.com/mlorentedev/ts-bridge/commit/398ead6a0ea81700e7611108ddc9491adebc6f4e))
* **bootstrapssh:** tie the ssh child to the parent's lifetime ([#597](https://github.com/mlorentedev/ts-bridge/issues/597)) ([a6bfe81](https://github.com/mlorentedev/ts-bridge/commit/a6bfe818cedb4b3cb6515ff17b3b75a531baba6d))
* **ci:** derive the toolchain guard's job and with: depth from the file ([#551](https://github.com/mlorentedev/ts-bridge/issues/551)) ([2b2a178](https://github.com/mlorentedev/ts-bridge/commit/2b2a1785896a884bbb165392e5b04c52fefe78de))
* **ci:** read persist-credentials only from the checkout step's with: block ([#552](https://github.com/mlorentedev/ts-bridge/issues/552)) ([46bdedc](https://github.com/mlorentedev/ts-bridge/commit/46bdedc8238d44e33f6ea4a9cceabdcc272a9c1a))
* **ci:** recognise go flags, go.exe and every go subcommand in the toolchain guard ([#555](https://github.com/mlorentedev/ts-bridge/issues/555)) ([aaf5e26](https://github.com/mlorentedev/ts-bridge/commit/aaf5e26bf7cf6bcb3fd3fb6c2d14f79102b7ce2b))
* **cli:** give auth key file errors one prefix and pin the init docs ([#549](https://github.com/mlorentedev/ts-bridge/issues/549)) ([88d0176](https://github.com/mlorentedev/ts-bridge/commit/88d01767fb8d3e09de2043ed0e7436c9b825bcf3))
* **cli:** hide the agent command until it manages sessions ([#794](https://github.com/mlorentedev/ts-bridge/issues/794)) ([8e27b00](https://github.com/mlorentedev/ts-bridge/commit/8e27b0040a90d6712f86cf2566142c19ba0333ac))
* **cli:** make discover --auto pick a host without prompting ([#711](https://github.com/mlorentedev/ts-bridge/issues/711)) ([639f6b4](https://github.com/mlorentedev/ts-bridge/commit/639f6b4447f65055c64edf5153f433722475f2a2))
* **cli:** name the next step on the import, auth set, connect path ([#645](https://github.com/mlorentedev/ts-bridge/issues/645)) ([82b1c69](https://github.com/mlorentedev/ts-bridge/commit/82b1c6918c3b101a2bd58e048f17bb1b5829ff8b))
* **cli:** remove the root --config flag that nothing reads ([#648](https://github.com/mlorentedev/ts-bridge/issues/648)) ([d19e0ad](https://github.com/mlorentedev/ts-bridge/commit/d19e0adbcfa8e61dc9927511bd6e851bb02df91a))
* **cli:** small message fixes from the pre-2.0.0 audit ([#650](https://github.com/mlorentedev/ts-bridge/issues/650)) ([ba44445](https://github.com/mlorentedev/ts-bridge/commit/ba444455a9345499d1a010cf918ea69809e24402))
* **config:** bind TCP mode to loopback when no listener is set ([#636](https://github.com/mlorentedev/ts-bridge/issues/636)) ([9554a27](https://github.com/mlorentedev/ts-bridge/commit/9554a2725f7c0aed1c5bffa6f4ff14625e386c9a))
* **config:** wire --verbose and TS_VERBOSE into Merge ([#658](https://github.com/mlorentedev/ts-bridge/issues/658)) ([72baa03](https://github.com/mlorentedev/ts-bridge/commit/72baa03023deb73c7460e798ff2af74330e78f53))
* **connect:** compare the override target's host case-insensitively ([#547](https://github.com/mlorentedev/ts-bridge/issues/547)) ([4de326c](https://github.com/mlorentedev/ts-bridge/commit/4de326c0db8e34070e05b5696ff2a5561d7b8e2c))
* **connect:** refuse a non-loopback health address without --health-allow-remote ([#659](https://github.com/mlorentedev/ts-bridge/issues/659)) ([3fd908d](https://github.com/mlorentedev/ts-bridge/commit/3fd908d8499e39b12be24ce504e7639fd4a5cda8))
* **discover:** ask for the API key each control plane needs ([#647](https://github.com/mlorentedev/ts-bridge/issues/647)) ([575c7d6](https://github.com/mlorentedev/ts-bridge/commit/575c7d699598fe4ad0dba4c96084bef9182efaab))
* **discover:** list Headscale nodes from /api/v1/node ([#731](https://github.com/mlorentedev/ts-bridge/issues/731)) ([38ed0ba](https://github.com/mlorentedev/ts-bridge/commit/38ed0ba05ee37157c3a080b7aae04852d72dbccf))
* **e2e:** make QA-606's acceptance checks fallible and lint the e2e scripts ([#703](https://github.com/mlorentedev/ts-bridge/issues/703)) ([7a8ad9d](https://github.com/mlorentedev/ts-bridge/commit/7a8ad9dc2840f44e3c2699f6aa554353edafaf95))
* **host:** add the tailscale0 rule when every allow is scoped off the tailnet ([#704](https://github.com/mlorentedev/ts-bridge/issues/704)) ([136c483](https://github.com/mlorentedev/ts-bridge/commit/136c4830fb6d7c94d447ff30aba70552d447debe))
* **host:** apply a declared RDP port to xrdp, and report one Windows does not listen on ([#660](https://github.com/mlorentedev/ts-bridge/issues/660)) ([00e8d26](https://github.com/mlorentedev/ts-bridge/commit/00e8d26e7199ccf21de7334787689d71e0bb33c5))
* **host:** carry the descriptor in host setup --json ([#587](https://github.com/mlorentedev/ts-bridge/issues/587)) ([e5adf4e](https://github.com/mlorentedev/ts-bridge/commit/e5adf4edaeb9bae7bb10d99bfc4be94aa9db6fa7))
* **host:** carry the host's real control plane in its descriptor ([#615](https://github.com/mlorentedev/ts-bridge/issues/615)) ([abcf51b](https://github.com/mlorentedev/ts-bridge/commit/abcf51b962d75ece7906fb14102d71f7902adbd4))
* **host:** check Windows power timeouts for drift ([#661](https://github.com/mlorentedev/ts-bridge/issues/661)) ([f88556d](https://github.com/mlorentedev/ts-bridge/commit/f88556df36337d30d122b2d56dbdb441d7da8f33))
* **host:** converge ssh.socket on socket-activated hosts ([#773](https://github.com/mlorentedev/ts-bridge/issues/773)) ([7de966d](https://github.com/mlorentedev/ts-bridge/commit/7de966d94948e207b8efbe1963eb5c7fe40651f4))
* **host:** credit iptables when it decides, and archive BUG-530 ([#577](https://github.com/mlorentedev/ts-bridge/issues/577)) ([3d37be5](https://github.com/mlorentedev/ts-bridge/commit/3d37be5be5a98a1127ff4556547252b37bb11087))
* **host:** cut iptables output into rules before any reader splits it ([#541](https://github.com/mlorentedev/ts-bridge/issues/541)) ([3683300](https://github.com/mlorentedev/ts-bridge/commit/36833007c7ecdd392c39bdbcadf1c768fdb3aaba))
* **host:** do not count or report an allow held to replies ([#761](https://github.com/mlorentedev/ts-bridge/issues/761)) ([f8ace2d](https://github.com/mlorentedev/ts-bridge/commit/f8ace2d22e7b22b89ba6261cb04a57563a4fea2e))
* **host:** fail a declared Linux service kind that has no step ([#769](https://github.com/mlorentedev/ts-bridge/issues/769)) ([c9177cf](https://github.com/mlorentedev/ts-bridge/commit/c9177cf0af55eb44c2b596a3867df8bf138a45cf))
* **host:** fail closed on a qualified jump into ts-input ([#533](https://github.com/mlorentedev/ts-bridge/issues/533)) ([aafda54](https://github.com/mlorentedev/ts-bridge/commit/aafda54f96c71359071309615d92ea4269e56624))
* **host:** flag unread services in check --json and stop blaming privilege ([#672](https://github.com/mlorentedev/ts-bridge/issues/672)) ([f77459d](https://github.com/mlorentedev/ts-bridge/commit/f77459d47fd2ba3c033e8694f7414470af004013))
* **host:** follow INPUT's jumps in the IPv6 exposure scan ([#752](https://github.com/mlorentedev/ts-bridge/issues/752)) ([3b72548](https://github.com/mlorentedev/ts-bridge/commit/3b72548c5d948a43269596cfec799a5605c159c3))
* **host:** follow INPUT's jumps into operator chains in the IPv4 exposure read ([#764](https://github.com/mlorentedev/ts-bridge/issues/764)) ([f369cfe](https://github.com/mlorentedev/ts-bridge/commit/f369cfecc401191b196baa2fd405a1ec3b786ac4))
* **host:** publish existing SSH services without convergence steps ([#698](https://github.com/mlorentedev/ts-bridge/issues/698)) ([e7cd305](https://github.com/mlorentedev/ts-bridge/commit/e7cd30548eaccbbc3d9b28e68ba757e1bbe82745))
* **host:** read a jump under an unmodelled match as an unknown exposure ([#775](https://github.com/mlorentedev/ts-bridge/issues/775)) ([f785fb4](https://github.com/mlorentedev/ts-bridge/commit/f785fb4b24d92a1c52da0721360d6241a42aad2c))
* **host:** read an unreadable IPv6 chain INPUT jumps to as unverified ([#771](https://github.com/mlorentedev/ts-bridge/issues/771)) ([354305c](https://github.com/mlorentedev/ts-bridge/commit/354305ced1499b2b575e8e0d5fea617d50ab70a2))
* **host:** read iptables on ufw hosts before allowing the port ([#567](https://github.com/mlorentedev/ts-bridge/issues/567)) ([7bd3fe7](https://github.com/mlorentedev/ts-bridge/commit/7bd3fe7a4aea5d72a39e2716f7262b42c06fd0f1))
* **host:** read RDP state from the policy key first on Windows ([#599](https://github.com/mlorentedev/ts-bridge/issues/599)) ([5c42542](https://github.com/mlorentedev/ts-bridge/commit/5c4254291bc471dd3eb4112ef701bc202195571b))
* **host:** read tailscaled's nodivert, nftables and IPv6 firewall rule sets on Linux ([#714](https://github.com/mlorentedev/ts-bridge/issues/714)) ([8814575](https://github.com/mlorentedev/ts-bridge/commit/8814575efa9cfcda828a2c1227bf5bc854803450))
* **host:** read tailscaled's ts-input chain before appending an open iptables rule ([#527](https://github.com/mlorentedev/ts-bridge/issues/527)) ([41f5a06](https://github.com/mlorentedev/ts-bridge/commit/41f5a0694b3c731387c55b86e4b8f7718d19aeea))
* **host:** read the Remote Desktop group by its resource id ([#716](https://github.com/mlorentedev/ts-bridge/issues/716)) ([e592426](https://github.com/mlorentedev/ts-bridge/commit/e592426187ea387892e0644719d698e0270a733c))
* **host:** read the Windows firewall rule before recreating it ([#600](https://github.com/mlorentedev/ts-bridge/issues/600)) ([402467e](https://github.com/mlorentedev/ts-bridge/commit/402467e463172363e1dcdcb3ac183bf260b4a93c))
* **host:** read Windows services, network profile and power before acting ([#611](https://github.com/mlorentedev/ts-bridge/issues/611)) ([a6f8721](https://github.com/mlorentedev/ts-bridge/commit/a6f87210e1e3233272ad50ac6392226a70a678d1))
* **host:** refuse a Unix auth_key_file under an ancestor others can write ([#612](https://github.com/mlorentedev/ts-bridge/issues/612)) ([aba1a0c](https://github.com/mlorentedev/ts-bridge/commit/aba1a0c48d1d520e1d7a9d1b5c1d1cb1f7888085))
* **host:** refuse a Unix state_dir under an ancestor others can write ([#596](https://github.com/mlorentedev/ts-bridge/issues/596)) ([9ab3b55](https://github.com/mlorentedev/ts-bridge/commit/9ab3b55e31e92613fa697de9b2ffc70c5a1ef5de))
* **host:** report a permissive iptables INPUT policy as manual ([#734](https://github.com/mlorentedev/ts-bridge/issues/734)) ([50f555c](https://github.com/mlorentedev/ts-bridge/commit/50f555c5508811211b05e4521f273e507bd05ce3))
* **host:** report a permissive IPv6 INPUT policy on iptables hosts ([#746](https://github.com/mlorentedev/ts-bridge/issues/746)) ([42e34ea](https://github.com/mlorentedev/ts-bridge/commit/42e34ea9d5fcbd3abfcff3e9aeaadf22773d9a1b))
* **host:** report a port-less accept that admits new states on IPv4 ([#779](https://github.com/mlorentedev/ts-bridge/issues/779)) ([b67eeaa](https://github.com/mlorentedev/ts-bridge/commit/b67eeaa8a069d87950e85a5ef19affccfadcea7d))
* **host:** report a source-port-only allow as exposing the port ([#759](https://github.com/mlorentedev/ts-bridge/issues/759)) ([dcddfee](https://github.com/mlorentedev/ts-bridge/commit/dcddfeef2ea38aeb06456f24ac88cbf0a8fdd243))
* **host:** report an active ufw's ACCEPT INPUT policy ([#748](https://github.com/mlorentedev/ts-bridge/issues/748)) ([9868909](https://github.com/mlorentedev/ts-bridge/commit/98689099475dedf54f3f92d529f1c046583b04ab))
* **host:** report an inactive ufw instead of claiming its rule admits the port ([#655](https://github.com/mlorentedev/ts-bridge/issues/655)) ([8b61b40](https://github.com/mlorentedev/ts-bridge/commit/8b61b40abb60c3c7a31c1e7ea2980753e65e2b35))
* **host:** report an IPv4 allow held only by a negated scope as exposing the port ([#776](https://github.com/mlorentedev/ts-bridge/issues/776)) ([1077ad8](https://github.com/mlorentedev/ts-bridge/commit/1077ad87a058f72c88c8f91c981976800b6a1a03)), closes [#743](https://github.com/mlorentedev/ts-bridge/issues/743)
* **host:** report copies of setup's own iptables rule in host check ([#534](https://github.com/mlorentedev/ts-bridge/issues/534)) ([50b0c3b](https://github.com/mlorentedev/ts-bridge/commit/50b0c3b4b1b391e6e7cbb63bdf80f2191e92987b))
* **host:** report firewalld and native nftables instead of reading iptables ([#750](https://github.com/mlorentedev/ts-bridge/issues/750)) ([f266385](https://github.com/mlorentedev/ts-bridge/commit/f2663855a8791747aa0b07952f7d62223fa36ce3))
* **host:** report leftover firewall copies from setup as check does ([#580](https://github.com/mlorentedev/ts-bridge/issues/580)) ([63871a3](https://github.com/mlorentedev/ts-bridge/commit/63871a3f38044f57eccc6fa4767cd7e651a59294))
* **host:** report negated and portless allows as exposing the port ([#742](https://github.com/mlorentedev/ts-bridge/issues/742)) ([8c0c0a4](https://github.com/mlorentedev/ts-bridge/commit/8c0c0a48b6bba3b07e1890e05344c5c2cce2c005))
* **host:** report ssh.service and ssh.socket both enabled as manual ([#780](https://github.com/mlorentedev/ts-bridge/issues/780)) ([cb4792f](https://github.com/mlorentedev/ts-bridge/commit/cb4792f1932cb0248391a9544b278289c3b0146e))
* **host:** report the host's real state in setup and check ([#613](https://github.com/mlorentedev/ts-bridge/issues/613)) ([b7e5460](https://github.com/mlorentedev/ts-bridge/commit/b7e5460a0ab7cff9b97d5748bee62723e7ca9647))
* **host:** report unattended mode as manual on a logged-out Windows node ([#756](https://github.com/mlorentedev/ts-bridge/issues/756)) ([c337705](https://github.com/mlorentedev/ts-bridge/commit/c337705bbd73abc0c0f76a3d40a41646bbbfd577))
* **host:** report unverified when tailscaled's ts-input is outside the iptables read ([#616](https://github.com/mlorentedev/ts-bridge/issues/616)) ([f80b57f](https://github.com/mlorentedev/ts-bridge/commit/f80b57feea8f01cff17d42f723540ea59aaa9a10))
* **host:** scope the Linux ufw allow to the tailnet interface ([#688](https://github.com/mlorentedev/ts-bridge/issues/688)) ([6d53dd6](https://github.com/mlorentedev/ts-bridge/commit/6d53dd69a5b0c7da0232d801124962a118b77dd8))
* **host:** scope the Windows firewall rule to the tailnet addresses ([#689](https://github.com/mlorentedev/ts-bridge/issues/689)) ([55cfd68](https://github.com/mlorentedev/ts-bridge/commit/55cfd68ba8ef0f91d8ecfc12669895e1498e216c))
* **host:** set Windows unattended mode with tailscale set, after reading it ([#594](https://github.com/mlorentedev/ts-bridge/issues/594)) ([88579c5](https://github.com/mlorentedev/ts-bridge/commit/88579c525b1b5c06e2664d023a5cd0f2c972bfe9))
* **host:** start the Tailscale service before the Windows unattended step ([#708](https://github.com/mlorentedev/ts-bridge/issues/708)) ([da6b81b](https://github.com/mlorentedev/ts-bridge/commit/da6b81b3b575e8662b10f90d83bfac2e926c6f87))
* **host:** warn before deleting a ufw allow that also lists SSH ([#774](https://github.com/mlorentedev/ts-bridge/issues/774)) ([4aae1b4](https://github.com/mlorentedev/ts-bridge/commit/4aae1b4f8a268183b12be1cc0bb85eff09e8005a))
* **host:** warn before removing a foreign allow that also lists SSH ([#770](https://github.com/mlorentedev/ts-bridge/issues/770)) ([6fc0fe7](https://github.com/mlorentedev/ts-bridge/commit/6fc0fe7e4ddf18f628ca1d8e629d20791db3232e))
* **init:** refuse a key flag without --target instead of discarding it ([#564](https://github.com/mlorentedev/ts-bridge/issues/564)) ([4011258](https://github.com/mlorentedev/ts-bridge/commit/4011258a8b56100167a7bfc4da66ce4e7bbec59b))
* **init:** refuse the wizard when stdin is not a terminal ([#649](https://github.com/mlorentedev/ts-bridge/issues/649)) ([b9bb19e](https://github.com/mlorentedev/ts-bridge/commit/b9bb19e9821c023019fd9de872cae157e63b3332))
* **profile:** bind a managed credential to its profile's control plane ([#637](https://github.com/mlorentedev/ts-bridge/issues/637)) ([af0b7a1](https://github.com/mlorentedev/ts-bridge/commit/af0b7a10dfe1d4b2200a0e71ee84c97f9f57c6aa))
* **profile:** serialize credential and profile transitions under a store lock ([#690](https://github.com/mlorentedev/ts-bridge/issues/690)) ([1a8d840](https://github.com/mlorentedev/ts-bridge/commit/1a8d84064bebfe1138630c51ded3a4f83b3e8baa))
* **proxy:** log the SOCKS5 tunnel close with duration and byte counts ([#667](https://github.com/mlorentedev/ts-bridge/issues/667)) ([99406dd](https://github.com/mlorentedev/ts-bridge/commit/99406ddab2d528a6f6a87be677ecf0bff8371253))
* **proxy:** publish byte metrics while a tunnel is open ([#586](https://github.com/mlorentedev/ts-bridge/issues/586)) ([49e83dc](https://github.com/mlorentedev/ts-bridge/commit/49e83dc2117187760b93fb0d6c97b4aed21a43c1))
* **run:** stop dialing the control plane after cancellation and close the SOCKS5, lifecycle and log-rotation test gaps ([#662](https://github.com/mlorentedev/ts-bridge/issues/662)) ([da7f8af](https://github.com/mlorentedev/ts-bridge/commit/da7f8afb1e875598e891a62dc876c260cba2ed52))
* **secretfile:** grant the Windows ACL to the token user SID ([#663](https://github.com/mlorentedev/ts-bridge/issues/663)) ([b422402](https://github.com/mlorentedev/ts-bridge/commit/b422402bf9176e44a293f531a16dd4cf287b2cc4))
* **security:** keep the auth key out of loose files and the ssh child ([#635](https://github.com/mlorentedev/ts-bridge/issues/635)) ([2d868df](https://github.com/mlorentedev/ts-bridge/commit/2d868df906972064f27742386d2eb708df211142))
* **security:** validate control URLs in one shared place ([#634](https://github.com/mlorentedev/ts-bridge/issues/634)) ([5dbf559](https://github.com/mlorentedev/ts-bridge/commit/5dbf559f1c09b14ae562d5d6c1c99584b29e8a11))
* **serve:** report a control-plane outage on /health/ready from Self.Online ([#719](https://github.com/mlorentedev/ts-bridge/issues/719)) ([c3a7a58](https://github.com/mlorentedev/ts-bridge/commit/c3a7a5868ecea58010aad6d7775af4a493d7601a))
* **status:** name the health opt-in and print JSON when nothing answers ([#643](https://github.com/mlorentedev/ts-bridge/issues/643)) ([ae74b5e](https://github.com/mlorentedev/ts-bridge/commit/ae74b5e9f5f3ab980a18c11904605a9b1cf7e4ca))


### Documentation

* **host:** mark the host init change as breaking ([#602](https://github.com/mlorentedev/ts-bridge/issues/602)) ([7bf4cd9](https://github.com/mlorentedev/ts-bridge/commit/7bf4cd99d21fe97be4737830e5e2dd6f3d0099e9))

## [1.20.0](https://github.com/mlorentedev/ts-bridge/compare/v1.19.1...v1.20.0) (2026-10-30)


### Features

* **host:** exit 2 from host check when a step is not ok ([#471](https://github.com/mlorentedev/ts-bridge/issues/471)) ([1fe8783](https://github.com/mlorentedev/ts-bridge/commit/1fe87838e50b69a41044458748f4f9bb87d184c5))
* **host:** exit 2 from host setup when a step failed ([#490](https://github.com/mlorentedev/ts-bridge/issues/490)) ([3fdbede](https://github.com/mlorentedev/ts-bridge/commit/3fdbedecacb65736298db0c5e27dec842fda1081))
* **host:** report check as read steps and services, with outcome and fix in the JSON ([#470](https://github.com/mlorentedev/ts-bridge/issues/470)) ([ae81b56](https://github.com/mlorentedev/ts-bridge/commit/ae81b568f98f099ae2f4cab5e310da852678d06b))
* make named profile connections self-contained ([#442](https://github.com/mlorentedev/ts-bridge/issues/442)) ([ed3ccb1](https://github.com/mlorentedev/ts-bridge/commit/ed3ccb104abb61619968c1960f163aebb90e6119))


### Bug Fixes

* **browser:** discover Microsoft Edge on macOS and Linux ([#465](https://github.com/mlorentedev/ts-bridge/issues/465)) ([d010591](https://github.com/mlorentedev/ts-bridge/commit/d01059156d6dbbc6709694fd27addf2474a1ee7a))
* **ci:** close DEPS-312 review findings in the toolchain guard and spec ([#501](https://github.com/mlorentedev/ts-bridge/issues/501)) ([813d898](https://github.com/mlorentedev/ts-bridge/commit/813d8980ee44c7b8f2985d11fa8494aaaacc8699))
* **ci:** match CRLF and tab-labelled conflict markers and archive CI-491 ([#515](https://github.com/mlorentedev/ts-bridge/issues/515)) ([0b86357](https://github.com/mlorentedev/ts-bridge/commit/0b863571b1c38dbd188024e74d9845e0a17279b8)), closes [#491](https://github.com/mlorentedev/ts-bridge/issues/491)
* **ci:** match quoted, sh -c and continued Go commands in the toolchain guard ([#509](https://github.com/mlorentedev/ts-bridge/issues/509)) ([933f904](https://github.com/mlorentedev/ts-bridge/commit/933f9048651bb66c40440f1c1b831ed9171c8d29)), closes [#435](https://github.com/mlorentedev/ts-bridge/issues/435)
* **ci:** name the in-use agent harnesses in the AI-trailer guard and archive CI-358 ([#517](https://github.com/mlorentedev/ts-bridge/issues/517)) ([4849933](https://github.com/mlorentedev/ts-bridge/commit/48499339e48baa033a532cc5e5754598df8ce2cc)), closes [#506](https://github.com/mlorentedev/ts-bridge/issues/506)
* **ci:** run the test suite on macOS and make the log-dir OS an input ([#448](https://github.com/mlorentedev/ts-bridge/issues/448)) ([bfd45ea](https://github.com/mlorentedev/ts-bridge/commit/bfd45ea54e6db7f0e49c2b4d4e9d0cc7678560e7))
* **ci:** scan binary files for conflict markers and close CI-491 round 1 ([#511](https://github.com/mlorentedev/ts-bridge/issues/511)) ([7bc98be](https://github.com/mlorentedev/ts-bridge/commit/7bc98be4a537ad98b4c4a3c77475d9edb746eb4d)), closes [#491](https://github.com/mlorentedev/ts-bridge/issues/491)
* explain named profile target overrides ([#489](https://github.com/mlorentedev/ts-bridge/issues/489)) ([ca636e4](https://github.com/mlorentedev/ts-bridge/commit/ca636e43f0f1510754ac1273c3d972c6bb01f2f6))
* **host:** count a port-less iptables ACCEPT as covering the port ([#493](https://github.com/mlorentedev/ts-bridge/issues/493)) ([fb51dad](https://github.com/mlorentedev/ts-bridge/commit/fb51dade6b65e7130860f77746f80e30e2dc5f14))
* **host:** count a scoped iptables allow as present instead of appending an open rule ([#482](https://github.com/mlorentedev/ts-bridge/issues/482)) ([0b2e63c](https://github.com/mlorentedev/ts-bridge/commit/0b2e63c71e38365fb7d8a5ef08b7e657d65d5b74))
* **host:** make Linux host setup state-first and read the exact firewall rule ([#477](https://github.com/mlorentedev/ts-bridge/issues/477)) ([8fd5fe8](https://github.com/mlorentedev/ts-bridge/commit/8fd5fe855c4411094534945316c64edf1b6c199d))
* **host:** print 'no service declared' instead of 'port 0' in host check text ([#486](https://github.com/mlorentedev/ts-bridge/issues/486)) ([c249243](https://github.com/mlorentedev/ts-bridge/commit/c249243076abdad068bebe072701779e204c2ffe))
* **host:** read xrdp from systemd, not PATH, and count static units as enabled ([#485](https://github.com/mlorentedev/ts-bridge/issues/485)) ([a576980](https://github.com/mlorentedev/ts-bridge/commit/a576980b0585872d12feffe415e09f9885aaf4c4))
* **host:** report an unreadable Linux firewall as unverified, not changed ([#473](https://github.com/mlorentedev/ts-bridge/issues/473)) ([d881023](https://github.com/mlorentedev/ts-bridge/commit/d881023670240b0cc35716e899854e31fec669b4))
* **init:** refuse to replace a .env that cannot be read ([#467](https://github.com/mlorentedev/ts-bridge/issues/467)) ([7c5ead3](https://github.com/mlorentedev/ts-bridge/commit/7c5ead3cdf9bf7e5a9cebf63d86aa7d9187fc7fc))
* route bootstrap control traffic through an HTTP-to-SOCKS proxy for DERP ([#444](https://github.com/mlorentedev/ts-bridge/issues/444)) ([874ca96](https://github.com/mlorentedev/ts-bridge/commit/874ca965febcfbb2fcb7bd04b43769524a07cc82))

## [1.19.1](https://github.com/mlorentedev/ts-bridge/compare/v1.19.0...v1.19.1) (2026-10-09)


### Bug Fixes

* **ci:** close the doc auth-key guard bypasses found in review ([#421](https://github.com/mlorentedev/ts-bridge/issues/421)) ([ec34eb7](https://github.com/mlorentedev/ts-bridge/commit/ec34eb75343a8e1260c88eebf4ab9019840d0ce2))
* **ci:** move PR-Agent off the retired mimo-v2.5 model ([#413](https://github.com/mlorentedev/ts-bridge/issues/413)) ([177355c](https://github.com/mlorentedev/ts-bridge/commit/177355cea297e6026a38234935dec463ae7d10bc))
* **ci:** stop check-lessons reporting indexed lessons as missing ([#419](https://github.com/mlorentedev/ts-bridge/issues/419)) ([b098987](https://github.com/mlorentedev/ts-bridge/commit/b098987cd2dc2179628d282df2a00b826313e129))
* **init:** require a control plane for Headscale keys ([#406](https://github.com/mlorentedev/ts-bridge/issues/406)) ([cea8341](https://github.com/mlorentedev/ts-bridge/commit/cea83417b0d1d398e5f7d03d4f8d816d1912444a))

## [1.19.0](https://github.com/mlorentedev/ts-bridge/compare/v1.18.0...v1.19.0) (2026-10-01)


### Features

* **browser:** open private services through SOCKS5 ([#367](https://github.com/mlorentedev/ts-bridge/issues/367)) ([d9c7ac7](https://github.com/mlorentedev/ts-bridge/commit/d9c7ac7b24bff6c97fe2229a37929cf1b590315c))
* **cli:** add managed credential commands ([#389](https://github.com/mlorentedev/ts-bridge/issues/389)) ([c51c845](https://github.com/mlorentedev/ts-bridge/commit/c51c8452aed5fc1225c5ef771ce5d66875426db9))
* **cli:** expose SOCKS5 mesh routing ([#366](https://github.com/mlorentedev/ts-bridge/issues/366)) ([19bda4d](https://github.com/mlorentedev/ts-bridge/commit/19bda4dd848ba15fb03e31eaf54daf658a7bd6e1))
* **config:** add SSH bootstrap settings ([#373](https://github.com/mlorentedev/ts-bridge/issues/373)) ([1c5a966](https://github.com/mlorentedev/ts-bridge/commit/1c5a966e5104eeb485d1cae7ebe918abbb708582))
* **connect:** bootstrap blocked control planes through SSH ([#374](https://github.com/mlorentedev/ts-bridge/issues/374)) ([54c3b91](https://github.com/mlorentedev/ts-bridge/commit/54c3b91594e08d3167a186a210640163ab9f9dd6))
* **credentials:** add managed per-user store ([#388](https://github.com/mlorentedev/ts-bridge/issues/388)) ([aad5fc9](https://github.com/mlorentedev/ts-bridge/commit/aad5fc96993bb2df9eefd27af7e18049ddd4c323))
* **network:** add OpenSSH bootstrap transport ([#372](https://github.com/mlorentedev/ts-bridge/issues/372)) ([ad9b8b1](https://github.com/mlorentedev/ts-bridge/commit/ad9b8b11ffa448097751c5b6f4cbf60110d05e98))
* **profiles:** resolve managed credentials ([#393](https://github.com/mlorentedev/ts-bridge/issues/393)) ([7cdcb41](https://github.com/mlorentedev/ts-bridge/commit/7cdcb41600f8806c7c928f1c6ac49c1d88927753))
* **proxy:** add SOCKS5 dynamic forwarding ([#365](https://github.com/mlorentedev/ts-bridge/issues/365)) ([745d02a](https://github.com/mlorentedev/ts-bridge/commit/745d02a2f518b223f12145bdd667d3363134e914))


### Bug Fixes

* **ci:** guard inline auth keys in docs ([#400](https://github.com/mlorentedev/ts-bridge/issues/400)) ([ea0a83d](https://github.com/mlorentedev/ts-bridge/commit/ea0a83d5b14de35ae19f79107bc143c46c87c166))
* **ci:** pin the review-guard checkout to the default branch ([#361](https://github.com/mlorentedev/ts-bridge/issues/361)) ([5a845e0](https://github.com/mlorentedev/ts-bridge/commit/5a845e0c6feb8f2ea3f7b68cdf5689670a7a6254))
* **ci:** tolerate null comment body in review guard; lint workflows ([#345](https://github.com/mlorentedev/ts-bridge/issues/345)) ([#359](https://github.com/mlorentedev/ts-bridge/issues/359)) ([4ec2499](https://github.com/mlorentedev/ts-bridge/commit/4ec249922e3bf02edb1ac1885c8527d70787f639))
* **cli:** load profiles before config validation ([#379](https://github.com/mlorentedev/ts-bridge/issues/379)) ([6d5f613](https://github.com/mlorentedev/ts-bridge/commit/6d5f613b272e97492d7aa462c6ab1d1ab6f2d9c8))
* **config:** reject bootstrap health listener collisions ([#377](https://github.com/mlorentedev/ts-bridge/issues/377)) ([9571055](https://github.com/mlorentedev/ts-bridge/commit/95710559561064e1ad708f504b1e2298cf8fdce7))
* **credentials:** close managed credential review gaps ([#405](https://github.com/mlorentedev/ts-bridge/issues/405)) ([822df20](https://github.com/mlorentedev/ts-bridge/commit/822df20e0c52b0a4a0241f8011be75722ef43835))
* **credentials:** reject malformed Tailscale keys ([#398](https://github.com/mlorentedev/ts-bridge/issues/398)) ([b9d37ff](https://github.com/mlorentedev/ts-bridge/commit/b9d37ff75e3ed6c645d5529ed2db808b955e18c7))
* **site:** upgrade undici past websocket DoS ([#380](https://github.com/mlorentedev/ts-bridge/issues/380)) ([8436b61](https://github.com/mlorentedev/ts-bridge/commit/8436b61411e157bd744d169daba588408f95ded1)), closes [#371](https://github.com/mlorentedev/ts-bridge/issues/371)

## [1.18.0](https://github.com/mlorentedev/ts-bridge/compare/v1.17.4...v1.18.0) (2026-09-25)


### Features

* **cli:** add init auth key file support ([#356](https://github.com/mlorentedev/ts-bridge/issues/356)) ([9d256e9](https://github.com/mlorentedev/ts-bridge/commit/9d256e93498c284902d3c7cf9d60108f65a5fb09))

## [1.17.4](https://github.com/mlorentedev/ts-bridge/compare/v1.17.3...v1.17.4) (2026-09-23)


### Bug Fixes

* **ci:** delimit workflow steps by their dash and read permissions values on later lines ([#350](https://github.com/mlorentedev/ts-bridge/issues/350)) ([752b2cb](https://github.com/mlorentedev/ts-bridge/commit/752b2cbd6652fa3cde01b1ecac30ef237e9bf619)), closes [#322](https://github.com/mlorentedev/ts-bridge/issues/322)
* **cli:** point init key-file advice at connect, warn on every inline auth key, archive SEC-212 ([#349](https://github.com/mlorentedev/ts-bridge/issues/349)) ([794b39b](https://github.com/mlorentedev/ts-bridge/commit/794b39b9e50441814a74795c66a62353065e139c))

## [1.17.3](https://github.com/mlorentedev/ts-bridge/compare/v1.17.2...v1.17.3) (2026-09-07)


### Bug Fixes

* **ci:** port the hardened PR-Agent guard from the dotfiles canon ([#323](https://github.com/mlorentedev/ts-bridge/issues/323)) ([6011bfd](https://github.com/mlorentedev/ts-bridge/commit/6011bfd9146c89bb1c1e7e068e88fed5fcfddb28))

## [1.17.2](https://github.com/mlorentedev/ts-bridge/compare/v1.17.1...v1.17.2) (2026-08-08)


### Bug Fixes

* **config:** stop unset --dial-retries/--idle-timeout clobbering env and YAML ([#286](https://github.com/mlorentedev/ts-bridge/issues/286)) ([76af077](https://github.com/mlorentedev/ts-bridge/commit/76af0772921002e0281296e56f5a9c3021f253f3))

## [1.17.1](https://github.com/mlorentedev/ts-bridge/compare/v1.17.0...v1.17.1) (2026-08-05)


### Bug Fixes

* **cli:** route host command logs to stderr so --json stdout stays parseable ([#267](https://github.com/mlorentedev/ts-bridge/issues/267)) ([48589b4](https://github.com/mlorentedev/ts-bridge/commit/48589b42664ccfa9e452f4529653f82eb7da025c))

## [1.17.0](https://github.com/mlorentedev/ts-bridge/compare/v1.16.0...v1.17.0) (2026-07-08)


### Features

* **cli:** structured lifecycle signals — READY/ERROR lines, --quiet (UX-004) ([#237](https://github.com/mlorentedev/ts-bridge/issues/237)) ([39c76ed](https://github.com/mlorentedev/ts-bridge/commit/39c76ed279fec628b5ac0b1bc249ea5ba19e5ee5))


### Bug Fixes

* **cli:** escape backslashes in ERROR signal detail (follow-up to [#237](https://github.com/mlorentedev/ts-bridge/issues/237)) ([#240](https://github.com/mlorentedev/ts-bridge/issues/240)) ([9cf3a18](https://github.com/mlorentedev/ts-bridge/commit/9cf3a181fe87f1bb88740c9d60eef82e29be199d))

## [1.16.0](https://github.com/mlorentedev/ts-bridge/compare/v1.15.6...v1.16.0) (2026-06-26)


### Features

* **config:** named profile model — init --profile, Store.Set, ADR-012 (CFG-001) ([#232](https://github.com/mlorentedev/ts-bridge/issues/232)) ([6046f22](https://github.com/mlorentedev/ts-bridge/commit/6046f22f01ab872f10e792ed380e887693716299))
* **profile:** host-emitted shareable connection profiles (CFG-002) ([#228](https://github.com/mlorentedev/ts-bridge/issues/228)) ([5208d9d](https://github.com/mlorentedev/ts-bridge/commit/5208d9d549254a22b3d58b9a096d1af12684e822))

## [1.15.6](https://github.com/mlorentedev/ts-bridge/compare/v1.15.5...v1.15.6) (2026-06-26)


### Bug Fixes

* **proxy:** exit non-zero and surface dead state when tsnet session dies ([#226](https://github.com/mlorentedev/ts-bridge/issues/226)) ([28715a4](https://github.com/mlorentedev/ts-bridge/commit/28715a42b7c765597021bbcb08f7520abf5da80c)), closes [#208](https://github.com/mlorentedev/ts-bridge/issues/208)

## [1.15.5](https://github.com/mlorentedev/ts-bridge/compare/v1.15.4...v1.15.5) (2026-06-24)


### Bug Fixes

* **config:** route auth keys by prefix; reject hskey- without a control URL ([#209](https://github.com/mlorentedev/ts-bridge/issues/209)) ([#223](https://github.com/mlorentedev/ts-bridge/issues/223)) ([1eee9e5](https://github.com/mlorentedev/ts-bridge/commit/1eee9e5096dd83ee503ca65f9bc64ce77ea31104))

## [1.15.4](https://github.com/mlorentedev/ts-bridge/compare/v1.15.3...v1.15.4) (2026-06-24)


### Bug Fixes

* **cli:** stop dumping the usage block on runtime errors ([#210](https://github.com/mlorentedev/ts-bridge/issues/210)) ([#222](https://github.com/mlorentedev/ts-bridge/issues/222)) ([d3985be](https://github.com/mlorentedev/ts-bridge/commit/d3985bedab77ed6448bd57069c1d63f6bd74758b))
* **config:** default tsnet state to a fixed per-user directory ([#207](https://github.com/mlorentedev/ts-bridge/issues/207)) ([#219](https://github.com/mlorentedev/ts-bridge/issues/219)) ([7aa99f6](https://github.com/mlorentedev/ts-bridge/commit/7aa99f6a8a6ba6e80e9c156bdba508c2f26835a4))

## [1.15.3](https://github.com/mlorentedev/ts-bridge/compare/v1.15.2...v1.15.3) (2026-06-23)


### Bug Fixes

* **cli:** correct -v rewrite that corrupted the --target value ([#215](https://github.com/mlorentedev/ts-bridge/issues/215)) ([373fd3f](https://github.com/mlorentedev/ts-bridge/commit/373fd3fa8c532b77c421a16137c562708dbdc43d))

## [1.15.2](https://github.com/mlorentedev/ts-bridge/compare/v1.15.1...v1.15.2) (2026-06-21)


### Bug Fixes

* **ci:** add PRs to bitácora board ([#196](https://github.com/mlorentedev/ts-bridge/issues/196)) ([c9d3006](https://github.com/mlorentedev/ts-bridge/commit/c9d3006c70828a5dd7d97b6f4da10c79c354ce2d))

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
