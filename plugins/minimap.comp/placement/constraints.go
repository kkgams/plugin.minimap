package placement

// ConstraintPropagation implements Nonogram/Sudoku-style constraint propagation
// for escape path reservations.
//
// Key insight: If a room has only ONE path to outside, ALL tiles in that path
// are "forced" and must be reserved. If multiple paths exist, only tiles that
// appear in ALL paths are forced.

// EscapePath represents a single path from a room to outside
type EscapePath struct {
	Tiles []Point // Tiles in the path (excluding room tiles and outside)
}

// ConstraintSystem manages escape path constraints for all unfinished rooms
type ConstraintSystem struct {
	// ForcedTiles are tiles that MUST remain free for at least one room
	// Maps tile -> set of room IDs that require this tile
	ForcedTiles map[Point]map[RoomID]bool

	// RoomPaths stores all known escape paths for each unfinished room
	RoomPaths map[RoomID][]EscapePath

	// RoomForcedTiles stores which tiles are forced for each room
	// (tiles that appear in ALL of that room's escape paths)
	RoomForcedTiles map[RoomID]map[Point]bool
}

// NewConstraintSystem creates a new constraint system
func NewConstraintSystem() *ConstraintSystem {
	return &ConstraintSystem{
		ForcedTiles:     make(map[Point]map[RoomID]bool),
		RoomPaths:       make(map[RoomID][]EscapePath),
		RoomForcedTiles: make(map[RoomID]map[Point]bool),
	}
}

// EnumerateEscapePaths finds distinct escape paths from a room to outside.
// Uses BFS with depth limit to find paths efficiently.
// Returns paths ordered by length (shortest first).
func (p *Placement) EnumerateEscapePaths(room *PlacedRoom, maxPaths int) []EscapePath {
	freeEdges := p.GetFreeEdges(room)
	if len(freeEdges) == 0 {
		return nil
	}

	// Get bounding box with margin for "outside"
	minX, minY, maxX, maxY := p.GetBoundingBox()
	outsideMinX, outsideMinY := minX-1, minY-1
	outsideMaxX, outsideMaxY := maxX+1, maxY+1

	// Limit search depth to prevent explosion
	maxDepth := 10

	// Track found paths
	paths := make([]EscapePath, 0, maxPaths)

	// BFS state: each state tracks the path taken to reach that point
	type searchState struct {
		pos   Point
		path  []Point
		depth int
	}

	// Start from all free edges
	visited := make(map[Point]bool)
	queue := make([]searchState, 0)

	// Mark room tiles as visited (can't path through room)
	for _, tile := range room.CurrentShape {
		visited[tile] = true
	}

	// Initialize queue with neighbors of free edges
	for _, edge := range freeEdges {
		if !visited[edge.Neighbor] && !p.IsOccupied(edge.Neighbor) {
			visited[edge.Neighbor] = true
			queue = append(queue, searchState{
				pos:   edge.Neighbor,
				path:  []Point{edge.Neighbor},
				depth: 1,
			})
		}
	}

	// BFS to find paths
	for len(queue) > 0 && len(paths) < maxPaths {
		current := queue[0]
		queue = queue[1:]

		// Check if we reached outside
		if current.pos.X <= outsideMinX || current.pos.X >= outsideMaxX ||
			current.pos.Y <= outsideMinY || current.pos.Y >= outsideMaxY {
			// Found a path! Don't include the outside tile itself
			pathTiles := make([]Point, len(current.path)-1)
			copy(pathTiles, current.path[:len(current.path)-1])
			paths = append(paths, EscapePath{Tiles: pathTiles})
			continue
		}

		// Don't expand beyond max depth
		if current.depth >= maxDepth {
			continue
		}

		// Expand to neighbors
		for _, neighbor := range current.pos.Neighbors() {
			if visited[neighbor] || p.IsOccupied(neighbor) {
				continue
			}
			visited[neighbor] = true

			newPath := make([]Point, len(current.path)+1)
			copy(newPath, current.path)
			newPath[len(current.path)] = neighbor

			queue = append(queue, searchState{
				pos:   neighbor,
				path:  newPath,
				depth: current.depth + 1,
			})
		}
	}

	return paths
}

// EnumerateAllDistinctPaths finds all distinct paths, where "distinct" means
// they don't share the first tile (the tile immediately adjacent to the room).
// This better captures true path independence.
func (p *Placement) EnumerateDistinctPaths(room *PlacedRoom, maxPathsPerEdge int) []EscapePath {
	freeEdges := p.GetFreeEdges(room)
	if len(freeEdges) == 0 {
		return nil
	}

	// Get bounding box with margin for "outside"
	minX, minY, maxX, maxY := p.GetBoundingBox()
	outsideMinX, outsideMinY := minX-1, minY-1
	outsideMaxX, outsideMaxY := maxX+1, maxY+1

	// Track found paths
	allPaths := make([]EscapePath, 0)

	// For each free edge, find paths starting from that edge
	for _, edge := range freeEdges {
		if p.IsOccupied(edge.Neighbor) {
			continue
		}

		edgePaths := p.findPathsFromPoint(
			edge.Neighbor,
			room.CurrentShape,
			outsideMinX, outsideMinY, outsideMaxX, outsideMaxY,
			maxPathsPerEdge,
		)
		allPaths = append(allPaths, edgePaths...)
	}

	return allPaths
}

