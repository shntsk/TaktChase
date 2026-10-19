package launcher

import (
	"math"
	"testing"

	"github.com/shntsk/taktchase/app/beatmap"
)

func TestParseQuery(t *testing.T) {
	terms, filters := parseQuery("Camellia stars>5 bpm>=180 mapper")

	if len(terms) != 2 || terms[0] != "camellia" || terms[1] != "mapper" {
		t.Fatalf("unexpected terms: %v", terms)
	}

	if len(filters) != 2 {
		t.Fatalf("unexpected filters: %v", filters)
	}

	if filters[0] != (mapFilter{"stars", ">", 5}) || filters[1] != (mapFilter{"bpm", ">=", 180}) {
		t.Fatalf("unexpected filters: %v", filters)
	}
}

func TestParseQueryIgnoresHalfTypedFilters(t *testing.T) {
	terms, filters := parseQuery("stars> bpm")

	if len(filters) != 0 {
		t.Fatalf("expected no filters, got %v", filters)
	}

	// "bpm" alone is plain text, "stars>" is a half typed filter and must not hide every map
	if len(terms) != 1 || terms[0] != "bpm" {
		t.Fatalf("unexpected terms: %v", terms)
	}
}

func TestMatchFilters(t *testing.T) {
	bm := beatmap.NewBeatMap()
	bm.Stars = 5.4
	bm.MinBPM = 120
	bm.MaxBPM = 200
	bm.Length = 90000
	bm.Circles = 100
	bm.Sliders = 20

	cases := []struct {
		q    string
		want bool
	}{
		{"stars>5", true},
		{"stars>6", false},
		{"bpm>180", true},
		{"bpm<100", false},
		{"bpm=150", true},
		{"length<=90", true},
		{"objects>=120", true},
		{"objects>121", false},
		{"stars>5 bpm>180", true},
		{"stars>5 bpm>200", false},
	}

	for _, c := range cases {
		_, f := parseQuery(c.q)

		if got := matchFilters(bm, f); got != c.want {
			t.Errorf("%q: got %v, want %v", c.q, got, c.want)
		}
	}

	bm.Stars = -1 // not calculated yet

	_, f := parseQuery("stars>1")
	if matchFilters(bm, f) {
		t.Error("maps without star rating must not match star filters")
	}
}

func TestFormatting(t *testing.T) {
	if got := fmtStat(4); got != "4.0" {
		t.Errorf("fmtStat(4) = %q", got)
	}

	if got := fmtStat(7.35); got != "7.35" {
		t.Errorf("fmtStat(7.35) = %q", got)
	}

	if got := fmtStars(-1); got != "N/A" {
		t.Errorf("fmtStars(-1) = %q", got)
	}

	if got := fmtDuration(58000); got != "0:58" {
		t.Errorf("fmtDuration = %q", got)
	}

	if got := fmtDuration(3723000); got != "1:02:03" {
		t.Errorf("fmtDuration = %q", got)
	}

	if got := fmtBPM(222, 222); got != "222" {
		t.Errorf("fmtBPM = %q", got)
	}

	if got := fmtBPM(100, 200); got != "100-200" {
		t.Errorf("fmtBPM = %q", got)
	}

	if got := fmtBPM(math.Inf(1), 0); got != "N/A" {
		t.Errorf("fmtBPM(inf) = %q", got)
	}
}

func TestPickDiff(t *testing.T) {
	mk := func(stars float64) *beatmap.BeatMap {
		b := beatmap.NewBeatMap()
		b.Stars = stars

		return b
	}

	set := &bSet{maps: []*beatmap.BeatMap{mk(2), mk(3.4), mk(5), mk(7)}}

	if got := pickDiff(set, nil); got != set.maps[3] {
		t.Error("without reference the hardest difficulty should be picked")
	}

	if got := pickDiff(set, mk(3.9)); got != set.maps[1] {
		t.Error("closest difficulty in stars should be picked")
	}
}

func TestDiffColorBuckets(t *testing.T) {
	if diffColor(1) == diffColor(3) || diffColor(3) == diffColor(7) {
		t.Error("different difficulty ranges should have different colors")
	}
}
