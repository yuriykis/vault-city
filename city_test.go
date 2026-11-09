package main

import (
	"fmt"
	"math"
	"math/rand/v2"
	"reflect"
	"slices"
	"testing"

	rl "github.com/gen2brain/raylib-go/raylib"
)

const eps = 1e-3

// sampleVault has districts of many sizes, some with a hub and some
// without, plus suburbs.
func sampleVault() []Note {
	var notes []Note
	for t := range 40 {
		topic := fmt.Sprintf("Topic %02d", t)
		if t%2 == 0 {
			notes = append(notes, Note{Name: topic, Inbound: t})
		}
		for m := range t%13 + 1 {
			notes = append(notes, Note{Name: fmt.Sprintf("%s note %d", topic, m), Topic: topic, Inbound: m})
		}
	}
	for s := range 50 {
		notes = append(notes, Note{Name: fmt.Sprintf("Loose %02d", s)})
	}
	return notes
}

// center is the middle of a box on the ground.
func center(b rl.BoundingBox) (x, z float64) {
	return float64(b.Min.X+b.Max.X) / 2, float64(b.Min.Z+b.Max.Z) / 2
}

// within reports whether the footprint of inner lies inside outer.
func within(inner, outer rl.BoundingBox) bool {
	return inner.Min.X >= outer.Min.X-eps && inner.Max.X <= outer.Max.X+eps &&
		inner.Min.Z >= outer.Min.Z-eps && inner.Max.Z <= outer.Max.Z+eps
}

// gap is the distance between two footprints along the axis that separates
// them most; negative when they overlap.
func gap(a, b rl.BoundingBox) float64 {
	dx := max(float64(b.Min.X-a.Max.X), float64(a.Min.X-b.Max.X))
	dz := max(float64(b.Min.Z-a.Max.Z), float64(a.Min.Z-b.Max.Z))
	return max(dx, dz)
}

// blockOf returns the name of the block a building stands in.
func blockOf(t *testing.T, c City, b Building) string {
	t.Helper()
	for _, block := range c.Blocks {
		if within(b.Box, block.Box) {
			return block.Name
		}
	}
	t.Fatalf("building %q stands in no block", b.Name)
	return ""
}

func blockNamed(t *testing.T, c City, name string) Block {
	t.Helper()
	for _, b := range c.Blocks {
		if b.Name == name {
			return b
		}
	}
	t.Fatalf("no block %q in %v", name, c.Blocks)
	return Block{}
}

func buildingNamed(t *testing.T, c City, name string) Building {
	t.Helper()
	for _, b := range c.Buildings {
		if b.Name == name {
			return b
		}
	}
	t.Fatalf("no building %q", name)
	return Building{}
}

// TestBuildCityDistricts checks where each note lands: hubs in their own
// district whatever their topic (R1), topics matched ignoring case, notes
// without a topic in the suburbs.
func TestBuildCityDistricts(t *testing.T) {
	c := BuildCity([]Note{
		{Name: "Cooking", Topic: "Home"}, // hub of Cooking, though its topic is Home
		{Name: "Pasta", Topic: "Cooking"},
		{Name: "Soup", Topic: "cooking"},
		{Name: "Home"}, // hub of Home, though it has no topic
		{Name: "Garden", Topic: "Home"},
		{Name: "Lonely", Topic: "Nowhere"}, // a district without a hub
		{Name: "Loose"},
	})
	want := map[string]string{
		"Cooking": "Cooking", "Pasta": "Cooking", "Soup": "Cooking",
		"Home": "Home", "Garden": "Home",
		"Lonely": "Nowhere",
		"Loose":  "",
	}
	got := make(map[string]string)
	for _, b := range c.Buildings {
		if _, dup := got[b.Name]; dup {
			t.Errorf("note %q has two buildings", b.Name)
		}
		got[b.Name] = blockOf(t, c, b)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("note → block = %v, want %v", got, want)
	}
	if hub := blockNamed(t, c, "Cooking").Hub; hub != "Cooking" {
		t.Errorf("Cooking hub = %q, want Cooking", hub)
	}
	if hub := blockNamed(t, c, "Nowhere").Hub; hub != "" {
		t.Errorf("Nowhere hub = %q, want none", hub)
	}
}

func TestBuildCityEveryNoteOnce(t *testing.T) {
	notes := sampleVault()
	c := BuildCity(notes)
	var want, got []string
	for _, n := range notes {
		want = append(want, n.Name)
	}
	for _, b := range c.Buildings {
		got = append(got, b.Name)
		blockOf(t, c, b)
	}
	slices.Sort(want)
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Errorf("got %d buildings, want one for each of %d notes", len(got), len(want))
	}
}

