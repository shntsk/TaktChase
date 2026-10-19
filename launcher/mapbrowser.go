package launcher

import (
	"cmp"
	"fmt"
	"image"
	"math"
	"math/rand"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/AllenDang/cimgui-go/imgui"

	"github.com/shntsk/taktchase/app/beatmap"
	"github.com/shntsk/taktchase/app/settings"
	"github.com/shntsk/taktchase/framework/graphics/texture"
	"github.com/shntsk/taktchase/framework/qpc"
)

// This file contains the model/logic of the new map browser (the two-panel
// map/difficulty selection screen). Drawing lives in mapbrowser_ui.go.
// Audio preview, random/newest selection and beatmap reloading are reused
// from songSelectPopup, so nothing is duplicated.

// ---------------------------------------------------------------- grouping

type GroupBy int

const (
	GroupNone = GroupBy(iota)
	GroupArtist
	GroupCreator
)

var groupMethods = []GroupBy{GroupNone, GroupArtist, GroupCreator}

func (g GroupBy) String() string {
	switch g {
	case GroupNone:
		return "None"
	case GroupArtist:
		return "Artist"
	case GroupCreator:
		return "Mapper"
	}

	return ""
}

func (g GroupBy) key(b *beatmap.BeatMap) string {
	var k string

	switch g {
	case GroupArtist:
		k = b.Artist
	case GroupCreator:
		k = b.Creator
	}

	if strings.TrimSpace(k) == "" {
		return "Unknown"
	}

	return k
}

// ---------------------------------------------------------------- query parsing

type mapFilter struct {
	field string
	op    string
	val   float64
}

var filterRegexp = regexp.MustCompile(`^(stars?|sr|bpm|cs|ar|od|hp|length|len|objects|obj)(>=|<=|!=|>|<|=|:)(-?\d+(?:[.,]\d+)?)$`)
var partialFilterRegexp = regexp.MustCompile(`^(stars?|sr|bpm|cs|ar|od|hp|length|len|objects|obj)(>=|<=|!=|>|<|=|:)?$`)

// parseQuery splits the search string into plain text terms (all of them have to match)
// and numeric filters like "stars>5" or "bpm<=180".
func parseQuery(q string) (terms []string, filters []mapFilter) {
	for _, tok := range strings.Fields(strings.ToLower(q)) {
		if m := filterRegexp.FindStringSubmatch(tok); m != nil {
			v, err := strconv.ParseFloat(strings.ReplaceAll(m[3], ",", "."), 64)
			if err == nil {
				op := m[2]
				if op == ":" {
					op = "="
				}

				f := m[1]
				switch f {
				case "star", "sr":
					f = "stars"
				case "len":
					f = "length"
				case "obj":
					f = "objects"
				}

				filters = append(filters, mapFilter{field: f, op: op, val: v})

				continue
			}
		}

		// Half-typed filter ("stars>", "bpm"): ignore it instead of hiding every map while typing.
		if strings.ContainsAny(tok, "<>=:") && partialFilterRegexp.MatchString(tok) {
			continue
		}

		terms = append(terms, tok)
	}

	return
}

func matchTerms(name string, terms []string) bool {
	for _, t := range terms {
		if !strings.Contains(name, t) {
			return false
		}
	}

	return true
}

func compareNum(v float64, op string, target float64, eps float64) bool {
	switch op {
	case ">":
		return v > target
	case ">=":
		return v >= target-eps
	case "<":
		return v < target
	case "<=":
		return v <= target+eps
	case "=":
		return math.Abs(v-target) < eps
	case "!=":
		return math.Abs(v-target) >= eps
	}

	return true
}