// findPathsFromPoint finds paths from a starting point to outside
func (p *Placement) findPathsFromPoint(
	start Point,
	roomTiles []Point,
	outsideMinX, outsideMinY, outsideMaxX, outsideMaxY int,
	maxPaths int,
) []EscapePath {
	paths := make([]EscapePath, 0)

	// Build room tile set for quick lookup
	roomSet := make(map[Point]bool)
	for _, t := range roomTiles {
		roomSet[t] = true
	}

	// DFS with path tracking
	type searchState struct {
		pos     Point
		path    []Point
		visited map[Point]bool
	}

	initial := searchState{
		pos:     start,
		path:    []Point{start},
		visited: make(map[Point]bool),
	}
	initial.visited[start] = true
	for _, t := range roomTiles {
		initial.visited[t] = true
	}

	stack := []searchState{initial}

	for len(stack) > 0 && len(paths) < maxPaths {
		current := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		// Check if we reached outside
		if current.pos.X <= outsideMinX || current.pos.X >= outsideMaxX ||
			current.pos.Y <= outsideMinY || current.pos.Y >= outsideMaxY {
			// Found a path! Don't include the outside tile
			pathTiles := make([]Point, len(current.path)-1)
			copy(pathTiles, current.path[:len(current.path)-1])
			paths = append(paths, EscapePath{Tiles: pathTiles})
			continue
		}

		// Expand to neighbors
		for _, neighbor := range current.pos.Neighbors() {
			if current.visited[neighbor] || p.IsOccupied(neighbor) {
				continue
			}

			// Clone visited map and path
			newVisited := make(map[Point]bool, len(current.visited)+1)
			for k, v := range current.visited {
				newVisited[k] = v
			}
			newVisited[neighbor] = true

			newPath := make([]Point, len(current.path)+1)
			copy(newPath, current.path)
			newPath[len(current.path)] = neighbor

			stack = append(stack, searchState{
				pos:     neighbor,
				path:    newPath,
				visited: newVisited,
			})
		}
	}

	return paths
}

// FindForcedTiles finds tiles that appear in ALL escape paths for a room.
// If a room has no paths, returns nil (room is blocked).
// If a room has paths but no forced tiles, returns empty slice.
func FindForcedTiles(paths []EscapePath) []Point {
	if len(paths) == 0 {
		return nil // No paths = blocked
	}

	if len(paths) == 1 {
		// Single path - all tiles in it are forced
		return paths[0].Tiles
	}

	// Count occurrences of each tile across all paths
	tileCounts := make(map[Point]int)
	for _, path := range paths {
		for _, tile := range path.Tiles {
			tileCounts[tile]++
		}
	}

	// Tiles that appear in ALL paths are forced
	numPaths := len(paths)
	forced := make([]Point, 0)
	for tile, count := range tileCounts {
		if count == numPaths {
			forced = append(forced, tile)
		}
	}

	return forced
}

// Propagate runs constraint propagation for all unfinished rooms
// Returns true if propagation succeeded, false if a conflict was detected
func (cs *ConstraintSystem) Propagate(p *Placement, maxPathsPerRoom int) bool {
	// Clear previous state
	cs.ForcedTiles = make(map[Point]map[RoomID]bool)
	cs.RoomPaths = make(map[RoomID][]EscapePath)
	cs.RoomForcedTiles = make(map[RoomID]map[Point]bool)

	// For each unfinished room, enumerate paths and find forced tiles
	for roomID := range p.Unfinished {
		room := p.Rooms[roomID]
		if room == nil {
			continue
		}

		// Enumerate escape paths
		paths := p.EnumerateEscapePaths(room, maxPathsPerRoom)
		cs.RoomPaths[roomID] = paths

		// If room has no paths, propagation failed
		if len(paths) == 0 {
			return false
		}

		// Find forced tiles for this room
		forced := FindForcedTiles(paths)
		cs.RoomForcedTiles[roomID] = make(map[Point]bool)

		for _, tile := range forced {
			cs.RoomForcedTiles[roomID][tile] = true

			// Add to global forced tiles
			if cs.ForcedTiles[tile] == nil {
				cs.ForcedTiles[tile] = make(map[RoomID]bool)
			}
			cs.ForcedTiles[tile][roomID] = true
		}
	}

	return true
}

// HasConflict checks if any two rooms share all their forced tiles.
// This would mean they both need the same escape path, which is impossible.
func (cs *ConstraintSystem) HasConflict() bool {
	roomIDs := make([]RoomID, 0, len(cs.RoomForcedTiles))
	for id := range cs.RoomForcedTiles {
		roomIDs = append(roomIDs, id)
	}

	for i := 0; i < len(roomIDs); i++ {
		forcedA := cs.RoomForcedTiles[roomIDs[i]]
		if len(forcedA) == 0 {
			continue // Room has multiple escape options, no conflict possible
		}

		for j := i + 1; j < len(roomIDs); j++ {
			forcedB := cs.RoomForcedTiles[roomIDs[j]]
			if len(forcedB) == 0 {
				continue
			}

			// Check if forced tiles overlap
			// A conflict exists if both rooms have forced tiles and they share any
			for tile := range forcedA {
				if forcedB[tile] {
					return true // Both rooms need this tile - conflict!
				}
			}
		}
	}

	return false
}