func TestBuildCityDistrictLeftEmptyByItsHub(t *testing.T) {
	// B's topic is A, but B is the hub of district B, so nothing is left in A.
	c := BuildCity([]Note{{Name: "B", Topic: "A"}, {Name: "C", Topic: "B"}})
	if len(c.Blocks) != 1 || c.Blocks[0].Name != "B" {
		t.Errorf("blocks = %+v, want only B", c.Blocks)
	}
}

// TestBuildCityHubOnCenterLot checks block sides (the smallest odd side
// that fits members plus hub, R8) and that the hub stands on the center lot.
func TestBuildCityHubOnCenterLot(t *testing.T) {
	for _, tt := range []struct{ members, side int }{{0, 1}, {1, 3}, {4, 3}, {8, 3}, {9, 5}} {
		t.Run(fmt.Sprintf("%d members", tt.members), func(t *testing.T) {
			notes := []Note{{Name: "Hub"}}
			for m := range tt.members {
				notes = append(notes, Note{Name: fmt.Sprintf("Member %d", m), Topic: "Hub"})
			}
			if tt.members == 0 {
				notes[0].Topic = "Hub" // a district needs a note with the topic
			}
			c := BuildCity(notes)
			block := blockNamed(t, c, "Hub")
			wantWidth := float64(tt.side)*lotPitch - alleyWidth
			if w := float64(block.Box.Max.X - block.Box.Min.X); math.Abs(w-wantWidth) > eps {
				t.Errorf("block width = %v, want %v (side %d)", w, wantWidth, tt.side)
			}
			hx, hz := center(buildingNamed(t, c, "Hub").Box)
			bx, bz := center(block.Box)
			if math.Abs(hx-bx) > eps || math.Abs(hz-bz) > eps {
				t.Errorf("hub at (%v, %v), block center (%v, %v)", hx, hz, bx, bz)
			}
		})
	}
}

// TestBuildCityNoOverlap checks that alleys and streets keep every building
// and block apart, wide enough for the player to walk between them (R11).
func TestBuildCityNoOverlap(t *testing.T) {
	if alleyWidth <= 2*playerRadius {
		t.Fatalf("alleyWidth %v is not wider than the player (%v)", alleyWidth, 2*playerRadius)
	}
	c := BuildCity(sampleVault())
	for i, a := range c.Buildings {
		for _, b := range c.Buildings[i+1:] {
			if g := gap(a.Box, b.Box); g < alleyWidth-eps {
				t.Fatalf("buildings %q and %q are %v apart, want at least %v", a.Name, b.Name, g, alleyWidth)
			}
		}
	}
	for i, a := range c.Blocks {
		for _, b := range c.Blocks[i+1:] {
			if g := gap(a.Box, b.Box); g < streetWidth-eps {
				t.Fatalf("blocks %q and %q are %v apart, want at least %v", a.Name, b.Name, g, streetWidth)
			}
		}
	}
}

// TestBuildCityBlockOrder checks blocks go largest first, then by name,
// with the suburbs last even when they are the largest (R2).
func TestBuildCityBlockOrder(t *testing.T) {
	c := BuildCity(sampleVault())
	count := make(map[string]int)
	for _, b := range c.Buildings {
		count[blockOf(t, c, b)]++
	}
	last := c.Blocks[len(c.Blocks)-1]
	if last.Name != "" {
		t.Errorf("last block = %q, want the suburbs", last.Name)
	}
	districts := c.Blocks[:len(c.Blocks)-1]
	for i := 1; i < len(districts); i++ {
		a, b := districts[i-1], districts[i]
		if count[a.Name] < count[b.Name] || count[a.Name] == count[b.Name] && a.Name > b.Name {
			t.Errorf("block %q (%d notes) comes before %q (%d notes)", a.Name, count[a.Name], b.Name, count[b.Name])
		}
	}
}

// TestBuildCityRowsWrap checks that no row of blocks is wider than
// √(total block area), unless a single block is wider on its own (R2).
func TestBuildCityRowsWrap(t *testing.T) {
	c := BuildCity(sampleVault())
	var area float64
	for _, b := range c.Blocks {
		area += math.Pow(float64(b.Box.Max.X-b.Box.Min.X)+streetWidth, 2)
	}
	limit := math.Sqrt(area)
	rows := 0
	for _, b := range c.Blocks {
		if b.Box.Min.X == 0 {
			rows++
		} else if float64(b.Box.Max.X) > limit+eps {
			t.Errorf("block %q ends at x = %v, past the row limit %v", b.Name, b.Box.Max.X, limit)
		}
	}
	if rows < 2 {
		t.Errorf("%d row(s), want the blocks wrapped into several", rows)
	}
}

