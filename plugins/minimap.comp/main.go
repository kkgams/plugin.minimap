//go:generate go tool wit-bindgen-go generate --world generator --out internal ./gams:minimap@1.0.0.wasm

package main

import (
	"sort"

	"github.com/kkgams/minimap/placement"
	minimapwit "github.com/kkgams/minimap/internal/gams/minimap/minimap"
	"github.com/kkgams/sdk/go/tilemap"
	"github.com/kkgams/sdk/go/tree"
	"go.bytecodealliance.org/cm"
)

type MyRandom struct{}

func (rng *MyRandom) Intn(n int) int {
	return 0
}

func init() {
	minimapwit.Exports.Gen = func(input minimapwit.Tree) cm.Result[minimapwit.TileMapShape, minimapwit.TileMap, string] {
		t := fromWITTree(input)

		gen := placement.NewGenerator(&t, getRoomShape, &MyRandom{})
		p, err := gen.Generate()
		if err != nil {
			return cm.Err[cm.Result[minimapwit.TileMapShape, minimapwit.TileMap, string]](err.Error())
		}

		return cm.OK[cm.Result[minimapwit.TileMapShape, minimapwit.TileMap, string]](toWITTileMap(p.ToTileMap()))
	}
}

func fromWITTree(input minimapwit.Tree) tree.Tree {
	witNodes := input.Slice()
	t := make(tree.Tree, len(witNodes))
	for i, node := range witNodes {
		t[i] = &tree.Node{
			Data:     fromWITData(node.Data),
			ParentId: int(node.ParentID),
		}
	}
	return t
}

func fromWITData(entries cm.List[minimapwit.DataEntry]) map[string]string {
	data := make(map[string]string, entries.Len())
	for _, entry := range entries.Slice() {
		data[entry[0]] = entry[1]
	}
	return data
}

func toWITTileMap(tm *tilemap.TileMap) minimapwit.TileMap {
	layers := make([]minimapwit.TileLayer, len(tm.Layers))
	for i, layer := range tm.Layers {
		layers[i] = minimapwit.TileLayer{
			Width: uint32(layer.Width),
			Data:  cm.ToList(layer.Data),
			Props: toWITData(layer.Props),
		}
	}
	return minimapwit.TileMap{
		Layers: cm.ToList(layers),
		Props:  toWITData(tm.Props),
	}
}

func toWITData(data map[string]string) cm.List[minimapwit.DataEntry] {
	entries := make([]minimapwit.DataEntry, 0, len(data))
	keys := make([]string, 0, len(data))
	for key := range data {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		entries = append(entries, minimapwit.DataEntry{key, data[key]})
	}
	return cm.ToList(entries)
}

func getRoomShape(node *tree.Node) (placement.RoomShape, error) {
	if room, ok := node.Data["minimap"]; ok {
		return placement.ParseRoomShapeMask(room)
	}

	rng := &MyRandom{}
	idx := rng.Intn(len(roomShapesToChooseFrom))
	return roomShapesToChooseFrom[idx], nil
}

var roomShapesToChooseFrom = []placement.RoomShape{
	{{X: 0, Y: 0}},
	{{X: 0, Y: 0}, {X: 0, Y: -1}},
	{{X: 0, Y: 0}, {X: 0, Y: 1}},
	{{X: 0, Y: 0}, {X: 1, Y: 0}},
	{{X: 0, Y: 0}, {X: -1, Y: 0}},
	{{X: 0, Y: 0}, {X: 1, Y: 0}, {X: 0, Y: 1}},
	{{X: 0, Y: 0}, {X: -1, Y: 0}, {X: 0, Y: 1}},
	{{X: 0, Y: 0}, {X: 1, Y: 0}, {X: 0, Y: 1}, {X: 1, Y: 1}},
	{{X: 0, Y: 0}, {X: 1, Y: 0}, {X: 0, Y: 1}, {X: 1, Y: 1}, {X: 0, Y: 2}, {X: 1, Y: 2}},
	{{X: 0, Y: 0}, {X: 0, Y: 1}, {X: 1, Y: 0}, {X: 1, Y: 1}, {X: 2, Y: 0}, {X: 2, Y: 1}},
	{{X: 0, Y: 0}, {X: 1, Y: 0}, {X: 0, Y: 1}, {X: 0, Y: 2}},
	{{X: 0, Y: 0}, {X: 1, Y: 0}, {X: 1, Y: 1}, {X: 1, Y: 2}},
	{{X: 0, Y: 0}, {X: 0, Y: 1}, {X: 1, Y: 1}, {X: 2, Y: 1}},
	{{X: 0, Y: 0}, {X: 0, Y: 1}, {X: -1, Y: 1}, {X: -2, Y: 1}},
	{{X: 0, Y: 0}, {X: 1, Y: 0}, {X: 2, Y: 0}, {X: 1, Y: 1}},
	{{X: 0, Y: 0}, {X: 0, Y: 1}, {X: 0, Y: 2}, {X: -1, Y: 1}},
}

func main() {}
