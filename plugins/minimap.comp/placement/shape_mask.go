package placement

import (
	"fmt"
	"strings"
)

// ParseRoomShapeMask parses a compact room-shape mask such as "##./#../...".
// Rows are separated by '/', '#' marks an occupied room cell, and '.' marks an
// empty cell. The top-left cell is coordinate (0,0), x increases to the right,
// and y increases downward.
func ParseRoomShapeMask(input string) (RoomShape, error) {
	mask := strings.TrimSpace(input)
	if mask == "" {
		return nil, fmt.Errorf("minimap mask is empty")
	}
	if strings.Contains(mask, "\n") || strings.Contains(mask, "\r") {
		return nil, fmt.Errorf("minimap mask must use '/' row separators")
	}

	rows := strings.Split(mask, "/")
	width := len(rows[0])
	if width == 0 {
		return nil, fmt.Errorf("minimap mask row 0 is empty")
	}

	shape := make(RoomShape, 0)
	seen := make(map[Point]bool)
	for y, row := range rows {
		if len(row) == 0 {
			return nil, fmt.Errorf("minimap mask row %d is empty", y)
		}
		if len(row) != width {
			return nil, fmt.Errorf("minimap mask row %d width %d does not match row 0 width %d", y, len(row), width)
		}
		for x, ch := range row {
			switch ch {
			case '#':
				p := Point{X: x, Y: y}
				if seen[p] {
					return nil, fmt.Errorf("minimap mask contains duplicate cell %d,%d", x, y)
				}
				seen[p] = true
				shape = append(shape, p)
			case '.':
				// empty cell
			default:
				return nil, fmt.Errorf("minimap mask contains invalid character %q at %d,%d", ch, x, y)
			}
		}
	}

	if len(shape) == 0 {
		return nil, fmt.Errorf("minimap mask must contain at least one occupied cell")
	}
	if !roomShapeConnected(shape) {
		return nil, fmt.Errorf("minimap mask occupied cells must be 4-connected")
	}

	return shape, nil
}

func roomShapeConnected(shape RoomShape) bool {
	if len(shape) == 0 {
		return false
	}
	cells := make(map[Point]bool, len(shape))
	for _, p := range shape {
		cells[p] = true
	}

	visited := make(map[Point]bool, len(shape))
	queue := []Point{shape[0]}
	visited[shape[0]] = true
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, neighbor := range current.Neighbors() {
			if !cells[neighbor] || visited[neighbor] {
				continue
			}
			visited[neighbor] = true
			queue = append(queue, neighbor)
		}
	}

	return len(visited) == len(shape)
}