func TestBuildCityIgnoresInputOrder(t *testing.T) {
	notes := sampleVault()
	want := BuildCity(notes)
	shuffled := slices.Clone(notes)
	rand.New(rand.NewPCG(1, 2)).Shuffle(len(shuffled), func(i, j int) {
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	})
	if got := BuildCity(shuffled); !reflect.DeepEqual(got, want) {
		t.Error("shuffled notes give a different city")
	}
}

// TestBuildCitySpawn checks the start: on the street next to the largest
// district, not the larger suburbs, outside every building and facing the
// district's hub (R10).
func TestBuildCitySpawn(t *testing.T) {
	notes := []Note{{Name: "Big"}, {Name: "Small"}, {Name: "Small note", Topic: "Small"}}
	for m := range 8 {
		notes = append(notes, Note{Name: fmt.Sprintf("Big note %d", m), Topic: "Big"})
	}
	for s := range 30 {
		notes = append(notes, Note{Name: fmt.Sprintf("Loose %02d", s)})
	}
	c := BuildCity(notes)
	checkSpawnOutside(t, c)

	block := blockNamed(t, c, "Big")
	if g := gap(block.Box, rl.BoundingBox{Min: c.Spawn, Max: c.Spawn}); g < 0 || g > streetWidth/2+eps {
		t.Errorf("spawn %v is %v from the Big block, want on the street next to it", c.Spawn, g)
	}
	hx, hz := center(buildingNamed(t, c, "Big").Box)
	want := rl.Vector3Normalize(rl.NewVector3(float32(hx)-c.Spawn.X, 0, float32(hz)-c.Spawn.Z))
	if rl.Vector3Distance(c.Facing, want) > eps {
		t.Errorf("facing %v, want %v toward the Big hub", c.Facing, want)
	}
}

func TestBuildCityOnlySuburbs(t *testing.T) {
	c := BuildCity([]Note{{Name: "A"}, {Name: "B"}, {Name: "C"}})
	if len(c.Blocks) != 1 || c.Blocks[0].Name != "" || len(c.Buildings) != 3 {
		t.Errorf("blocks = %+v with %d buildings, want the suburbs with 3", c.Blocks, len(c.Buildings))
	}
	checkSpawnOutside(t, c)
}

func TestBuildCityEmpty(t *testing.T) {
	c := BuildCity(nil)
	if len(c.Buildings) != 0 || len(c.Blocks) != 0 {
		t.Errorf("empty vault: %d buildings, %d blocks", len(c.Buildings), len(c.Blocks))
	}
	if l := rl.Vector3Length(c.Facing); math.Abs(float64(l)-1) > eps {
		t.Errorf("facing %v, want a unit vector", c.Facing)
	}
}

// checkSpawnOutside fails when the player at the spawn would touch a building.
func checkSpawnOutside(t *testing.T, c City) {
	t.Helper()
	at := rl.BoundingBox{Min: c.Spawn, Max: c.Spawn}
	for _, b := range c.Buildings {
		if gap(b.Box, at) < playerRadius {
			t.Errorf("spawn %v is inside or touching %q", c.Spawn, b.Name)
		}
	}
}

// TestPushOut checks how a building 3×3 wide and 5 high pushes the player
// out along the shortest axis, keeping playerRadius (0.5) from its walls.
func TestPushOut(t *testing.T) {
	box := []Building{{Name: "A", Box: rl.BoundingBox{Min: rl.NewVector3(0, 0, 0), Max: rl.NewVector3(3, 5, 3)}}}
	for _, tt := range []struct {
		name     string
		pos, out rl.Vector3
	}{
		{"street", rl.NewVector3(10, 2, 10), rl.NewVector3(10, 2, 10)},
		{"exactly the radius away", rl.NewVector3(-0.5, 2, 1.5), rl.NewVector3(-0.5, 2, 1.5)},
		{"inside near the min x wall", rl.NewVector3(0.2, 2, 1.5), rl.NewVector3(-0.5, 2, 1.5)},
		{"too close to the max x wall", rl.NewVector3(3.3, 2, 1), rl.NewVector3(3.5, 2, 1)},
		{"inside near the min z wall", rl.NewVector3(1.4, 2, 0.1), rl.NewVector3(1.4, 2, -0.5)},
		{"too close to the max z wall", rl.NewVector3(2, 2, 3.2), rl.NewVector3(2, 2, 3.5)},
		{"corner goes along the shorter axis", rl.NewVector3(-0.4, 2, -0.1), rl.NewVector3(-0.5, 2, -0.1)},
		{"just below roof plus radius", rl.NewVector3(0.2, 5.4, 1.5), rl.NewVector3(-0.5, 5.4, 1.5)},
		{"above the roof", rl.NewVector3(1.5, 5.5, 1.5), rl.NewVector3(1.5, 5.5, 1.5)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := pushOut(tt.pos, box); rl.Vector3Distance(got, tt.out) > eps {
				t.Errorf("pushOut(%v) = %v, want %v", tt.pos, got, tt.out)
			}
		})
	}
}

