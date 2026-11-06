# Windows guest scenario (QA-606). Runs elevated on a fresh Windows 11 Pro
# Arm64 guest, with C:\tsb\tsb-new.exe (candidate) and C:\tsb\tsb-old.exe
# (previous release) copied in. Exercises the real PowerShell reads and
# writes (#601). Prints one line per case: PASS, FAIL, XFAIL #N or XPASS #N.
$ErrorActionPreference = 'Continue'
$B = 'C:\tsb\tsb-new.exe'
$OLD = 'C:\tsb\tsb-old.exe'
$C = 'C:\ProgramData\ts-bridge\host.yaml'
$W = 'C:\tsb\e2e'
New-Item -ItemType Directory -Force $W | Out-Null
Set-Location $W
$TS = 'HKLM:\System\CurrentControlSet\Control\Terminal Server'

# Result NAME OK [ISSUE]: ISSUE marks a known open bug, reported XFAIL while
# failing and XPASS once fixed (AC5).
function Result($name, [bool]$ok, $issue) {
    if ($issue) { if ($ok) { "XPASS #$issue $name" } else { "XFAIL #$issue $name" } }
    elseif ($ok) { "PASS $name" } else { "FAIL $name" }
}
# Case NAME [ISSUE] { block }: the case holds when the block returns $true.
function Case($name, $issue, [scriptblock]$test) {
    if ($issue -is [scriptblock]) { $test = $issue; $issue = $null }
    try { $ok = [bool](& $test) } catch { $ok = $false }
    Result $name $ok $issue
}
# Tsb runs the candidate, keeping its combined output in $script:Out and its
# exit code in $script:Code.
function Tsb {
    $script:Out = (& $B @args 2>&1 | Out-String)
    $script:Code = $LASTEXITCODE
}
# TsbBounded is Tsb with a 20 s ceiling, as linux.sh's `timeout 20`: a serve
# that wrongly starts is killed and fails its case (code 124) instead of
# hanging the run.
function TsbBounded {
    $p = Start-Process -FilePath $B -ArgumentList $args -NoNewWindow -PassThru `
        -RedirectStandardOutput "$W\bounded.out" -RedirectStandardError "$W\bounded.err"
    $null = $p.Handle # without a cached handle ExitCode reads empty after exit
    if ($p.WaitForExit(20000)) { $script:Code = $p.ExitCode } else { $p.Kill(); $p.WaitForExit(); $script:Code = 124 }
    $script:Out = (Get-Content "$W\bounded.out", "$W\bounded.err" -Raw -ErrorAction SilentlyContinue) -join ''
}
function Json($cmd, $file) { & $B host $cmd --config $file --json 2>$null | Out-String | ConvertFrom-Json }
function Steps($j, $prefix) { @($j.steps | Where-Object { $_.name -like "$prefix*" }) }
function Rule($name) { Get-NetFirewallRule -DisplayName $name -ErrorAction SilentlyContinue }
function AcZero {
    foreach ($q in @('SUB_SLEEP STANDBYIDLE', 'SUB_SLEEP HIBERNATEIDLE', 'SUB_VIDEO VIDEOIDLE')) {
        $a = $q.Split(' ')
        $hex = ([regex]::Matches((powercfg /query SCHEME_CURRENT $a[0] $a[1] | Out-String), '0x[0-9a-fA-F]{8}'))
        if ($hex.Count -lt 2 -or $hex[$hex.Count - 2].Value -ne '0x00000000') { return $false }
    }
    $true
}
function Yaml($path, $text) { Set-Content -Path $path -Value $text -Encoding ascii }

Case 'version prints' { Tsb version; $Code -eq 0 }
Case 'init: --config required' { Tsb host init; $Code -eq 1 -and $Out -match '--config is required' }
Case 'init: writes the YAML with firewall and power' {
    Tsb host init --config $C --port 3389 --force
    $y = Get-Content $C -Raw
    $Code -eq 0 -and $y -match 'kind: rdp' -and $y -match 'rule: Tailscale-RDP-Ingress' -and $y -match 'prevent_sleep: true'
}
Case 'init: existing file kept without --force' {
    $before = Get-Content $C -Raw; Tsb host init --config $C --port 3390
    $Code -ne 0 -and (Get-Content $C -Raw) -eq $before
}
Case 'init: directory refused even with --force' {
    New-Item -ItemType Directory -Force "$W\dir.yaml" | Out-Null; Tsb host init --config "$W\dir.yaml" --force; $Code -ne 0
}
Case 'init: invalid firewall rule name refused' {
    Tsb host init --config "$W\bad.yaml" --firewall-rule 'bad rule'; $Code -ne 0 -and -not (Test-Path "$W\bad.yaml")
}

# Fresh guest: RDP denied, no rule, sleep on, no Tailscale.
Case 'fresh guest denies RDP' { (Get-ItemProperty $TS).fDenyTSConnections -eq 1 }
Tsb host setup --config $C
Case 'setup not converged: no HOST READY banner' { $Out -notmatch 'HOST READY' }
Case 'setup without IP: no empty TS_TARGET' { $Out -notmatch 'TS_TARGET=:' }
Case 'setup without Tailscale: unattended step says not installed' { $Out -match 'Tailscale CLI not found' }
$j2 = Json setup $C
# A negative needs a document: with no steps at all, nothing was verified.
Case 'setup #2: nothing changed' { @($j2.steps).Count -gt 0 -and -not @($j2.steps | Where-Object { $_.outcome -eq 'changed' }) }
Case 'setup: no Tailscale adapter is manual' { (Steps $j2 'Network profile')[0].outcome -eq 'manual' }
Case 'RDP enabled, TermService running, listening on 3389' {
    (Get-ItemProperty $TS).fDenyTSConnections -eq 0 -and (Get-Service TermService).Status -eq 'Running' -and
    @(Get-NetTCPConnection -LocalPort 3389 -State Listen -ErrorAction SilentlyContinue).Count -gt 0
}
Case 'firewall rule: one, enabled, inbound TCP 3389' {
    $r = @(Rule 'Tailscale-RDP-Ingress'); $f = $r | Get-NetFirewallPortFilter
    $r.Count -eq 1 -and $r[0].Enabled -eq 'True' -and $r[0].Direction -eq 'Inbound' -and $f.Protocol -eq 'TCP' -and $f.LocalPort -eq '3389'
}
Case 'firewall rule: scoped to the tailnet addresses' {
    $ra = @((Rule 'Tailscale-RDP-Ingress' | Get-NetFirewallAddressFilter).RemoteAddress)
    # Windows may print the IPv4 range with a dotted mask; compare the normalized set.
    $ra.Count -eq 2 -and ($ra | Where-Object { $_ -match '^100\.64\.0\.0/(10|255\.192\.0\.0)$' }) -and ($ra | Where-Object { $_ -ieq 'fd7a:115c:a1e0::/48' })
}
# The built-in group's rules allow any address. setup does not enable or disable it; check reports
# it (firewall_ok false) until the operator disables it (#685, folded into #631).
function RdGroupOn { @(Get-NetFirewallRule -DisplayGroup 'Remote Desktop' -ErrorAction SilentlyContinue | Where-Object { $_.Enabled -eq 'True' }).Count }
Case 'built-in Remote Desktop group enabled: check reports it, setup leaves it on' {
    Enable-NetFirewallRule -DisplayGroup 'Remote Desktop'
    $seen = -not (Json check $C).services[0].firewall_ok
    Json setup $C | Out-Null
    $seen -and (RdGroupOn) -gt 0 -and -not (Json check $C).services[0].firewall_ok
}
Case 'built-in Remote Desktop group disabled by the operator: firewall ok again' {
    Disable-NetFirewallRule -DisplayGroup 'Remote Desktop'
    (Json check $C).services[0].firewall_ok
}
Case 'power: AC sleep, hibernate and monitor timeouts are 0' { AcZero }
Case 'check --json: rdp 3389 enabled, firewall ok' {
    $s = (Json check $C).services; $s.Count -eq 1 -and $s[0].kind -eq 'rdp' -and $s[0].port -eq 3389 -and $s[0].enabled -and $s[0].firewall_ok
}

# Drift: each change is seen by check and repaired by setup. A negative is
# asserted only on a document that still lists the service, so a check that
# drops it cannot pass as drift seen.
$drifts = [ordered]@{
    'rule switched to UDP' = { Rule 'Tailscale-RDP-Ingress' | Set-NetFirewallRule -Protocol UDP }
    'rule disabled'        = { Rule 'Tailscale-RDP-Ingress' | Disable-NetFirewallRule }
    'rule deleted'         = { Rule 'Tailscale-RDP-Ingress' | Remove-NetFirewallRule }
    'rule open to any address (legacy)' = { Rule 'Tailscale-RDP-Ingress' | Get-NetFirewallAddressFilter | Set-NetFirewallAddressFilter -RemoteAddress Any }
}
foreach ($d in $drifts.Keys) {
    & $drifts[$d]
    Case "drift: $d seen by check" { $s = (Json check $C).services; $s.Count -eq 1 -and -not $s[0].firewall_ok }
    Json setup $C | Out-Null
    Case "drift: $d repaired by setup" { (Json check $C).services[0].firewall_ok }
}
Set-ItemProperty $TS -Name fDenyTSConnections -Value 1
Case 'drift: RDP denied seen by check' { $s = (Json check $C).services; $s.Count -eq 1 -and -not $s[0].enabled }
Json setup $C | Out-Null
Case 'drift: RDP denied repaired by setup' { (Get-ItemProperty $TS).fDenyTSConnections -eq 0 }
powercfg /change standby-timeout-ac 30
Case 'drift: power reset seen by check' { (Steps (Json check $C) 'Power')[0].outcome -eq 'changed' }
Case 'drift: power reset is changed, then ok' {
    (Steps (Json setup $C) 'Power')[0].outcome -eq 'changed' -and (Steps (Json setup $C) 'Power')[0].outcome -eq 'ok' -and (AcZero)
}

Yaml "$W\nosleep.yaml" "version: 1`nhost:`n  services:`n    - kind: rdp`n      port: 3389`n  power:`n    prevent_sleep: false"
$jn = Json setup "$W\nosleep.yaml"
Case 'prevent_sleep false: no power step' { @($jn.steps).Count -gt 0 -and -not (Steps $jn 'Power') }
Yaml "$W\custom.yaml" "version: 1`nhost:`n  services:`n    - kind: rdp`n      port: 3389`n  firewall:`n    rule: Custom-E2E"
Case 'custom rule name from the file is created' { Json setup "$W\custom.yaml" | Out-Null; @(Rule 'Custom-E2E').Count -eq 1 }
Yaml "$W\ssh.yaml" "version: 1`nhost:`n  services:`n    - kind: ssh`n      port: 22"
Case 'ssh service refused, citing #521' { Tsb host setup --config "$W\ssh.yaml"; $Code -eq 1 -and $Out -match '#521' }
Yaml "$W\unk.yaml" "version: 1`nhost:`n  services:`n    - kind: rdp`n      port: 3389`n  colour: blue"
Case 'unknown key warned by name, check still runs' { Tsb host check --config "$W\unk.yaml"; $Out -match 'colour' -and $Out -match 'HOST CHECK' }
Yaml "$W\v2.yaml" "version: 2`nhost: {}"
Case 'unsupported version refused' { Tsb host check --config "$W\v2.yaml"; $Code -eq 1 -and $Out -match 'version' }
Yaml "$W\p.yaml" "version: 1`nhost:`n  services:`n    - kind: rdp`n      port: 3390"
# Windows does not move the RDP listener: a declared port it does not listen on is manual (#557).
Case 'declared port 3390: check prints its steps as JSON' { @((Json check "$W\p.yaml").steps).Count -gt 0 }
Case 'check: declared port differs from the registry port' {
    @((Json check "$W\p.yaml").steps | Where-Object { $_.name -eq 'RDP configuration' -and $_.outcome -eq 'manual' -and $_.message -match '3390' -and $_.message -match '3389' }).Count -eq 1
}
Case 'check: a declared port mismatch exits 2' { Tsb host check --config "$W\p.yaml"; $Code -eq 2 }

# Upgrade: the previous release's .env still applies from its directory.
New-Item -ItemType Directory -Force "$W\up" | Out-Null; Set-Location "$W\up"; Remove-Item .env -ErrorAction SilentlyContinue
& $OLD host init --firewall-rule Old-E2E-Rule *> $null
Case 'upgrade: previous init wrote TS_HOST_FIREWALL_RULE' { (Get-Content .env -Raw) -match 'TS_HOST_FIREWALL_RULE=Old-E2E-Rule' }
Case 'upgrade: .env beats the file in setup' { Json setup $C | Out-Null; @(Rule 'Old-E2E-Rule').Count -eq 1 }
Case 'upgrade: init without --config fails' { Tsb host init; $Code -ne 0 }
Set-Location $W

# host serve preflight refusals; nothing may start or reach the network.
function Mesh($state) { "version: 1`nhost:`n  control_url: https://hs.invalid`n  mesh:`n    hostname: e2e`n    state_dir: $state`n    auth_key_file: C:\ProgramData\ts-bridge\authkey`n  services:`n    - kind: rdp`n      port: 3389" }
Yaml "$W\nomesh.yaml" "version: 1`nhost:`n  control_url: https://hs.invalid`n  services:`n    - kind: rdp`n      port: 3389"
Case 'serve: no host.mesh refused' { TsbBounded host serve --config "$W\nomesh.yaml"; $Code -eq 1 -and $Out -match 'host\.mesh' }
# An absent state_dir is created protected, so the loose one must exist first.
New-Item -ItemType Directory -Force "$W\state" | Out-Null; icacls "$W\state" /grant 'Users:(OI)(CI)F' | Out-Null
Yaml "$W\loose.yaml" (Mesh "$W\state")
Case 'serve: state_dir other users can reach refused' { TsbBounded host serve --config "$W\loose.yaml"; $Code -eq 1 -and $Out -match 'state_dir' }

Case 'client: import a tsb:// descriptor' { Tsb import e2e 'tsb://host-e2e:3389?cp=saas'; $Code -eq 0 }
Case 'client: malformed descriptor refused' { Tsb import bad 'http://nope'; $Code -ne 0 }
Case 'client: status with no bridge prints running false, hint on stderr' {
    $o = (& $B status --json --addr 127.0.0.1:1 2>"$W\err" | Out-String); $c = $LASTEXITCODE
    $j = $o | ConvertFrom-Json
    $c -eq 0 -and $j.running -eq $false -and $j.addr -eq '127.0.0.1:1' -and (Get-Content "$W\err" -Raw) -match 'not running'
}

# OwnerOnly PATH: the ACL is protected and grants only the token's user SID (#632).
function OwnerOnly($path) {
    if (-not $path) { return $false } # Get-Acl without a path reads the current directory
    $acl = Get-Acl -LiteralPath $path
    $me = [Security.Principal.WindowsIdentity]::GetCurrent().User
    $rules = @($acl.GetAccessRules($true, $true, [Security.Principal.SecurityIdentifier]))
    $acl.AreAccessRulesProtected -and $rules.Count -eq 1 -and $rules[0].IdentityReference -eq $me
}
# USERNAME names another account, as a renamed or domain user would see it.
$user = $env:USERNAME; $env:USERNAME = 'Guest'
'tskey-auth-e2e-notreal' | & $B auth set --stdin --profile e2e 2>&1 | Out-Null; $authCode = $LASTEXITCODE
$env:USERNAME = $user
$key = Get-ChildItem "$env:LOCALAPPDATA\ts-bridge" -Recurse -Filter *.key -ErrorAction SilentlyContinue | Select-Object -First 1
Case 'auth set: stores the credential' { $authCode -eq 0 -and $key }
Case 'auth set: key file grants only the token user' { $key -and (OwnerOnly $key.FullName) }
Case 'auth set: store directory grants only the token user' { $key -and (OwnerOnly $key.DirectoryName) }
exit 0
