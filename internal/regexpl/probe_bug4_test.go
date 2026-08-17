package regexpl

import "testing"

func TestProbeUnicodeLiteralNode(t *testing.T) {
	res := Explain("é")
	if !res.OK || len(res.Tree) != 1 {
		t.Fatalf("unexpected result: %+v", res)
	}
	n := res.Tree[0]
	if n.Kind != "literal" || n.Text != "é" || n.Start != 0 || n.End != 2 {
		t.Fatalf("literal node=%+v", n)
	}
}