func matchFilters(b *beatmap.BeatMap, filters []mapFilter) bool {
	for _, f := range filters {
		var ok bool

		switch f.field {
		case "stars":
			ok = b.Stars >= 0 && compareNum(b.Stars, f.op, f.val, 0.05)
		case "bpm":
			switch f.op {
			case ">", ">=":
				ok = compareNum(b.MaxBPM, f.op, f.val, 0.5)
			case "<", "<=":
				ok = compareNum(b.MinBPM, f.op, f.val, 0.5)
			case "=":
				ok = b.MinBPM-0.5 <= f.val && f.val <= b.MaxBPM+0.5
			default:
				ok = !(b.MinBPM-0.5 <= f.val && f.val <= b.MaxBPM+0.5)
			}
		case "cs":
			ok = compareNum(b.Diff.GetBaseCS(), f.op, f.val, 0.05)
		case "ar":
			ok = compareNum(b.Diff.GetBaseAR(), f.op, f.val, 0.05)
		case "od":
			ok = compareNum(b.Diff.GetBaseOD(), f.op, f.val, 0.05)
		case "hp":
			ok = compareNum(b.Diff.GetBaseHP(), f.op, f.val, 0.05)
		case "length":
			ok = compareNum(float64(b.Length)/1000, f.op, f.val, 0.5)
		case "objects":
			ok = compareNum(float64(b.Circles+b.Sliders+b.Spinners), f.op, f.val, 0.5)
		default:
			ok = true
		}

		if !ok {
			return false
		}
	}

	return true
}

// ---------------------------------------------------------------- formatting helpers

func fmtStat(v float64) string {
	r := math.Round(v*10) / 10
	if math.Abs(v-r) < 0.005 {
		return strconv.FormatFloat(r, 'f', 1, 64)
	}

	return strconv.FormatFloat(math.Round(v*100)/100, 'f', 2, 64)
}

func fmtStars(v float64) string {
	if v < 0 {
		return "N/A"
	}

	return fmt.Sprintf("%.2f", v)
}

func fmtDuration(ms int) string {
	s := max(ms, 0) / 1000

	if s >= 3600 {
		return fmt.Sprintf("%d:%02d:%02d", s/3600, (s/60)%60, s%60)
	}

	return fmt.Sprintf("%d:%02d", s/60, s%60)
}

func fmtBPM(minBPM, maxBPM float64) string {
	if math.IsInf(minBPM, 0) || math.IsInf(maxBPM, 0) || maxBPM <= 0 {
		return "N/A"
	}

	if math.Abs(minBPM-maxBPM) > 0.01 {
		return fmt.Sprintf("%.0f-%.0f", minBPM, maxBPM)
	}

	return fmt.Sprintf("%.0f", maxBPM)
}

func diffColor(stars float64) imgui.Vec4 {
	switch {
	case stars < 0:
		return vec4(0.55, 0.55, 0.6, 1)
	case stars < 2:
		return vec4(0.31, 0.89, 0.93, 1)
	case stars < 2.7:
		return vec4(0.40, 1.00, 0.66, 1)
	case stars < 4:
		return vec4(0.62, 1.00, 0.28, 1)
	case stars < 5.3:
		return vec4(1.00, 0.72, 0.35, 1)
	case stars < 6.5:
		return vec4(1.00, 0.40, 0.55, 1)
	}

	return vec4(0.60, 0.35, 0.80, 1)
}

// ---------------------------------------------------------------- model

const (
	cardH   = 80
	diffH   = 48
	diffGap = 6
	diffPad = 8
	rowGap  = 8
	headerH = 30
)

type bSet struct {
	dir      string
	maps     []*beatmap.BeatMap
	minStars float64
	maxStars float64
}

type bRow struct {
	header string // non-empty for group headers
	count  int
	set    *bSet
	y, h   float32
}

type mapBrowser struct {
	l   *launcher
	sel *songSelectPopup

	searchStr    string
	searchActive bool
	focusSearch  bool

	terms   []string
	filters []mapFilter

	rows         []bRow
	sets         []*bSet
	setIdx       map[string]int
	rowIdx       map[string]int
	totalH       float32
	totalSetsAll int
	totalDiffs   int // difficulties matching the current query

	expandedDir  string
	layoutDirty  bool
	needRebuild  bool
	scrollFrames int
	centerNext   bool

	previewAt  float64
	previewMap *beatmap.BeatMap

	enterDown bool
	lastCur   *beatmap.BeatMap

	thumbs *thumbCache
	trunc  map[truncKey]string
}

func newMapBrowser(l *launcher, sel *songSelectPopup) *mapBrowser {
	b := &mapBrowser{
		l:           l,
		sel:         sel,
		thumbs:      newThumbCache(),
		trunc:       make(map[truncKey]string),
		needRebuild: true,
		centerNext:  true,
	}

	if l.bld.currentMap != nil {
		b.expandedDir = l.bld.currentMap.Dir
	}

	return b
}

func (b *mapBrowser) setSearch(s string) {
	if s != b.searchStr {
		b.searchStr = s
		b.needRebuild = true
	}
}

