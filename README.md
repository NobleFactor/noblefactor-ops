# noblefactor-ops

Team site for NobleFactor projects. **This repository is private.**

- **nf-ops** — Operations tooling: release signing, key management, registry maintenance, and cross-repo automation

## Purpose

This repository contains shared operations tooling for all NobleFactor projects:

- Release signing ceremonies (YubiKey/HSM required)
- SSH key generation and rotation
- INDEX.yaml generation and signing
- Registry maintenance utilities
- Cross-repo automation (token rotation, deploy workflows)

## Building

```bash
make build          # Build nf-ops to bin/
```

## Installing

```bash
make install        # Install to ~/.local/bin (or GOBIN)
```

## Commands

```bash
# Key management (ADR-040)
nf-ops key generate --type release      # Generate release signing key
nf-ops key generate --type ci           # Generate CI signing key
nf-ops key rotate --key release         # Rotate release key with ceremony
nf-ops key list                         # List all managed keys

# Release signing
nf-ops sign pmm <path>                  # Sign a PMM with release key
nf-ops sign index                       # Generate and sign INDEX.yaml
nf-ops sign binary <path>               # Sign a release binary

# Registry maintenance
nf-ops registry reindex                 # Regenerate INDEX.yaml
nf-ops registry verify                  # Verify all PMM signatures
nf-ops registry audit                   # Audit trail report
```

## Security Model

This tool is designed for **ceremony-based operations**:

1. **Human presence required** — Most operations require interactive confirmation
2. **Hardware key support** — Release signing expects YubiKey/HSM
3. **Audit logging** — All operations logged for compliance
4. **No CI secrets** — Release keys never stored in CI; signing happens post-build

See [ADR-040: SSH Key Ceremony](https://github.com/NobleFactor/noblefactor/blob/main/lore/design/adr/040-ssh-key-ceremony.md) for the full key management protocol.

## Project Structure

```
noblefactor-ops/
├── cmd/
│   └── nf-ops/main.go        # nf-ops entry point
├── internal/
│   ├── ceremony/             # Key ceremony workflows
│   ├── signing/              # SSH signing operations
│   ├── index/                # INDEX.yaml generation
│   └── audit/                # Audit logging
├── Makefile
└── go.mod
```

## CI Integration

This repo's CI does **not** perform signing. Instead:

1. `devlore-cli` CI builds unsigned artifacts
2. Human runs ceremony from this repo to sign
3. Signatures uploaded to release

For INDEX.yaml (lower-trust operation), a CI-specific key may be used.

## License

MIT License. See [LICENSE](LICENSE) for details.