func TestKeepOut(t *testing.T) {
	box := []Building{{Name: "A", Box: rl.BoundingBox{Min: rl.NewVector3(0, 0, 0), Max: rl.NewVector3(3, 5, 3)}}}
	walker, other := rl.NewVector3(0.2, 2, 1.5), rl.NewVector3(-10, 3, 1.5)
	keepOut(&walker, &other, box)
	if want := rl.NewVector3(-0.5, 2, 1.5); rl.Vector3Distance(walker, want) > eps {
		t.Errorf("walker = %v, want %v", walker, want)
	}
	if want := rl.NewVector3(-10.7, 3, 1.5); rl.Vector3Distance(other, want) > eps {
		t.Errorf("other = %v, want %v, moved by as much as the walker", other, want)
	}
}

// TestBuildCityEveryBuildingReachable walks from the spawn at eye height
// over a half-unit grid, through every point the buildings don't push the
// player out of, and checks it gets right next to every building, hubs in
// the middle of their blocks included (R11).
func TestBuildCityEveryBuildingReachable(t *testing.T) {
	c := BuildCity(sampleVault())
	const step = 0.5
	var maxX, maxZ float64
	for _, b := range c.Blocks {
		maxX, maxZ = max(maxX, float64(b.Box.Max.X)), max(maxZ, float64(b.Box.Max.Z))
	}
	origin := -streetWidth
	nx, nz := int((maxX+2*streetWidth)/step)+1, int((maxZ+2*streetWidth)/step)+1
	point := func(i, j int) rl.Vector3 {
		return rl.NewVector3(float32(origin+float64(i)*step), eyeHeight, float32(origin+float64(j)*step))
	}
	cell := func(x, z float32) (int, int) {
		return int(math.Round((float64(x) - origin) / step)), int(math.Round((float64(z) - origin) / step))
	}

	reached := make([][]bool, nx)
	for i := range reached {
		reached[i] = make([]bool, nz)
	}
	i0, j0 := cell(c.Spawn.X, c.Spawn.Z)
	queue := [][2]int{{i0, j0}}
	reached[i0][j0] = true
	for len(queue) > 0 {
		i, j := queue[0][0], queue[0][1]
		queue = queue[1:]
		for _, d := range [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			ni, nj := i+d[0], j+d[1]
			if ni < 0 || nj < 0 || ni >= nx || nj >= nz || reached[ni][nj] {
				continue
			}
			if p := point(ni, nj); pushOut(p, c.Buildings) == p {
				reached[ni][nj] = true
				queue = append(queue, [2]int{ni, nj})
			}
		}
	}

	for _, b := range c.Buildings {
		x, z := center(b.Box)
		// The middle of each side, exactly playerRadius from the wall.
		sides := [][2]float64{
			{x, float64(b.Box.Min.Z) - playerRadius}, {x, float64(b.Box.Max.Z) + playerRadius},
			{float64(b.Box.Min.X) - playerRadius, z}, {float64(b.Box.Max.X) + playerRadius, z},
		}
		if !slices.ContainsFunc(sides, func(s [2]float64) bool {
			i, j := cell(float32(s[0]), float32(s[1]))
			return reached[i][j]
		}) {
			t.Errorf("building %q can't be reached from the spawn", b.Name)
		}
	}
}

func TestBuildCityHeight(t *testing.T) {
	c := BuildCity([]Note{{Name: "A"}, {Name: "B", Inbound: 1}, {Name: "C", Inbound: 3}, {Name: "D", Inbound: 157}})
	for name, want := range map[string]float64{
		"A": minHeight,
		"B": minHeight + heightPerDoubling,
		"C": minHeight + 2*heightPerDoubling,
		"D": minHeight + heightPerDoubling*math.Log2(158),
	} {
		if got := float64(buildingNamed(t, c, name).Box.Max.Y); math.Abs(got-want) > eps {
			t.Errorf("%s height = %v, want %v", name, got, want)
		}
	}
}

func TestBuildCityColors(t *testing.T) {
	c := BuildCity([]Note{
		{Name: "A", Category: "Knowledge"},
		{Name: "B", Category: "Books"},
		{Name: "C", Category: "Knowledge"},
		{Name: "D"},
	})
	for name, want := range map[string]rl.Color{
		"A": categoryPalette[1], // Books comes first by name
		"B": categoryPalette[0],
		"C": categoryPalette[1],
		"D": uncategorized,
	} {
		if got := buildingNamed(t, c, name).Color; got != want {
			t.Errorf("%s color = %v, want %v", name, got, want)
		}
	}
}