func (b *mapBrowser) rebuild() {
	b.needRebuild = false
	b.sel.browserDirty = false

	b.terms, b.filters = parseQuery(b.searchStr)

	all := make(map[string]struct{})
	found := make([]*beatmap.BeatMap, 0, len(b.sel.beatmaps))

	for _, m := range b.sel.beatmaps {
		all[m.bMap.Dir] = struct{}{}

		if matchTerms(m.name, b.terms) && matchFilters(m.bMap, b.filters) {
			found = append(found, m.bMap)
		}
	}

	b.totalSetsAll = len(all)
	b.totalDiffs = len(found)

	sortMaps(found, launcherConfig.SortMapsBy)

	// One entry per beatmap set, in order of first appearance.
	b.sets = b.sets[:0]

	byDir := make(map[string]*bSet)

	for _, bm := range found {
		s := byDir[bm.Dir]
		if s == nil {
			s = &bSet{dir: bm.Dir, minStars: -1, maxStars: -1}
			byDir[bm.Dir] = s
			b.sets = append(b.sets, s)
		}

		s.maps = append(s.maps, bm)
	}

	for _, s := range b.sets {
		slices.SortStableFunc(s.maps, func(a, c *beatmap.BeatMap) int { return cmp.Compare(a.Stars, c.Stars) })

		for _, bm := range s.maps {
			if bm.Stars < 0 {
				continue
			}

			if s.minStars < 0 || bm.Stars < s.minStars {
				s.minStars = bm.Stars
			}

			s.maxStars = max(s.maxStars, bm.Stars)
		}
	}

	group := GroupBy(launcherConfig.GroupMapsBy)

	if group != GroupNone {
		slices.SortStableFunc(b.sets, func(a, c *bSet) int {
			return compareStrings(group.key(a.maps[0]), group.key(c.maps[0]))
		})
	}

	b.rows = b.rows[:0]
	b.setIdx = make(map[string]int, len(b.sets))
	b.rowIdx = make(map[string]int, len(b.sets))

	lastKey := ""

	for i, s := range b.sets {
		if group != GroupNone {
			k := group.key(s.maps[0])

			if len(b.rows) == 0 || k != lastKey {
				b.rows = append(b.rows, bRow{header: k})
				lastKey = k
			}

			b.rows[len(b.rows)-1].count++
		}

		b.setIdx[s.dir] = i
		b.rowIdx[s.dir] = len(b.rows)
		b.rows = append(b.rows, bRow{set: s})
	}

	b.layoutDirty = true
}

func (b *mapBrowser) relayout() {
	b.layoutDirty = false

	var y float32

	for i := range b.rows {
		r := &b.rows[i]
		r.y = y

		if r.set == nil {
			r.h = headerH
		} else {
			r.h = cardH

			if r.set.dir == b.expandedDir {
				r.h += diffPad + float32(len(r.set.maps))*(diffH+diffGap) - diffGap
			}
		}

		y += r.h + rowGap
	}

	b.totalH = y
}

// ---------------------------------------------------------------- selection

func (b *mapBrowser) currentSetIndex() int {
	cur := b.l.bld.currentMap
	if cur == nil {
		return -1
	}

	if i, ok := b.setIdx[cur.Dir]; ok {
		return i
	}

	return -1
}

// pickDiff chooses the difficulty of s closest in stars to the reference (or the hardest one).
func pickDiff(s *bSet, ref *beatmap.BeatMap) *beatmap.BeatMap {
	if ref == nil || ref.Stars < 0 {
		return s.maps[len(s.maps)-1]
	}

	best := s.maps[len(s.maps)-1]
	bestD := math.Inf(1)

	for _, m := range s.maps {
		if m.Stars < 0 {
			continue
		}

		if d := math.Abs(m.Stars - ref.Stars); d < bestD {
			bestD = d
			best = m
		}
	}

	return best
}

func (b *mapBrowser) selectMap(bm *beatmap.BeatMap) {
	cur := b.l.bld.currentMap

	if bm == nil || cur == bm {
		return
	}

	setChanged := cur == nil || cur.Dir != bm.Dir

	b.l.bld.setMap(bm)

	if b.expandedDir != bm.Dir {
		b.expandedDir = bm.Dir
		b.layoutDirty = true
	}

	b.scrollFrames = 3

	if setChanged || b.sel.prevMap == nil || b.sel.prevMap.Dir != bm.Dir {
		b.previewMap = bm
		b.previewAt = qpc.GetMilliTimeF() + 250 // debounce fast keyboard navigation
	}
}

