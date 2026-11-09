# ethkit build spec

This spec answers one question: what do I build for project 1, in what order, and how do I know each piece works? It covers two small packages (address checksums and unit conversion), the command-line entry point, and their tests. Network calls, `big.Int` arithmetic and the `flag` package are out of scope, since projects 2 and 3 introduce them.

## Project layout

The layout splits the logic into packages so it can be tested on its own and imported by later projects.

```
ethkit/
├── go.mod
├── main.go
├── address/
│   ├── checksum.go
│   └── checksum_test.go
└── units/
    ├── units.go
    └── units_test.go
```

Each folder is a package, roughly like a Solidity library you import, and `main.go` is the only file that reads arguments or prints. Keeping input and output in `main.go` means everything in `address` and `units` is a pure function (input in, value out), which is the easiest shape to test. The tradeoff is ceremony, because three packages is more structure than a tool this size strictly needs, but project 2 will import `address` directly and the split pays off there.

## Step 1: start the module

This step creates the module, the unit Go uses to name and version your code.

Run `go mod init github.com/i3arba/ethkit` in the repo root. The resulting `go.mod` plays the role of `foundry.toml` plus a package name, and the module path is also the import prefix, so your own packages become `github.com/i3arba/ethkit/address` and `github.com/i3arba/ethkit/units`. Add a `main.go` with `package main` and an empty `func main() {}`.

Done when `go build` succeeds and produces an `ethkit` binary that does nothing.

## Step 2: EIP-55 checksum

This step builds the `address` package, which turns any valid address into its checksummed form and checks whether an address already has it.

EIP-55 hides a checksum in letter case. You take the 40 hex characters after `0x` in lowercase, hash that text (the ASCII characters, not the decoded bytes) with Keccak-256, and walk the address one character at a time. A letter at position `i` becomes uppercase when hex digit `i` of the hash is `8` or higher, and digits stay as they are.

Watch the hash function, because this is the classic trap. Go's standard library `crypto/sha3` implements NIST SHA3-256, which pads differently from Ethereum's `keccak256` and gives different output. Use `sha3.NewLegacyKeccak256()` from `golang.org/x/crypto/sha3` instead (add it with `go get golang.org/x/crypto`). Solidity's `keccak256` is the legacy version too.

The package exports two functions.

```go
// ToChecksum returns addr in EIP-55 mixed case.
func ToChecksum(addr string) (string, error)

// IsChecksummed reports whether addr already has the correct EIP-55 case.
func IsChecksummed(addr string) (bool, error)
```

A valid input starts with `0x` followed by exactly 40 hex characters in any case. Anything else returns an error, which is Go's version of a revert: the function hands back a zero value plus the error, and the caller decides what to do with it.

For reading hash digit `i`, the simplest route is `hex.EncodeToString(hash)` and comparing character `i` against `'8'`. The lower-level route reads byte `i/2` and takes the high half (`b >> 4`) for even `i` or the low half (`b & 0x0f`) for odd `i`. The hex string is easier to read and the byte version skips building an extra string, which does not matter at this size, so start with the string.

New Go in this step: exported names (a capital first letter makes a name public, like `public` in Solidity), multiple return values, `errors.New` and `fmt.Errorf`, the `strings` and `encoding/hex` packages, and the difference between a `string` and a `[]byte`.

### Tests

The tests check the function against the official examples, then fuzz it for properties that must hold on any input.

Write one table-driven test, a slice of cases looped with `t.Run` so each case gets its own name. These vectors come from the EIP-55 document itself. Feed each one in lowercase and expect it back exactly as written.

| Expected output |
|---|
| `0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed` |
| `0xfB6916095ca1df60bB79Ce92cE3Ea74c37c5d359` |
| `0xdbF03B407c01E7cD3CBea99509d93f8DDDC8C6FB` |
| `0xD1220A0cf47c7B9Be7A2E6BA89F429762e7b9aDb` |
| `0x52908400098527886E0F7030069857D2E4169EE7` |
| `0xde709f2102306220921060314715629080e2fb77` |

The last two look like they carry no checksum, but their correct checksum happens to be all uppercase and all lowercase. They catch a bug where you skip addresses that "look plain".

Add error cases for an empty string, an address without `0x`, one that is too short (`0x1234`), and one with 40 characters that are not all hex (`0x` plus 40 `z`). For `IsChecksummed`, the first vector should return `true`, and the same address with one letter's case flipped (`0x5aaeb6053F3E94C9b9A09f33669435E7Ef1BeAed`) should return `false`.

Then add `FuzzToChecksum`, which works like a Foundry `testFuzz_` function. Go hands your fuzz function random `[]byte` values of any length, so return early when there are fewer than 20 bytes and use the first 20. Build the lowercase address with `"0x" + hex.EncodeToString(b[:20])` and assert three properties.

