package placement

// ReservationType indicates how critical a tile reservation is
type ReservationType int

const (
	// NotReserved - tile is free to use
	NotReserved ReservationType = iota
	// PartiallyReserved - tile is part of a path but alternatives exist
	PartiallyReserved
	// FullyReserved - tile is a bottleneck, must remain free
	FullyReserved
)

// Reservation tracks which rooms need a tile to remain free
type Reservation struct {
	Type    ReservationType
	RoomIDs map[RoomID]bool // Which rooms have reserved this tile
}

// ReservationSystem manages path reservations for unfinished rooms
type ReservationSystem struct {
	// Reserved maps tiles to their reservation status
	Reserved map[Point]*Reservation
}

// NewReservationSystem creates a new reservation system
func NewReservationSystem() *ReservationSystem {
	return &ReservationSystem{
		Reserved: make(map[Point]*Reservation),
	}
}

// IsFullyReserved checks if a tile is a critical bottleneck for any room
func (rs *ReservationSystem) IsFullyReserved(p Point) bool {
	r, ok := rs.Reserved[p]
	return ok && r.Type == FullyReserved
}

// IsReservedBy checks if a tile is reserved by a specific room
func (rs *ReservationSystem) IsReservedBy(p Point, roomID RoomID) bool {
	r, ok := rs.Reserved[p]
	if !ok {
		return false
	}
	return r.RoomIDs[roomID]
}

// GetReservation returns the reservation for a tile, or nil if not reserved
func (rs *ReservationSystem) GetReservation(p Point) *Reservation {
	return rs.Reserved[p]
}

// CanPlaceTile checks if a tile can be placed without blocking critical paths
// It returns true if the tile is not fully reserved, or if it's only reserved
// by the room that's being extended (parentRoomID)
func (rs *ReservationSystem) CanPlaceTile(p Point, parentRoomID RoomID) bool {
	r, ok := rs.Reserved[p]
	if !ok {
		return true // Not reserved at all
	}

	if r.Type != FullyReserved {
		return true // Only partially reserved, can still use
	}

	// Fully reserved - only ok if reserved solely by the parent being extended
	if len(r.RoomIDs) == 1 && r.RoomIDs[parentRoomID] {
		return true
	}

	return false
}

// FindCriticalTiles finds the bottleneck tiles that MUST remain free for a room
// to have any path to outside. These are "articulation points" in the graph of
// empty tiles between the room and the outside.
func (p *Placement) FindCriticalTiles(room *PlacedRoom) []Point {
	freeEdges := p.GetFreeEdges(room)
	if len(freeEdges) == 0 {
		return nil
	}

	// Get bounding box with margin for "outside"
	minX, minY, maxX, maxY := p.GetBoundingBox()
	outsideMinX, outsideMinY := minX-1, minY-1
	outsideMaxX, outsideMaxY := maxX+1, maxY+1

	// Find reachable empty tiles between room and outside using BFS
	// We stop at the boundary (outside)
	reachable := make(map[Point]bool)
	queue := make([]Point, 0)

	// Start BFS from each free edge
	for _, edge := range freeEdges {
		if !reachable[edge.Neighbor] && !p.IsOccupied(edge.Neighbor) {
			reachable[edge.Neighbor] = true
			queue = append(queue, edge.Neighbor)
		}
	}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		// Stop expanding if we're at or beyond the boundary
		if current.X <= outsideMinX || current.X >= outsideMaxX ||
			current.Y <= outsideMinY || current.Y >= outsideMaxY {
			continue
		}

		for _, n := range current.Neighbors() {
			if reachable[n] || p.IsOccupied(n) {
				continue
			}
			reachable[n] = true
			queue = append(queue, n)
		}
	}

	// Now find critical tiles using articulation point detection
	// A tile is critical if removing it disconnects the room from outside
	criticalTiles := make([]Point, 0)

	// For each reachable tile (within bounds), check if removing it disconnects room from outside
	for tile := range reachable {
		// Skip boundary tiles - they're outside
		if tile.X <= outsideMinX || tile.X >= outsideMaxX ||
			tile.Y <= outsideMinY || tile.Y >= outsideMaxY {
			continue
		}
		if p.wouldDisconnectFromOutside(room, tile, outsideMinX, outsideMinY, outsideMaxX, outsideMaxY) {
			criticalTiles = append(criticalTiles, tile)
		}
	}

	return criticalTiles
}

