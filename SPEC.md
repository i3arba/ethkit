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

Done when `go build` succeeds and produces an `ethkit` binary that does nothing. Go needs no semicolons at the end of lines, and `gofmt -w .` rewrites every file into the standard Go format, so run it whenever you finish editing.

## Step 2: EIP-55 checksum

This step builds the `address` package in nine small tasks, and you run the tests after most of them, so you always know whether the last thing you wrote works.

EIP-55 hides a checksum in letter case. You take the 40 hex characters after `0x` in lowercase, hash that text (the ASCII characters, not the decoded bytes) with Keccak-256, and walk the address one character at a time. A letter at position `i` becomes uppercase when character `i` of the hash, written in hex, is `8` or higher, and digits stay as they are. Your `ToChecksum` function will do this in three parts (check the input, hash it, apply the case), and tasks 2.3, 2.5 and 2.6 build one part each.

### 2.1 Create the package

This task creates the folder and file the package lives in.

Create a folder `address` in the repo root and a file `address/checksum.go` inside it. Its first line is `package address`. In Go a folder is a package, every file in the folder starts with the same `package` line, and by convention that name matches the folder.

### 2.2 Write the function shell

This task adds a function that compiles but does nothing yet, so you have something to build on.

```go
// ToChecksum returns addr in EIP-55 mixed case.
func ToChecksum(addr string) (string, error) {
	return "", nil
}
```

The capital `T` exports the function, so `main.go` can call it later as `address.ToChecksum` (a lowercase name stays private to the package). `(string, error)` means it returns two values, and `nil` as the error means "no error". An error is Go's version of a revert, except nothing unwinds by itself: the function hands back a zero value plus the error, and the caller decides what to do.

Run `go build ./...`, which prints nothing when the code compiles.

### 2.3 Reject bad input

This task fills in the first part of the body, the input checks.

A valid address passes three checks, in this order.

1. It starts with `0x`. Use `strings.HasPrefix(addr, "0x")`.
2. After `0x` it has exactly 40 characters. Slice off the prefix with `body := addr[2:]`, like `data[2:]` on calldata in Solidity, and check `len(body)`. The slice is safe only because check 1 already proved the string is at least 2 long.
3. Those 40 characters are all hex. `hex.DecodeString(body)` returns an error when they are not. You don't need the decoded bytes, so write `_, err := hex.DecodeString(body)` and look only at `err`.

When a check fails, return `"", errors.New("some short message")`. When all three pass, lowercase the body with `lower := strings.ToLower(body)` and, for now, return `"0x" + lower, nil`.

Each package you use goes in an import block under the `package` line.

```go
import (
	"encoding/hex"
	"errors"
	"strings"
)
```

Go refuses to compile when an import is unused, so add each one when you first use it (the VS Code Go extension does this on save).

### 2.4 Write your first test

This task writes a test for what the function does so far, which is rejecting bad input.

Create `address/checksum_test.go`, also starting with `package address`. Files ending in `_test.go` are compiled only by `go test`, so they never end up in your binary.

```go
package address

import (
	"strings"
	"testing"
)

func TestToChecksumErrors(t *testing.T) {
	cases := []struct {
		name string
		addr string
	}{
		{"empty", ""},
		{"no prefix", "5aaeb6053f3e94c9b9a09f33669435e7ef1beaed"},
		{"too short", "0x1234"},
		{"not hex", "0x" + strings.Repeat("z", 40)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ToChecksum(tc.addr)
			if err == nil {
				t.Errorf("ToChecksum(%q) returned no error", tc.addr)
			}
		})
	}
}
```

`cases` is the table, a list of small unnamed structs with one row per case. `for _, tc := range cases` walks it (the `_` throws away the row number), and `t.Run` names each row so a failure tells you which one broke. `t.Errorf` marks the test failed but keeps going.

Run `go test ./address` and expect `ok`. Then prove the test can fail: delete the length check, run again, watch "too short" fail, and put the check back.

### 2.5 Hash the address

This task adds the second part of the body, the Keccak hash of the lowercase text.

Add `"golang.org/x/crypto/sha3"` to the imports and these three lines after `lower`.

```go
h := sha3.NewLegacyKeccak256()
h.Write([]byte(lower))
hashHex := hex.EncodeToString(h.Sum(nil))
```

