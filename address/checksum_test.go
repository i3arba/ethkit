package address

import (
	"encoding/hex"
	"strings"
	"testing"
)

// ToChecksum Tests

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

func FuzzToChecksum(f *testing.F) {
	f.Add([]byte("seed input at least 20 bytes long"))
	f.Fuzz(func(t *testing.T, a []byte) {
		if len(a) < 20 {
			return
		}
		lower := "0x" + hex.EncodeToString(a[:20])

		// Checks
		got, err := ToChecksum(lower)
		if err != nil {
			t.Fatalf("ToChecksum(%q) failed: %v", lower, err)
		}

		if strings.ToLower(got) != lower {
			t.Fatalf("ToChecksum(%q) changed characters: got %q", lower, got)
		}

		converted, _ := ToChecksum(got)
		if converted != got {
			t.Fatalf("ToChecksum is not stable: %q became %q", got, converted)
		}
		result, _ := IsChecksummed(got)
		if !result {
			t.Fatalf("IsChecksummed(%q) returned false", got)
		}
	})
}

// IsChecksum Tests

func TestIsChecksumErrors(t *testing.T) {
	cases := []string{
		"0x1234",
	}

	for _, ic := range cases {
		t.Run(ic, func(t *testing.T) {
			_, err := IsChecksummed(ic)

			if err == nil {
				t.Errorf("IsChecksummed(%q) returned no error", ic)
			}
		})
	}
}

func TestIsChecksum(t *testing.T) {
	cases := []struct {
		addr string
		want bool
	}{
		{"0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed", true},
		{"0x5aaeb6053F3E94C9b9A09f33669435E7Ef1BeAed", false},
		{"0xfB6916095ca1df60bB79Ce92cE3Ea74c37c5d359", true},
		{"0xFb6916095ca1df60bB79Ce92cE3Ea74c37c5d359", false},
		{"0xdbF03B407c01E7cD3CBea99509d93f8DDDC8C6FB", true},
		{"0xDBf03B407c01E7cD3CBea99509d93f8DDDC8C6FB", false},
		{"0xD1220A0cf47c7B9Be7A2E6BA89F429762e7b9aDb", true},
		{"0xD1220A0cf47c7B9Be7A2E6BA89F429762e7b9AdB", false},
		{"0x52908400098527886E0F7030069857D2E4169EE7", true},
		{"0x52908400098527886e0f7030069857D2E4169EE7", false},
		{"0xde709f2102306220921060314715629080e2fb77", true},
		{"0xDE709f2102306220921060314715629080e2fb77", false},
	}

	for _, ic := range cases {
		t.Run(ic.addr, func(t *testing.T) {
			got, err := IsChecksummed(ic.addr)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != ic.want {
				t.Errorf("IsChecksummed(%q) = %v, want %v", ic.addr, got, ic.want)
			}
		})
	}
}
