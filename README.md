# ethkit

ethkit is a small command-line tool for two everyday Ethereum chores: checking an address's EIP-55 checksum and converting amounts between wei, gwei and ether. It runs offline and never talks to a chain.

This is a learning project, the first of seven in a series where I learn Go by building blockchain tooling from a blank file. Each project reuses the last one and adds a little. The build plan lives in [SPEC.md](SPEC.md).

## Usage

The tool has one subcommand per job.

```bash
ethkit checksum 0x5aaeb6053f3e94c9b9a09f33669435e7ef1beaed
# 0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed

ethkit convert 1.5 ether gwei
# 1500000000
```

## Build and test

Everything runs with the standard Go toolchain, with no other setup.

```bash
go build
go test ./...
go test -fuzz=FuzzToChecksum ./address
```
