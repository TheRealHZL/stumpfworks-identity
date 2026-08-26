package pin

import "testing"

func TestPIN(t *testing.T) {
	h, e := Hash("5831")
	if e != nil {
		t.Fatal(e)
	}
	if !Verify("5831", h) || Verify("5832", h) {
		t.Fatal("verification")
	}
	if _, e = Hash("12"); e == nil {
		t.Fatal("short PIN accepted")
	}
}
