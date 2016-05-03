package tiles

import (
	"sort"
	"strings"
	"sync"
)

// TileIndex stores indexes values by tile. If a deep level of tile is added and a shallower
// one is requested, the values are aggregated up.
// TileIndex is thread safe
// implementation uses a sorted keyset, trie would be better
type TileIndex struct {
	sorted bool
	keys   []qkey
	values []interface{}
	sync.RWMutex
}

// TileRange returns a channel of all tiles in the index in the zoom range
// If zmax is greater than the deepest tile level, the deepest tile level returns
// Acquires a readlock for duration of returned channel being open
func (idx *TileIndex) TileRange(zmin, zmax int) <-chan Tile {
	idx.sort()
	tiles := make(chan Tile, 1<<10)
	go func() {
		defer close(tiles)
		idx.RLock()
		defer idx.RUnlock()
		for i := 0; i < len(idx.keys)-1; i++ {
			q := idx.keys[i].qk
			n := idx.keys[i+1].qk
			for z := zmin; z <= zmax && z <= len(q); z++ {
				if !strings.HasPrefix(n, q[:z]) {
					tiles <- TileFromQuadKey(q[:z])
				}
			}
		}
		q := idx.keys[len(idx.keys)-1].qk
		for z := zmin; z <= zmax && z <= len(q); z++ {
			tiles <- TileFromQuadKey(q[:z])
		}
	}()
	return tiles
}

// Values returns a list of values aggregated under the requested tile
func (idx *TileIndex) Values(t Tile) (vals []interface{}) {
	idx.sort()
	idx.RLock()
	defer idx.RUnlock()
	qk := t.QuadKey()
	i := idx.search(qk)
	if i >= len(idx.keys) {
		return //404
	}
	n := idx.keys[i]
	for i < len(idx.keys) && strings.HasPrefix(n.qk, qk) {
		n = idx.keys[i]
		vals = append(vals, idx.values[n.v])
		i++
	}
	return
}

// Add adds a value, but will not be indexed
func (idx *TileIndex) Add(t Tile, val interface{}) {
	idx.Lock()
	defer idx.Unlock()
	idx.values = append(idx.values, val)
	qk := qkey{qk: t.QuadKey(), v: len(idx.values) - 1}
	idx.keys = append(idx.keys, qk)
	idx.sorted = false
}

// sorts the tiles, nothing happens if the sorted flag is set
func (idx *TileIndex) sort() {
	if !idx.sorted {
		idx.Lock()
		sort.Sort(byQk(idx.keys))
		idx.sorted = true
		idx.Unlock()
	}
}

func (idx *TileIndex) search(qk string) int {
	return sort.Search(len(idx.keys), func(i int) bool { return idx.keys[i].qk >= qk })
}

type qkey struct {
	qk string
	v  int
}

type byQk []qkey

func (q byQk) Len() int           { return len(q) }
func (q byQk) Swap(i, j int)      { q[i], q[j] = q[j], q[i] }
func (q byQk) Less(i, j int) bool { return q[i].qk < q[j].qk }