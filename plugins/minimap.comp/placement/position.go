package placement

// FindTouchingPositions finds all positions where a shape can be placed such that
// it touches the parent room (shares at least one edge) and doesn't collide with
// any existing room.
func (p *Placement) FindTouchingPositions(parent *PlacedRoom, shape RoomShape) []Point {
	positions := make([]Point, 0)
	seen := make(map[Point]bool)

	// For each free edge of parent, try placing shape so it touches that edge
	freeEdges := p.GetFreeEdges(parent)

	for _, edge := range freeEdges {
		// For each tile in shape, try aligning it to touch this edge
		for _, shapeTile := range shape {
			// Calculate offset so that shapeTile ends up at edge.Neighbor
			// (the empty space adjacent to parent)
			offset := Point{
				X: edge.Neighbor.X - shapeTile.X,
				Y: edge.Neighbor.Y - shapeTile.Y,
			}

			if seen[offset] {
				continue
			}
			seen[offset] = true

			// Check if shape at this offset is valid (no collisions, touches parent)
			if p.isValidPlacement(parent, shape, offset) {
				positions = append(positions, offset)
			}
		}
	}

	return positions
}

// isValidPlacement checks if placing shape at offset is valid:
// - No tile collides with existing rooms
// - At least one tile is adjacent to parent
func (p *Placement) isValidPlacement(parent *PlacedRoom, shape RoomShape, offset Point) bool {
	absoluteTiles := shape.Translate(offset)

	// Check for collisions
	for _, tile := range absoluteTiles {
		if p.IsOccupied(tile) {
			return false
		}
	}

	// Check that at least one tile touches parent
	touchesParent := false
	for _, tile := range absoluteTiles {
		for _, neighbor := range tile.Neighbors() {
			if parent.ContainsTile(neighbor) {
				touchesParent = true
				break
			}
		}
		if touchesParent {
			break
		}
	}

	return touchesParent
}

// FilterValidPositions filters positions to only those where:
// 1. No tiles are placed on fully reserved (critical) tiles
// 2. After placing, all unfinished rooms still have path to outside
// 3. If the child has children, it must also have path to outside
func (p *Placement) FilterValidPositions(
	positions []Point,
	childShape RoomShape,
	childNodeIndex int,
	childHasChildren bool,
) []Point {
	// For backward compatibility, call the version without reservations
	return p.FilterValidPositionsWithReservations(positions, childShape, childNodeIndex, childHasChildren, nil)
}

// FilterValidPositionsWithReservations filters positions using reservation system
func (p *Placement) FilterValidPositionsWithReservations(
	positions []Point,
	childShape RoomShape,
	childNodeIndex int,
	childHasChildren bool,
	rs *ReservationSystem,
) []Point {
	valid := make([]Point, 0)
	childRoomID := RoomID(childNodeIndex + 1)

	for _, pos := range positions {
		absoluteTiles := childShape.Translate(pos)

		// Check if any tile would be placed on a fully reserved tile
		if rs != nil {
			blocksReservation := false
			for _, tile := range absoluteTiles {
				if rs.IsFullyReserved(tile) {
					// Check if we're the only one who reserved it (our parent extending)
					res := rs.GetReservation(tile)
					if res != nil && len(res.RoomIDs) > 0 {
						// Check if it's reserved by rooms other than our ancestors
						// For now, just reject if any room reserves it
						blocksReservation = true
						break
					}
				}
			}
			if blocksReservation {
				continue
			}
		}

		// Temporarily place child
		for _, tile := range absoluteTiles {
			p.Grid[tile] = childRoomID
		}

		// Create temporary room for checking
		tempRoom := &PlacedRoom{
			NodeIndex:    childNodeIndex,
			CurrentShape: absoluteTiles,
		}
		p.Rooms[childRoomID] = tempRoom

		// Check all unfinished rooms still have path to outside
		allValid := true
		for roomID := range p.Unfinished {
			room := p.Rooms[roomID]
			if room == nil {
				continue
			}
			if !p.HasPathToOutside(room) {
				allValid = false
				break
			}
		}

		// If child has children, it must also have path to outside and room to grow
		if allValid && childHasChildren {
			if !p.HasPathToOutside(tempRoom) {
				allValid = false
			} else {
				extensionTiles := p.GetValidExtensionTiles(tempRoom)
				if len(extensionTiles) < 2 {
					allValid = false
				}
			}
		}

		// Remove temporary placement
		for _, tile := range absoluteTiles {
			delete(p.Grid, tile)
		}
		delete(p.Rooms, childRoomID)

		if allValid {
			valid = append(valid, pos)
		}
	}

	return valid
}