func (b *mapBrowser) moveSet(delta int) {
	if len(b.sets) == 0 {
		return
	}

	idx := b.currentSetIndex()
	if idx < 0 {
		if delta > 0 {
			idx = -1
		} else {
			idx = len(b.sets)
		}
	}

	idx = min(max(idx+delta, 0), len(b.sets)-1)

	b.selectMap(pickDiff(b.sets[idx], b.l.bld.currentMap))
}

func (b *mapBrowser) selectEdge(last bool) {
	if len(b.sets) == 0 {
		return
	}

	idx := 0
	if last {
		idx = len(b.sets) - 1
	}

	b.selectMap(pickDiff(b.sets[idx], b.l.bld.currentMap))
}

func (b *mapBrowser) moveDiff(delta int) {
	idx := b.currentSetIndex()
	if idx < 0 {
		return
	}

	s := b.sets[idx]
	cur := slices.Index(s.maps, b.l.bld.currentMap)

	if cur < 0 {
		return
	}

	b.selectMap(s.maps[min(max(cur+delta, 0), len(s.maps)-1)])
}

func (b *mapBrowser) selectRandom() {
	if len(b.sets) == 0 {
		return
	}

	s := b.sets[rand.Intn(len(b.sets))]

	b.selectMap(pickDiff(s, b.l.bld.currentMap))
	b.centerNext = true
}

func (b *mapBrowser) toggleSet(s *bSet) {
	cur := b.l.bld.currentMap

	if cur != nil && cur.Dir == s.dir {
		if b.expandedDir == s.dir {
			b.expandedDir = ""
		} else {
			b.expandedDir = s.dir
		}

		b.layoutDirty = true

		return
	}

	b.selectMap(pickDiff(s, cur))
}

func (b *mapBrowser) updatePreview() {
	if b.previewMap == nil || qpc.GetMilliTimeF() < b.previewAt {
		return
	}

	bm := b.previewMap
	b.previewMap = nil

	if launcherConfig.PreviewSelected && b.l.bld.currentMap == bm {
		b.sel.stopPreview()
		b.sel.startPreview(bm)
	}
}

func (b *mapBrowser) togglePreview() {
	cur := b.l.bld.currentMap
	if cur == nil {
		return
	}

	if b.sel.PreviewedSong != nil && b.sel.prevMap != nil && b.sel.prevMap.Dir == cur.Dir {
		b.sel.stopPreview()
	} else {
		b.sel.stopPreview()
		b.sel.startPreview(cur)
	}
}

// ---------------------------------------------------------------- text truncation cache

type truncKey struct {
	s  string
	w  int
	fs int
}

func (b *mapBrowser) truncate(s string, maxW float32, fs int) string {
	k := truncKey{s, int(maxW), fs}

	if v, ok := b.trunc[k]; ok {
		return v
	}

	if len(b.trunc) > 4000 {
		b.trunc = make(map[truncKey]string)
	}

	v := truncText(s, maxW)
	b.trunc[k] = v

	return v
}

// truncText shortens s with "..." so it fits in maxW in the current font. Must be called with the font pushed.
func truncText(s string, maxW float32) string {
	if maxW <= 0 {
		return ""
	}

	if imgui.CalcTextSizeV(s, false, 0).X <= maxW {
		return s
	}

	r := []rune(s)
	lo, hi := 0, len(r)

	for lo < hi {
		mid := (lo + hi + 1) / 2

		if imgui.CalcTextSizeV(string(r[:mid])+"...", false, 0).X <= maxW {
			lo = mid
		} else {
			hi = mid - 1
		}
	}

	return strings.TrimRight(string(r[:lo]), " ") + "..."
}

// ---------------------------------------------------------------- cover thumbnails

const (
	thumbW       = 384
	thumbH       = 216
	thumbMaxKeep = 48
)

type thumbState int

const (
	thumbLoading = thumbState(iota)
	thumbReady
	thumbFailed
)

type thumb struct {
	state    thumbState
	tex      *texture.TextureSingle
	ref      *imgui.TextureRef
	lastUsed int64
}

type thumbResult struct {
	path string
	img  *image.RGBA
}

type thumbCache struct {
	items map[string]*thumb
	frame int64

	mu    sync.Mutex
	ready []thumbResult
	sem   chan struct{}
}