- `ToChecksum` returns no error.
- Lowercasing the result gives back the input.
- `ToChecksum` on its own result changes nothing, and `IsChecksummed` on it returns `true`.

Run it with `go test -fuzz=FuzzToChecksum -fuzztime=30s ./address`. When the fuzzer finds a failing input it saves it under `testdata/fuzz/`, and plain `go test` replays it from then on, so commit that file as a regression test.

Done when `go test ./address` passes and 30 seconds of fuzzing finds nothing.

## Step 3: unit converter

This step builds the `units` package, which moves an amount between wei, gwei and ether without ever losing precision.

```go
// Convert moves amount between units, e.g. Convert("1.5", "ether", "gwei") returns "1500000000".
func Convert(amount, from, to string) (string, error)
```

Keep the units in a `map[string]int` of decimals: `wei` is 0, `gwei` is 9, `ether` is 18. A Go map works like a Solidity mapping, except a missing key can be detected with `d, ok := decimals[unit]`, where `ok` is `false` for an unknown unit.

Floats are not an option here, for the same reason Solidity has none. A `float64` holds about 16 significant digits and cannot store `0.1` exactly, while 1 ether in wei is already 19 digits. A `uint64` overflows at about 18.4 ether in wei. So the amount stays a decimal string and you move its decimal point: the shift is `decimals(from) - decimals(to)`, and a positive shift moves the point right. The tradeoff is that strings can convert but cannot add or compare amounts. That needs `big.Int`, which arrives in project 3.

The input and output follow these rules.

- An amount is digits, optionally with one `.` that has digits on both sides. Reject `""`, `"abc"`, `"-1"`, `"1.2.3"`, `".5"` and `"5."`.
- An unknown unit is an error.
- The result has no leading zeros (except a single `0` before the point), no trailing zeros after the point, and no `.` when nothing follows it.
- A result in wei with a fractional part is an error, since nothing is smaller than 1 wei.

Useful tools are `strings.Cut(amount, ".")` to split the whole and fractional parts, `strings.Repeat("0", n)` for padding, and `strings.TrimLeft` and `strings.TrimRight` for zeros. To check for digits, loop over the bytes and test `c >= '0' && c <= '9'`. Put that check in a small helper with a lowercase name, which keeps it private to the package (like `internal` in Solidity).

New Go in this step: maps and the comma-ok lookup, more of the `strings` package, unexported helpers, and `for` loops over a string.

### Tests

The tests cover each direction of the shift plus the formatting rules.

| amount | from | to | want |
|---|---|---|---|
| `1` | ether | wei | `1000000000000000000` |
| `1.5` | ether | gwei | `1500000000` |
| `1.5` | gwei | wei | `1500000000` |
| `1000000000` | gwei | ether | `1` |
| `21000` | gwei | ether | `0.000021` |
| `1` | wei | ether | `0.000000000000000001` |
| `0` | ether | wei | `0` |
| `007` | gwei | gwei | `7` |
| `2.50` | ether | ether | `2.5` |

Add error cases for every rejected amount in the rules above, for an unknown unit such as `banana`, and for `0.0000000001` gwei to wei (a tenth of a wei).

Done when `go test ./units` passes.

## Step 4: command line

This step wires both packages into `main.go`, the only place that reads input and prints output.

`os.Args` is a slice of strings where index 0 is the program name and index 1 is the subcommand. A `switch` on `os.Args[1]` routes to `checksum <address>` or `convert <amount> <from> <to>`. Go's `switch` does not fall through to the next case, unlike C. When something fails, print the error with `fmt.Fprintln(os.Stderr, err)` and stop with `os.Exit(1)`. Running with no subcommand, or with the wrong number of arguments, prints a short usage line and exits with 1 too.

Reading `os.Args` by hand is fine for two subcommands with fixed arguments. The `flag` package becomes worth it once options appear (a `--json` output switch, say), so leave it for later.

Main is glue code, so check it by hand and do not write tests for it.

```bash
go run . checksum 0x5aaeb6053f3e94c9b9a09f33669435e7ef1beaed
go run . convert 1.5 ether gwei
go run . convert 1 banana wei
echo $?
```

The third command should print an error, and `echo $?` should print `1`.

## Done for the whole project

The project is finished when all of these hold.

- `go test ./...` passes.
- `go test -fuzz=FuzzToChecksum -fuzztime=30s ./address` finds nothing.
- `go vet ./...` reports nothing and `gofmt -l .` prints no file names.
- Both subcommands work, and bad input fails with a message and exit code 1.

A commit per step keeps the history readable and makes it easy to see how the code grew.
