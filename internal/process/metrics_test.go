package process

import "testing"

func TestSumGroup(t *testing.T) {
	out := ` 1234   1.5  2048
 1234  10.0  4096
 5678   3.0  1024
	1234	0.5	512
 9999   7.0  8192
`

	cpu, rssKB, matched := sumGroup(out, 1234)
	if !matched {
		t.Fatal("expected match for pgid 1234")
	}
	if cpu != 12.0 {
		t.Errorf("cpu = %v, want 12.0", cpu)
	}
	if rssKB != 6656 {
		t.Errorf("rssKB = %v, want 6656", rssKB)
	}
}

func TestSumGroup_NoMatch(t *testing.T) {
	out := " 5678   3.0  1024\n 9999   7.0  8192\n"

	cpu, rssKB, matched := sumGroup(out, 1234)
	if matched {
		t.Error("expected no match for pgid 1234")
	}
	if cpu != 0 || rssKB != 0 {
		t.Errorf("expected zeroes, got cpu=%v rssKB=%v", cpu, rssKB)
	}
}

func TestSumGroup_Empty(t *testing.T) {
	if _, _, matched := sumGroup("", 1234); matched {
		t.Error("expected no match for empty output")
	}
}

func TestSumGroup_MalformedLines(t *testing.T) {
	out := "garbage\n 1234\n 1234   1.0\n 1234   2.0  1000\n"

	cpu, rssKB, matched := sumGroup(out, 1234)
	if !matched {
		t.Fatal("expected match on the one well-formed line")
	}
	if cpu != 2.0 || rssKB != 1000 {
		t.Errorf("got cpu=%v rssKB=%v, want 2.0/1000", cpu, rssKB)
	}
}