// wouldDisconnectFromOutside checks if blocking a tile would disconnect a room from outside
func (p *Placement) wouldDisconnectFromOutside(room *PlacedRoom, blockedTile Point, outsideMinX, outsideMinY, outsideMaxX, outsideMaxY int) bool {
	freeEdges := p.GetFreeEdges(room)

	// BFS from room edges to outside, avoiding the blocked tile
	visited := make(map[Point]bool)
	visited[blockedTile] = true // Pretend it's blocked

	queue := make([]Point, 0)
	for _, edge := range freeEdges {
		if edge.Neighbor != blockedTile && !p.IsOccupied(edge.Neighbor) {
			queue = append(queue, edge.Neighbor)
			visited[edge.Neighbor] = true
		}
	}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		// Check if we reached outside
		if current.X <= outsideMinX || current.X >= outsideMaxX ||
			current.Y <= outsideMinY || current.Y >= outsideMaxY {
			return false // Can still reach outside
		}

		for _, neighbor := range current.Neighbors() {
			if visited[neighbor] || p.IsOccupied(neighbor) {
				continue
			}
			visited[neighbor] = true
			queue = append(queue, neighbor)
		}
	}

	return true // Cannot reach outside without the blocked tile
}

// UpdateReservations recalculates reservations for all unfinished rooms
func (p *Placement) UpdateReservations(rs *ReservationSystem) {
	// Clear existing reservations
	rs.Reserved = make(map[Point]*Reservation)

	// For each unfinished room, find and reserve critical tiles
	for roomID := range p.Unfinished {
		room := p.Rooms[roomID]
		if room == nil {
			continue
		}

		criticalTiles := p.FindCriticalTiles(room)
		for _, tile := range criticalTiles {
			if rs.Reserved[tile] == nil {
				rs.Reserved[tile] = &Reservation{
					Type:    FullyReserved,
					RoomIDs: make(map[RoomID]bool),
				}
			}
			rs.Reserved[tile].RoomIDs[roomID] = true
		}
	}
}

// CountIndependentPaths counts how many independent paths to outside a room has
// Two paths are independent if they don't share any tiles except at the room's edges
func (p *Placement) CountIndependentPaths(room *PlacedRoom) int {
	freeEdges := p.GetFreeEdges(room)
	if len(freeEdges) == 0 {
		return 0
	}

	// Get bounding box with margin
	minX, minY, maxX, maxY := p.GetBoundingBox()
	outsideMinX, outsideMinY := minX-1, minY-1
	outsideMaxX, outsideMaxY := maxX+1, maxY+1

	// Count how many edges can reach outside via independent paths
	// Simple approximation: count how many edges can reach outside
	// without going through any other free edge's "territory"
	pathCount := 0

	// For simplicity, count edges that can reach outside directly
	for _, edge := range freeEdges {
		if p.canReachOutsideFrom(edge.Neighbor, outsideMinX, outsideMinY, outsideMaxX, outsideMaxY) {
			pathCount++
		}
	}

	return pathCount
}

// canReachOutsideFrom checks if we can reach outside from a starting point
func (p *Placement) canReachOutsideFrom(start Point, outsideMinX, outsideMinY, outsideMaxX, outsideMaxY int) bool {
	if start.X <= outsideMinX || start.X >= outsideMaxX ||
		start.Y <= outsideMinY || start.Y >= outsideMaxY {
		return true
	}

	visited := make(map[Point]bool)
	queue := []Point{start}
	visited[start] = true

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if current.X <= outsideMinX || current.X >= outsideMaxX ||
			current.Y <= outsideMinY || current.Y >= outsideMaxY {
			return true
		}

		for _, n := range current.Neighbors() {
			if visited[n] || p.IsOccupied(n) {
				continue
			}
			visited[n] = true
			queue = append(queue, n)
		}
	}

	return false
}

// UpdateReservationsForRoom updates reservations for a specific room
// Call this after placing a child to potentially release reservations
func (p *Placement) UpdateReservationsForRoom(rs *ReservationSystem, roomID RoomID) {
	room := p.Rooms[roomID]
	if room == nil {
		return
	}

	// Remove old reservations for this room
	for tile, res := range rs.Reserved {
		if res.RoomIDs[roomID] {
			delete(res.RoomIDs, roomID)
			if len(res.RoomIDs) == 0 {
				delete(rs.Reserved, tile)
			}
		}
	}

	// If room is still unfinished, recalculate its critical tiles
	if p.Unfinished[roomID] {
		criticalTiles := p.FindCriticalTiles(room)
		for _, tile := range criticalTiles {
			if rs.Reserved[tile] == nil {
				rs.Reserved[tile] = &Reservation{
					Type:    FullyReserved,
					RoomIDs: make(map[RoomID]bool),
				}
			}
			rs.Reserved[tile].RoomIDs[roomID] = true
		}
	}
}

// CheckPlacementValid checks if placing tiles would block any critical reservations
// Returns true if placement is valid, false if it would block a critical path
func (rs *ReservationSystem) CheckPlacementValid(tiles []Point, placingRoomID RoomID) bool {
	for _, tile := range tiles {
		if !rs.CanPlaceTile(tile, placingRoomID) {
			return false
		}
	}
	return true
}