func newThumbCache() *thumbCache {
	return &thumbCache{
		items: make(map[string]*thumb),
		sem:   make(chan struct{}, 2),
	}
}

func coverPath(bm *beatmap.BeatMap) string {
	if bm == nil || bm.Bg == "" {
		return ""
	}

	return filepath.Join(settings.General.GetSongsDir(), bm.Dir, bm.Bg)
}

// get returns a ready thumbnail for the path or nil. Decoding happens off the main thread.
func (c *thumbCache) get(path string) *thumb {
	if path == "" {
		return nil
	}

	t, ok := c.items[path]
	if !ok {
		t = &thumb{state: thumbLoading}
		c.items[path] = t

		go c.load(path)
	}

	t.lastUsed = c.frame

	if t.state == thumbReady {
		return t
	}

	return nil
}

func (c *thumbCache) load(path string) {
	c.sem <- struct{}{}

	defer func() { <-c.sem }()

	var img *image.RGBA

	func() {
		defer func() { _ = recover() }()

		img = loadCover(path)
	}()

	c.mu.Lock()
	c.ready = append(c.ready, thumbResult{path, img})
	c.mu.Unlock()
}

// pump uploads at most one decoded image per frame to the GPU (main thread only) and evicts old entries.
func (c *thumbCache) pump() {
	c.frame++

	c.mu.Lock()

	var res *thumbResult

	if len(c.ready) > 0 {
		r := c.ready[0]
		c.ready = c.ready[1:]
		res = &r
	}

	c.mu.Unlock()

	if res != nil {
		if t, ok := c.items[res.path]; ok {
			if res.img == nil {
				t.state = thumbFailed
			} else {
				t.tex = texture.LoadTextureSingle(res.img, 4)
				t.ref = imgui.NewTextureRefTextureID(imgui.TextureID(t.tex.GetID()))
				t.state = thumbReady
			}
		}
	}

	if len(c.items) > thumbMaxKeep {
		var oldestKey string
		oldest := int64(math.MaxInt64)

		for k, t := range c.items {
			if t.state != thumbLoading && t.lastUsed < c.frame-1 && t.lastUsed < oldest {
				oldest = t.lastUsed
				oldestKey = k
			}
		}

		if oldestKey != "" {
			t := c.items[oldestKey]

			if t.tex != nil {
				t.tex.Dispose()
			}

			if t.ref != nil {
				t.ref.Destroy()
			}

			delete(c.items, oldestKey)
		}
	}
}

// loadCover decodes an image, center-crops it to 16:9 and box-downsamples it to a small texture,
// so dozens of covers fit in memory.
func loadCover(path string) *image.RGBA {
	pix, err := texture.NewPixmapFileString(path)
	if err != nil || pix == nil {
		return nil
	}

	defer pix.Dispose()

	if pix.Components < 4 || pix.Width < 2 || pix.Height < 2 {
		return nil
	}

	src := pix.RGBA()
	sw, sh := pix.Width, pix.Height

	cw, ch := sw, sh
	if sw*9 > sh*16 {
		cw = sh * 16 / 9
	} else {
		ch = sw * 9 / 16
	}

	x0, y0 := (sw-cw)/2, (sh-ch)/2

	dst := image.NewRGBA(image.Rect(0, 0, thumbW, thumbH))

	const samples = 3

	for y := 0; y < thumbH; y++ {
		for x := 0; x < thumbW; x++ {
			var r, g, bl, a, n uint32

			for sy := 0; sy < samples; sy++ {
				for sx := 0; sx < samples; sx++ {
					px := x0 + ((x*samples+sx)*cw)/(thumbW*samples)
					py := y0 + ((y*samples+sy)*ch)/(thumbH*samples)

					px = min(px, sw-1)
					py = min(py, sh-1)

					o := py*src.Stride + px*4

					r += uint32(src.Pix[o])
					g += uint32(src.Pix[o+1])
					bl += uint32(src.Pix[o+2])
					a += uint32(src.Pix[o+3])
					n++
				}
			}

			d := y*dst.Stride + x*4
			dst.Pix[d] = uint8(r / n)
			dst.Pix[d+1] = uint8(g / n)
			dst.Pix[d+2] = uint8(bl / n)
			dst.Pix[d+3] = uint8(a / n)
		}
	}

	return dst
}
