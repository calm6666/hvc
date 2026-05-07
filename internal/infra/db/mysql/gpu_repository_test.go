package mysql

import "testing"

func TestMapHWType(t *testing.T) {
	if got := mapHWType([]string{"nvidia"}); got != 1 {
		t.Fatalf("unexpected hw type for nvidia: %d", got)
	}
	if got := mapHWType([]string{"apple_videotoolbox"}); got != 5 {
		t.Fatalf("unexpected hw type for videotoolbox: %d", got)
	}
}