`h` is a hasher you feed bytes into. `[]byte(lower)` turns the text into its bytes, like `bytes(str)` in Solidity, and `h.Sum(nil)` returns the 32-byte hash, which `hex.EncodeToString` writes out as 64 hex characters. You only need the first 40, one per address character. Run `go mod tidy` afterwards, and the `// indirect` next to `golang.org/x/crypto` in `go.mod` disappears, because your code now imports it directly.

Watch the hash function, because this is the classic trap. Go's standard library `crypto/sha3` implements NIST SHA3-256, which pads differently from Ethereum's `keccak256` and gives different output. The `Legacy` in `NewLegacyKeccak256` is the original Keccak that Ethereum (and Solidity's `keccak256`) uses.

### 2.6 Apply the case

This task adds the third part of the body, which builds the checksummed result.

Go strings are read-only, so copy the text into bytes you can change with `out := []byte(lower)`. Loop over it with `for i := 0; i < len(out); i++`, and inside the loop uppercase `out[i]` when both of these hold.

- `out[i]` is a letter, meaning `out[i] >= 'a' && out[i] <= 'f'`.
- The matching hash character is `8` or higher, meaning `hashHex[i] >= '8'`.

Single quotes hold one character (a byte), double quotes hold a string, and comparing bytes compares their ASCII numbers. That gives you a cheap way to uppercase: `'a'` is 97 and `'A'` is 65, so `out[i] = out[i] - 32` turns any lowercase letter into its capital. After the loop, return `"0x" + string(out), nil`.

### 2.7 Test against the official examples

This task checks your function against the examples published in EIP-55 itself.

Add a second test to `checksum_test.go`. Each row holds only the expected output, and the input is that same address lowercased.

```go
func TestToChecksum(t *testing.T) {
	cases := []string{
		"0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed",
		// add the other five rows from the table below
	}
	for _, want := range cases {
		t.Run(want, func(t *testing.T) {
			got, err := ToChecksum(strings.ToLower(want))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != want {
				t.Errorf("got %s, want %s", got, want)
			}
		})
	}
}
```

`t.Fatalf` stops the current row on the spot, since comparing a result makes no sense once there is an error.

| Expected output |
|---|
| `0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed` |
| `0xfB6916095ca1df60bB79Ce92cE3Ea74c37c5d359` |
| `0xdbF03B407c01E7cD3CBea99509d93f8DDDC8C6FB` |
| `0xD1220A0cf47c7B9Be7A2E6BA89F429762e7b9aDb` |
| `0x52908400098527886E0F7030069857D2E4169EE7` |
| `0xde709f2102306220921060314715629080e2fb77` |

The last two look like they carry no checksum, but their correct checksum happens to be all uppercase and all lowercase. They catch a bug where you skip addresses that "look plain".

Run `go test ./address`. If a row fails, compare `got` and `want` letter by letter, because the first difference shows where your loop went wrong.

### 2.8 Check an existing checksum

This task adds the second exported function, which mostly reuses the first.

```go
// IsChecksummed reports whether addr already has the correct EIP-55 case.
func IsChecksummed(addr string) (bool, error)
```

Call `ToChecksum(addr)`. If it returns an error, return `false` and that error, otherwise return whether the result equals `addr`. Test it in the same table style: `0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed` should give `true`, and `0x5aaeb6053F3E94C9b9A09f33669435E7Ef1BeAed` (first `A` lowercased) should give `false`.

### 2.9 Fuzz it

This task adds a fuzz test, which works like a Foundry `testFuzz_` function: Go feeds it random inputs and you assert properties that must always hold.

```go
func FuzzToChecksum(f *testing.F) {
	f.Add([]byte("seed input at least 20 bytes long"))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) < 20 {
			return
		}
		lower := "0x" + hex.EncodeToString(b[:20])
		// your checks go here
	})
}
```

`f.Add` gives the fuzzer a starting input, and Go then mutates it into random byte slices of any length. Returning early skips the ones too short to make an address, like `vm.assume` in Foundry. Add `"encoding/hex"` to the test imports and write three checks in place of the comment.

- `ToChecksum(lower)` returns no error.
- Lowercasing the result gives back `lower`.
- `ToChecksum` on its own result returns the same string, and `IsChecksummed` on it returns `true`.

Run it with `go test -fuzz=FuzzToChecksum -fuzztime=30s ./address`. When the fuzzer finds a failing input it saves it under `testdata/fuzz/`, and plain `go test` replays it from then on, so commit that file as a regression test.

Done when `go test ./address` passes and 30 seconds of fuzzing finds nothing.

## Step 3: unit converter

This step builds the `units` package in eight small tasks, which together convert an amount between wei, gwei and ether without ever losing precision.

Floats are not an option here, for the same reason Solidity has none. A `float64` holds about 16 significant digits and cannot store `0.1` exactly, while 1 ether in wei is already 19 digits, and a `uint64` overflows at about 18.4 ether in wei. So the amount stays a decimal string and you move its decimal point, the way you would on paper. The tradeoff is that strings can convert amounts but cannot add or compare them. That needs `big.Int`, which arrives in project 3.

Your `Convert` function works in four parts (check the units, check the amount, move the point, clean up the result), and tasks 3.2, 3.4, 3.5 and 3.6 build one part each.

### 3.1 Create the package

This task creates the folder and file, the same way as task 2.1.

Create a folder `units` and a file `units/units.go` inside it, starting with `package units`.

### 3.2 Look up the units

This task adds the table of units and the first part of the body, which rejects unknown unit names.

Each unit is defined by its number of decimals, so store them in a map at the top of the file, outside any function.

```go
var decimals = map[string]int{
	"wei":   0,
	"gwei":  9,
	"ether": 18,
}
```

A Go map works like a Solidity mapping, with one difference. Solidity returns 0 for a missing key and you can't tell it apart from a stored 0, but Go can tell you whether the key exists: `fromDec, ok := decimals[from]` sets `ok` to `false` when `from` is not in the map. That matters here, because `wei` really is 0. Go has no `const` maps, so it is a `var`, and nothing stops other code in the package from changing it. In a package this small that is an acceptable risk.

Then add the function.

```go
// Convert moves amount between units, e.g. Convert("1.5", "ether", "gwei") returns "1500000000".
func Convert(amount, from, to string) (string, error) {
	return "", nil
}
```

`amount, from, to string` is shorthand for three `string` parameters. Inside the body, look up `from` and `to` with the comma-ok form, and return `"", fmt.Errorf("unknown unit: %q", from)` (or `to`) when `ok` is false. Keep both looked-up numbers, as `fromDec` and `toDec`, since task 3.5 needs them. Until then, Go will refuse to compile because they are unused, so temporarily write `_ = fromDec` and `_ = toDec` before the `return`.

### 3.3 Test unknown units

This task writes the error test now, so every later task can add rows to it.

Create `units/units_test.go` with `package units` and a test called `TestConvertErrors`. Use a struct table like `TestToChecksumErrors`, but with three fields, `amount`, `from` and `to`, since `Convert` takes three inputs. Start with two rows.

| amount | from | to |
|---|---|---|
| `1` | banana | wei |
| `1` | ether | banana |

For the `t.Run` name, join the fields into one string, for example `tc.amount + " " + tc.from + " to " + tc.to`. Fail the row with `t.Errorf` when `err == nil`, as you did in step 2. Run `go test ./units` and expect `ok`.

### 3.4 Reject bad amounts

This task adds the second part of the body, which checks that the amount is a valid number.

Split the amount at the dot with `whole, frac, found := strings.Cut(amount, ".")`. For `"1.5"` that gives `"1"`, `"5"` and `true`, and for `"7"` it gives `"7"`, `""` and `false`. `Cut` splits only at the first dot, so `"1.2.3"` gives `"1"` and `"2.3"`.

Then write a small helper below `Convert` that reports whether a string is all digits.

```go
func isDigits(s string) bool
```

Its lowercase name keeps it private to the package. Inside, return `false` for an empty string, then loop over the bytes as in task 2.6 and return `false` as soon as one is outside `'0'` to `'9'`. Return `true` after the loop.

An amount is valid when `whole` is all digits and, if `found` is true, `frac` is all digits too. Otherwise return `fmt.Errorf("invalid amount: %q", amount)`. The empty-string rule in `isDigits` is what rejects `""`, `".5"` and `"5."`. The stray dot in `"2.3"` is what rejects `"1.2.3"`.

Add these rows to `TestConvertErrors`, all converting from ether to wei, and run the tests: `""`, `abc`, `-1`, `1.2.3`, `.5` and `5.`.

### 3.5 Move the decimal point

This task adds the third part of the body, the actual conversion.

The distance the point moves is `shift := fromDec - toDec`. Ether to gwei gives 18 - 9 = 9, so the point moves 9 places right, because there are more of the smaller unit. Gwei to ether gives -9, so it moves 9 places left. When `shift` is 0 there is nothing to move. Write it as an `if shift > 0 { ... } else if shift < 0 { ... }`.

**Moving right** (`shift > 0`) takes `shift` digits from the start of `frac` and puts them on the end of `whole`. When `frac` is too short, pad it with zeros on the right first.

```
"1.5" ether to gwei, shift 9
frac "5" is shorter than 9  ->  pad to "500000000"
whole = "1" + frac[:9]   ->  "1500000000"
frac  = frac[9:]         ->  ""
```

Padding is `frac = frac + strings.Repeat("0", shift-len(frac))`, done only when `len(frac) < shift`. `frac[:9]` is the first 9 characters and `frac[9:]` is everything after them, the same slicing you used for `addr[2:]`.

**Moving left** (`shift < 0`) is the mirror image. Let `n := -shift`, then take the last `n` digits of `whole` and put them in front of `frac`. When `whole` has `n` digits or fewer, pad it with zeros on the left first, so at least one digit stays in front of the point.

```
"21000" gwei to ether, n 9
whole "21000" has 9 or fewer digits  ->  pad to "0000021000" (10 digits)
frac  = whole[10-9:] + frac   ->  "000021000"
whole = whole[:10-9]          ->  "0"
```

Padding is `whole = strings.Repeat("0", n-len(whole)+1) + whole`, done only when `len(whole) <= n`. In code, `10` is `len(whole)`, so the two lines are `frac = whole[len(whole)-n:] + frac` and `whole = whole[:len(whole)-n]`. Write them in that order, because the second line changes `whole`.

Remove the two `_ =` lines from task 3.2, and for now return `whole + "." + frac, nil`. The output still has extra zeros, which the next task removes.

### 3.6 Clean up the result

This task adds the last part of the body, which formats the result and applies the wei rule.

Trim the zeros that carry no meaning: `whole = strings.TrimLeft(whole, "0")` and `frac = strings.TrimRight(frac, "0")`. Trimming can leave `whole` empty (for `"0.000021"` or `"0"`), so set it back to `"0"` when it is `""`.

Nothing is smaller than 1 wei, so when `to` is `"wei"` and `frac` is still not empty, return an error such as `errors.New("amount is less than 1 wei")`. Finally, return `whole` alone when `frac` is empty, and `whole + "." + frac` otherwise.

Add the row `0.0000000001` gwei to wei (a tenth of a wei) to `TestConvertErrors`.

### 3.7 Test the conversions

This task checks real conversions against answers you can verify by hand.

Add `TestConvert` with a struct table of four fields, `amount`, `from`, `to` and `want`. Since they are all strings, you can declare them on one line as `amount, from, to, want string`. The body follows the same `got`/`want` pattern as `TestToChecksum`, with `t.Fatalf` on an unexpected error.

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

The rows cover moving right, moving left, no move at all, and the cleanup rules. If one fails, print both values and count the zeros, since an off-by-one in the slicing shows up as one zero too many or too few.

### 3.8 Fuzz the round trip

This task adds one invariant over random amounts: converting ether to wei and back gives the same amount you started with.

```go
func FuzzConvertRoundTrip(f *testing.F) {
	f.Add(uint64(1), uint32(5))
	f.Fuzz(func(t *testing.T, whole uint64, frac uint32) {
		amount := fmt.Sprintf("%d.%d", whole, frac)
		// your checks go here
	})
}
```

This time the fuzzer hands you two numbers instead of bytes, and `fmt.Sprintf` (which works like `Printf` but returns the string) joins them into an amount such as `"1.5"`. A `uint32` has at most 10 digits, so the fractional part never goes below 1 wei, and every generated amount converts to wei without an error.

Write three calls and compare the results.

1. `wei` is `amount` converted from ether to wei.
2. `back` is `wei` converted from wei to ether.
3. `clean` is `amount` converted from ether to ether, which only applies the cleanup rules (`"1.50"` becomes `"1.5"`).

None of the three calls should return an error, and `back` must equal `clean`. You compare against `clean` and not `amount` because the fuzzer can produce inputs like `"1.50"` that come back as `"1.5"`, which is correct.

Run it with `go test -fuzz=FuzzConvertRoundTrip -fuzztime=30s ./units`.

Done when `go test ./units` passes and 30 seconds of fuzzing finds nothing.

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
