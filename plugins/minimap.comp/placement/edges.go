package placement

// GetFreeEdges returns all edges of a room that face empty space
func (p *Placement) GetFreeEdges(room *PlacedRoom) []Edge {
	edges := make([]Edge, 0)

	for _, tile := range room.CurrentShape {
		// Check all 4 directions
		for dir := 0; dir < 4; dir++ {
			dx, dy := DirectionOffset(dir)
			neighbor := tile.Add(dx, dy)

			// If neighbor is empty (not occupied by any room), it's a free edge
			if !p.IsOccupied(neighbor) {
				edges = append(edges, Edge{
					Position:  tile,
					Direction: dir,
					Neighbor:  neighbor,
				})
			}
		}
	}

	return edges
}

// GetValidExtensionTiles returns tiles that can be added to extend a room
// Valid extension tiles are:
// - Adjacent to current room shape
// - Not occupied by any room
func (p *Placement) GetValidExtensionTiles(room *PlacedRoom) []Point {
	seen := make(map[Point]bool)
	extensions := make([]Point, 0)

	// Mark all current room tiles as seen (can't extend into ourselves)
	for _, tile := range room.CurrentShape {
		seen[tile] = true
	}

	// Check neighbors of each tile in current shape
	for _, tile := range room.CurrentShape {
		for _, neighbor := range tile.Neighbors() {
			if seen[neighbor] {
				continue
			}
			seen[neighbor] = true

			if !p.IsOccupied(neighbor) {
				extensions = append(extensions, neighbor)
			}
		}
	}

	return extensions
}

// HasPathToOutside checks if a room has at least one free edge that can reach
// "outside" (beyond the bounding box of all placed tiles) via empty tiles.
// This guarantees the room can be extended indefinitely if needed.
func (p *Placement) HasPathToOutside(room *PlacedRoom) bool {
	freeEdges := p.GetFreeEdges(room)
	if len(freeEdges) == 0 {
		return false
	}

	// Get bounding box with margin (outside = beyond this)
	minX, minY, maxX, maxY := p.GetBoundingBox()

	// For each free edge, try to reach outside via BFS through empty tiles
	for _, edge := range freeEdges {
		if p.canReachOutside(edge.Neighbor, minX-1, minY-1, maxX+1, maxY+1) {
			return true
		}
	}

	return false
}

// canReachOutside performs BFS from start through empty tiles to reach outside the bounding box
func (p *Placement) canReachOutside(start Point, minX, minY, maxX, maxY int) bool {
	// If start is already outside, we're done
	if start.X <= minX || start.X >= maxX || start.Y <= minY || start.Y >= maxY {
		return true
	}

	visited := make(map[Point]bool)
	queue := []Point{start}
	visited[start] = true

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		// Check if we've reached outside
		if current.X <= minX || current.X >= maxX || current.Y <= minY || current.Y >= maxY {
			return true
		}

		// Explore neighbors
		for _, neighbor := range current.Neighbors() {
			if visited[neighbor] {
				continue
			}
			visited[neighbor] = true

			// Only traverse through empty tiles
			if !p.IsOccupied(neighbor) {
				queue = append(queue, neighbor)
			}
		}
	}

	return false
}

// AllUnfinishedHavePathToOutside checks if all unfinished rooms have a path to outside
func (p *Placement) AllUnfinishedHavePathToOutside() bool {
	for roomID := range p.Unfinished {
		room := p.Rooms[roomID]
		if room == nil {
			continue
		}
		if !p.HasPathToOutside(room) {
			return false
		}
	}
	return true
}

// WouldBlockAnyUnfinished checks if placing tiles would block any unfinished room's path to outside
// tempTiles are temporarily added to check, then removed
func (p *Placement) WouldBlockAnyUnfinished(tempTiles []Point, tempRoomID RoomID) bool {
	// Temporarily add tiles
	for _, tile := range tempTiles {
		p.Grid[tile] = tempRoomID
	}

	// Check all unfinished rooms
	blocked := false
	for roomID := range p.Unfinished {
		room := p.Rooms[roomID]
		if room == nil {
			continue
		}
		if !p.HasPathToOutside(room) {
			blocked = true
			break
		}
	}

	// Remove temporary tiles
	for _, tile := range tempTiles {
		delete(p.Grid, tile)
	}

	return blocked
}
