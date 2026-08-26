package badge

import "testing"

func TestToken(t *testing.T) {
	a, e := GenerateToken()
	if e != nil || len(a) < 40 {
		t.Fatal(a, e)
	}
	b, _ := GenerateToken()
	if a == b {
		t.Fatal("tokens equal")
	}
	h := HashToken(a)
	if !VerifyToken(a, h) || VerifyToken(b, h) {
		t.Fatal("verification")
	}
}
func TestParse(t *testing.T) {
	c, x, e := ParsePayload("SWBADGE:1:SW-0001:secret")
	if e != nil || c != "SW-0001" || x != "secret" {
		t.Fatal(c, x, e)
	}
	if _, _, e = ParsePayload("bad"); e == nil {
		t.Fatal("expected error")
	}
}
