package pulse

import (
	"bytes"
	"testing"
	"time"
)

func TestLocation(t *testing.T) {
	raw := "https://shop.example/a?ref=x%2Fy&ref=z&utm_source=old&utm%5Fsource=dup&flag#bottom"
	want := "https://shop.example/a?ref=x%2Fy&ref=z&flag&utm_source=new+one#bottom"
	if got := Location(raw, map[string]string{"source": "new one"}); got != want {
		t.Fatal(got)
	}
	if Location(raw, nil) != raw {
		t.Fatal("raw query changed")
	}
}
func TestVisitorScope(t *testing.T) {
	at := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	key := []byte("test")
	v := Visitor(key, at, "a", "192.0.2.1", " A  B ")
	if !bytes.Equal(v, Visitor(key, at, "a", "::ffff:192.0.2.1", "A B")) || bytes.Equal(v, Visitor(key, at.Add(24*time.Hour), "a", "192.0.2.1", "A B")) || bytes.Equal(v, Visitor(key, at, "b", "192.0.2.1", "A B")) {
		t.Fatal("normalization or daily scope")
	}
}
