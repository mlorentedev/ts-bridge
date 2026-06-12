---
tags: [spec, tasks, CLI-005]
created: "2026-06-10"
---

# Tasks - CLI-005

## Setup

- [ ] Branch from master: feat/yaml-config
- [ ] Depends on CLI-001, CLI-002

## Implementation

- [ ] go get gopkg.in/yaml.v3
- [ ] Create internal/config/yaml.go with YAML struct and loader
- [ ] Implement LoadYAMLConfig(path string) (partial Config, error)
- [ ] Implement merge logic: flags > env > yaml > defaults
- [ ] Wire --config flag in root command
- [ ] Reject auth key in YAML with clear error
- [ ] Warn on unknown YAML fields
- [ ] Validate all YAML values

## Testing

- [ ] Write tests: valid YAML, invalid YAML, unknown fields, auth key rejection
- [ ] go test ./... green
- [ ] golangci-lint run clean

## Closing

- [ ] PR < 250 lines diff (excluding tests)
- [ ] PR references issue #52