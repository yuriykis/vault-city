package main

import (
	"cmp"
	"math"
	"slices"
	"strings"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// City layout in world units; the camera's eye is 2 units above the ground.
const (
	lotSize           = 3.0 // building footprint, square
	alleyWidth        = 2.0 // gap between buildings in a block, wider than the player
	streetWidth       = 6.0 // gap between blocks
	playerRadius      = 0.5 // how close the camera may come to a wall
	minHeight         = 2.0 // a note nobody links to
	heightPerDoubling = 3.0 // added each time the inbound link count doubles

	lotPitch = lotSize + alleyWidth // distance between neighboring lot centers

	pickRange   = 60.0 // farthest building whose name is shown
	labelGap    = 12.0 // pixels between the aim point and the label above it
	labelMargin = 8.0  // pixels a label keeps from the screen edges
)

// categoryPalette colors categories in name order (Okabe-Ito plus two);
// with more categories than colors, colors repeat.
var categoryPalette = []rl.Color{
	rl.NewColor(230, 159, 0, 255),   // orange
	rl.NewColor(86, 180, 233, 255),  // sky blue
	rl.NewColor(0, 158, 115, 255),   // bluish green
	rl.NewColor(240, 228, 66, 255),  // yellow
	rl.NewColor(0, 114, 178, 255),   // blue
	rl.NewColor(213, 94, 0, 255),    // vermillion
	rl.NewColor(204, 121, 167, 255), // reddish purple
	rl.NewColor(117, 112, 179, 255), // purple
	rl.NewColor(102, 166, 30, 255),  // green
}

// uncategorized colors notes without a category.
var uncategorized = rl.NewColor(150, 150, 150, 255)

// Building is one note standing on its lot.
type Building struct {
	Name  string
	Box   rl.BoundingBox
	Color rl.Color
}

// Block is one district, or the suburbs, on the map: a square of lots.
type Block struct {
	Name string         // district name; "" for the suburbs
	Hub  string         // note on the center lot; "" when the district has none
	Box  rl.BoundingBox // ground from the first lot's edge to the last one's, y = 0
}

// City is everything the game draws and walks through.
type City struct {
	Buildings []Building
	Blocks    []Block
	Spawn     rl.Vector3 // a street point next to the largest district, y = 0
	Facing    rl.Vector3 // unit vector from Spawn toward that district's hub
}

// district is a block before it has a place on the map.
type district struct {
	name    string
	hub     *Note
	members []Note // by name
}

// size is how many lots the district fills.
func (d *district) size() int {
	if d.hub != nil {
		return len(d.members) + 1
	}
	return len(d.members)
}

// BuildCity lays the notes out as a city. Each topic is a district: a square
// block whose side is the smallest odd number of lots that fits its notes, so
// it always has a center lot. The district's hub, the note named like the
// topic, stands there whatever its own topic is; the other notes fill the
// remaining lots by name, row by row. Notes without a topic are the suburbs.
//
// A block for 4 notes plus a hub (5 lots, side 3):
//
//	┌───┬───┬───┐
//	│ a │ b │ c │   a..d: members by name
//	├───┼───┼───┤   HUB: the center lot
//	│ d │HUB│   │   alleys between lots are wider than the player
//	├───┼───┼───┤
//	│   │   │   │
//	└───┴───┴───┘
//
// Blocks go largest first, then by name, into rows at most √(total block
// area) wide, with streets between them; the suburbs come last. The player
// starts on the street in front of the largest district, facing its hub.
// The city depends only on the notes, never on their order.
func BuildCity(notes []Note) City {
	sorted := slices.Clone(notes)
	slices.SortFunc(sorted, func(a, b Note) int {
		return cmp.Or(strings.Compare(a.Name, b.Name), strings.Compare(a.Topic, b.Topic),
			strings.Compare(a.Category, b.Category), cmp.Compare(a.Inbound, b.Inbound))
	})
	blocks := districts(sorted)
	colors := categoryColors(sorted)

	var area float64
	for _, d := range blocks {
		area += math.Pow(blockWidth(d)+streetWidth, 2)
	}
	rowLimit := math.Sqrt(area)

	var city City
	var x, z, rowDepth float64
	for _, d := range blocks {
		w := blockWidth(d)
		if x > 0 && x+w > rowLimit {
			x, z, rowDepth = 0, z+rowDepth+streetWidth, 0
		}
		city.Buildings = append(city.Buildings, placeBuildings(d, x, z, colors)...)
		block := Block{Name: d.name, Box: rl.BoundingBox{
			Min: rl.NewVector3(float32(x), 0, float32(z)),
			Max: rl.NewVector3(float32(x+w), 0, float32(z+w)),
		}}
		if d.hub != nil {
			block.Hub = d.hub.Name
		}
		city.Blocks = append(city.Blocks, block)
		x += w + streetWidth
		rowDepth = max(rowDepth, w)
	}

	city.Spawn, city.Facing = spawn(city.Blocks)
	return city
}

// districts groups notes sorted by name into districts, largest first and
// then by name, followed by the suburbs. Topics match notes the way links
// do, ignoring case.
func districts(notes []Note) []*district {
	byKey := make(map[string]*district)
	for _, n := range notes {
		if n.Topic == "" {
			continue
		}
		key := noteKey(n.Topic)
		if d, ok := byKey[key]; !ok {
			byKey[key] = &district{name: n.Topic}
		} else if n.Topic < d.name {
			d.name = n.Topic // the same topic spelled differently
		}
	}

	suburbs := &district{}
	for i := range notes {
		n := &notes[i]
		if d, isHub := byKey[noteKey(n.Name)]; isHub && d.hub == nil {
			d.hub, d.name = n, n.Name
		} else if n.Topic != "" {
			d := byKey[noteKey(n.Topic)]
			d.members = append(d.members, *n)
		} else {
			suburbs.members = append(suburbs.members, *n)
		}
	}

	var out []*district
	for _, d := range byKey {
		if d.size() > 0 { // empty when its only note left to be another hub
			out = append(out, d)
		}
	}
	slices.SortFunc(out, func(a, b *district) int {
		return cmp.Or(cmp.Compare(b.size(), a.size()), strings.Compare(a.name, b.name))
	})
	if suburbs.size() > 0 {
		out = append(out, suburbs)
	}
	return out
}

// side is the smallest odd number of lots per side that fits n lots.
func side(n int) int {
	s := 1
	for s*s < n {
		s += 2
	}
	return s
}

// blockWidth is a block's side from the first lot's edge to the last one's.
func blockWidth(d *district) float64 {
	return float64(side(d.size()))*lotPitch - alleyWidth
}

// placeBuildings puts the district's notes on its lots, the block's corner
// at (x, z): the hub on the center lot, the members on the others by name.
func placeBuildings(d *district, x, z float64, colors map[string]rl.Color) []Building {
	s := side(d.size())
	center := s * s / 2 // lots are numbered row by row
	out := make([]Building, 0, d.size())
	put := func(lot int, n Note) {
		cx := x + float64(lot%s)*lotPitch + lotSize/2
		cz := z + float64(lot/s)*lotPitch + lotSize/2
		out = append(out, Building{
			Name: n.Name,
			Box: rl.BoundingBox{
				Min: rl.NewVector3(float32(cx-lotSize/2), 0, float32(cz-lotSize/2)),
				Max: rl.NewVector3(float32(cx+lotSize/2), float32(height(n.Inbound)), float32(cz+lotSize/2)),
			},
			Color: colors[n.Category],
		})
	}
	lot := 0
	for _, n := range d.members {
		if d.hub != nil && lot == center {
			lot++ // the hub's
		}
		put(lot, n)
		lot++
	}
	if d.hub != nil {
		put(center, *d.hub)
	}
	return out
}

// height grows with the logarithm of the inbound links, so one very linked
// note doesn't dwarf the whole city.
func height(inbound int) float64 {
	return minHeight + heightPerDoubling*math.Log2(1+float64(inbound))
}

// categoryColors gives each category a palette color in name order.
func categoryColors(notes []Note) map[string]rl.Color {
	var names []string
	for _, n := range notes {
		if n.Category != "" {
			names = append(names, n.Category)
		}
	}
	slices.Sort(names)
	colors := map[string]rl.Color{"": uncategorized}
	for i, name := range slices.Compact(names) {
		colors[name] = categoryPalette[i%len(categoryPalette)]
	}
	return colors
}

// pushOut returns pos moved out of every building it has entered, so the
// player keeps playerRadius from each wall. It moves along the axis that
// needs the shortest push, so walking into a wall stops you and walking
// along it slides. Higher than playerRadius above a roof, nothing blocks.
func pushOut(pos rl.Vector3, buildings []Building) rl.Vector3 {
	const r = playerRadius
	for _, b := range buildings {
		if pos.Y >= b.Box.Max.Y+r {
			continue
		}
		toMinX := pos.X - (b.Box.Min.X - r) // how far each side must move to get out
		toMaxX := b.Box.Max.X + r - pos.X
		toMinZ := pos.Z - (b.Box.Min.Z - r)
		toMaxZ := b.Box.Max.Z + r - pos.Z
		if toMinX <= 0 || toMaxX <= 0 || toMinZ <= 0 || toMaxZ <= 0 {
			continue // outside, or exactly playerRadius from a wall
		}
		switch min(toMinX, toMaxX, toMinZ, toMaxZ) {
		case toMinX:
			pos.X = b.Box.Min.X - r
		case toMaxX:
			pos.X = b.Box.Max.X + r
		case toMinZ:
			pos.Z = b.Box.Min.Z - r
		default:
			pos.Z = b.Box.Max.Z + r
		}
	}
	return pos
}

// keepOut moves walker out of any building it entered (see pushOut) and
// other by the same amount, so the camera's view doesn't turn.
func keepOut(walker, other *rl.Vector3, buildings []Building) {
	moved := pushOut(*walker, buildings)
	*other = rl.Vector3Add(*other, rl.Vector3Subtract(moved, *walker))
	*walker = moved
}

// pick returns the building the ray hits first, if that is within
// pickRange, and the point where the ray hits it.
func pick(ray rl.Ray, buildings []Building) (b Building, at rl.Vector3, ok bool) {
	nearest := float32(pickRange)
	for _, c := range buildings {
		if hit := rl.GetRayCollisionBox(ray, c.Box); hit.Hit && hit.Distance <= nearest {
			b, at, ok, nearest = c, hit.Point, true, hit.Distance
		}
	}
	return b, at, ok
}

// labelPosition is where a label of the given size goes: centered just
// above the screen point, moved in to stay whole on the screen.
func labelPosition(at, size rl.Vector2, screenWidth, screenHeight float32) rl.Vector2 {
	x := at.X - size.X/2
	y := at.Y - labelGap - size.Y
	return rl.NewVector2(
		max(labelMargin, min(x, screenWidth-labelMargin-size.X)),
		max(labelMargin, min(y, screenHeight-labelMargin-size.Y)))
}

// labelRunes lists every character a label may need: printable ASCII and
// whatever the note names use, such as Polish letters, Cyrillic or "—".
func labelRunes(notes []Note) []rune {
	var runes []rune
	for r := ' '; r <= '~'; r++ {
		runes = append(runes, r)
	}
	for _, n := range notes {
		runes = append(runes, []rune(n.Name)...)
	}
	slices.Sort(runes)
	return slices.Compact(runes)
}

// spawn returns a point on the street in front of the first district block
// (the largest), or of the suburbs when there are no districts, and the
// direction toward its center lot.
func spawn(blocks []Block) (at, facing rl.Vector3) {
	if len(blocks) == 0 {
		return rl.Vector3{}, rl.NewVector3(0, 0, 1)
	}
	b := blocks[0] // districts come first, so this is the suburbs only when they're alone
	center := rl.NewVector3((b.Box.Min.X+b.Box.Max.X)/2, 0, (b.Box.Min.Z+b.Box.Max.Z)/2)
	at = rl.NewVector3(center.X, 0, b.Box.Min.Z-streetWidth/2)
	return at, rl.Vector3Normalize(rl.Vector3Subtract(center, at))
}