// IsForcedTile checks if a tile is forced (must remain free)
func (cs *ConstraintSystem) IsForcedTile(p Point) bool {
	return len(cs.ForcedTiles[p]) > 0
}

// IsForcedBy checks if a tile is forced by a specific room
func (cs *ConstraintSystem) IsForcedBy(p Point, roomID RoomID) bool {
	rooms := cs.ForcedTiles[p]
	return rooms != nil && rooms[roomID]
}

// CanPlaceTiles checks if placing tiles would violate any constraints.
// placingRoomID is the room being placed (can use its own forced tiles).
// parentRoomID is the parent room (can use its forced tiles for extension).
func (cs *ConstraintSystem) CanPlaceTiles(tiles []Point, placingRoomID, parentRoomID RoomID) bool {
	for _, tile := range tiles {
		rooms := cs.ForcedTiles[tile]
		if rooms == nil {
			continue // Not forced
		}

		// Check if forced by any room other than placing room or its parent
		for roomID := range rooms {
			if roomID != placingRoomID && roomID != parentRoomID {
				return false // Would block another room's forced path
			}
		}
	}
	return true
}

// SimulatePlacement checks if placing tiles would leave all unfinished rooms
// with at least one escape path. This is more thorough than CanPlaceTiles
// as it actually simulates the placement.
func (cs *ConstraintSystem) SimulatePlacement(p *Placement, tiles []Point, tempRoomID RoomID, maxPaths int) bool {
	// Temporarily place tiles
	for _, tile := range tiles {
		p.Grid[tile] = tempRoomID
	}

	// Check each unfinished room still has paths
	allHavePaths := true
	for roomID := range p.Unfinished {
		room := p.Rooms[roomID]
		if room == nil {
			continue
		}

		paths := p.EnumerateEscapePaths(room, 1) // Just need to find ONE path
		if len(paths) == 0 {
			allHavePaths = false
			break
		}
	}

	// Remove temporary tiles
	for _, tile := range tiles {
		delete(p.Grid, tile)
	}

	return allHavePaths
}

// CheckNoForcedConflicts checks if placement would create a situation where
// two different rooms share the same forced tiles (escape path conflict).
// Returns true if no conflict would arise.
func (cs *ConstraintSystem) CheckNoForcedConflicts(p *Placement, tiles []Point, tempRoomID RoomID, maxPaths int) bool {
	// Temporarily place tiles
	for _, tile := range tiles {
		p.Grid[tile] = tempRoomID
	}

	// Re-enumerate paths for all unfinished rooms and check for conflicts
	forcedByRoom := make(map[RoomID]map[Point]bool)

	for roomID := range p.Unfinished {
		room := p.Rooms[roomID]
		if room == nil {
			continue
		}

		paths := p.EnumerateEscapePaths(room, maxPaths)
		if len(paths) == 0 {
			// Room has no escape - definitely a problem
			for _, tile := range tiles {
				delete(p.Grid, tile)
			}
			return false
		}

		forced := FindForcedTiles(paths)
		forcedByRoom[roomID] = make(map[Point]bool)
		for _, f := range forced {
			forcedByRoom[roomID][f] = true
		}
	}

	// Check if any two rooms share forced tiles (conflict)
	// A conflict means both rooms MUST use the same tile, which is impossible
	// when one room extends through it (blocks the other)
	hasConflict := false

	roomIDs := make([]RoomID, 0, len(forcedByRoom))
	for id := range forcedByRoom {
		roomIDs = append(roomIDs, id)
	}

	for i := 0; i < len(roomIDs); i++ {
		for j := i + 1; j < len(roomIDs); j++ {
			roomA := roomIDs[i]
			roomB := roomIDs[j]

			// Check if A and B share any forced tiles
			for tile := range forcedByRoom[roomA] {
				if forcedByRoom[roomB][tile] {
					// Both rooms force this tile - they conflict!
					hasConflict = true
					break
				}
			}
			if hasConflict {
				break
			}
		}
		if hasConflict {
			break
		}
	}

	// Remove temporary tiles
	for _, tile := range tiles {
		delete(p.Grid, tile)
	}

	return !hasConflict
}

// GetRoomPathCount returns the number of escape paths for a room
func (cs *ConstraintSystem) GetRoomPathCount(roomID RoomID) int {
	return len(cs.RoomPaths[roomID])
}

// GetForcedTilesForRoom returns all forced tiles for a specific room
func (cs *ConstraintSystem) GetForcedTilesForRoom(roomID RoomID) []Point {
	forced := cs.RoomForcedTiles[roomID]
	result := make([]Point, 0, len(forced))
	for tile := range forced {
		result = append(result, tile)
	}
	return result
}
