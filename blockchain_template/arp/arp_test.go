package arp

import "testing"

func TestNormalizeMAC(t *testing.T) {
	cases := map[string]string{
		"d0-94-66-e2-8e-93": "D0:94:66:E2:8E:93",
		"08:00:27:16:04:f0": "08:00:27:16:04:F0",
		"AA-BB-CC-DD-EE-FF": "AA:BB:CC:DD:EE:FF",
	}
	for in, want := range cases {
		if got := NormalizeMAC(in); got != want {
			t.Errorf("NormalizeMAC(%q) = %q, esperado %q", in, got, want)
		}
	}
}

func TestParseARPOutput(t *testing.T) {
	// Saída representativa de `arp -a` no Linux, incluindo uma entrada
	// link-local que deve ser filtrada.
	sample := `
gateway (172.25.10.1) at 00:11:22:33:44:55 [ether] on eth0
? (172.25.10.52) at d0-94-66-e2-8e-93 [ether] on eth0
? (172.25.10.70) at 08:00:27:16:04:f0 [ether] on eth0
? (169.254.0.5) at aa:bb:cc:dd:ee:ff [ether] on eth0
`
	entries := parseARPOutput(sample)

	if len(entries) != 3 {
		t.Fatalf("esperava 3 entradas (link-local filtrada), obtive %d: %+v", len(entries), entries)
	}

	want := map[string]string{
		"172.25.10.1":  "00:11:22:33:44:55",
		"172.25.10.52": "D0:94:66:E2:8E:93", // normalizado de "-" para ":" e maiúsculas
		"172.25.10.70": "08:00:27:16:04:F0",
	}
	for _, e := range entries {
		if want[e.IP] != e.MAC {
			t.Errorf("IP %s: MAC = %q, esperado %q", e.IP, e.MAC, want[e.IP])
		}
		if e.IP == "169.254.0.5" {
			t.Errorf("entrada link-local não deveria aparecer: %+v", e)
		}
	}
}