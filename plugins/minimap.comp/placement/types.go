package placement

// Point represents a 2D coordinate
type Point struct {
	X, Y int
}

// Add returns a new Point offset by the given delta
func (p Point) Add(dx, dy int) Point {
	return Point{X: p.X + dx, Y: p.Y + dy}
}

// Neighbors returns the 4 cardinal neighbors of this point
func (p Point) Neighbors() [4]Point {
	return [4]Point{
		{p.X, p.Y - 1}, // North
		{p.X + 1, p.Y}, // East
		{p.X, p.Y + 1}, // South
		{p.X - 1, p.Y}, // West
	}
}

// Direction constants
const (
	DirNorth = 0
	DirEast  = 1
	DirSouth = 2
	DirWest  = 3
)

// Door direction bit masks (for door layer encoding)
const (
	DoorNorth uint8 = 1
	DoorEast  uint8 = 2
	DoorSouth uint8 = 4
	DoorWest  uint8 = 8
)

// DoorConnection represents a door tile on the grid
type DoorConnection struct {
	Point     Point // Grid coordinate of the door tile
	RoomID    int   // Which room this door belongs to (1-based)
	Direction uint8 // Bit mask: North=1, East=2, South=4, West=8
}

// DirectionOffset returns the dx, dy for a direction
func DirectionOffset(dir int) (int, int) {
	switch dir {
	case DirNorth:
		return 0, -1
	case DirEast:
		return 1, 0
	case DirSouth:
		return 0, 1
	case DirWest:
		return -1, 0
	}
	return 0, 0
}

// Edge represents one face of a tile that borders empty space
type Edge struct {
	Position  Point // Absolute position of tile owning this edge
	Direction int   // Which face (DirNorth, DirEast, DirSouth, DirWest)
	Neighbor  Point // Absolute position of the empty neighbor
}

// RoomShape represents a room's shape as relative coordinates
// The shape must be connected (each tile adjacent to at least one other)
type RoomShape []Point

// Translate returns a new shape offset by the given position
func (s RoomShape) Translate(offset Point) []Point {
	result := make([]Point, len(s))
	for i, p := range s {
		result[i] = Point{X: offset.X + p.X, Y: offset.Y + p.Y}
	}
	return result
}

// Contains checks if the shape contains a point (relative coordinates)
func (s RoomShape) Contains(p Point) bool {
	for _, sp := range s {
		if sp.X == p.X && sp.Y == p.Y {
			return true
		}
	}
	return false
}

// PlacedRoom represents a room that has been placed on the grid
type PlacedRoom struct {
	NodeIndex     int     // Index in the tree
	OriginalShape []Point // Original shape (absolute coordinates)
	CurrentShape  []Point // Current shape including extensions (absolute coordinates)
}

// ContainsTile checks if the room contains the given absolute position
func (r *PlacedRoom) ContainsTile(p Point) bool {
	for _, tile := range r.CurrentShape {
		if tile.X == p.X && tile.Y == p.Y {
			return true
		}
	}
	return false
}

// RoomID is the identifier for a room (node index + 1 to avoid 0 = empty confusion)
type RoomID int

// Placement is the working data structure for room placement
type Placement struct {
	Grid       map[Point]RoomID // Sparse grid: position -> room owner
	Rooms      map[RoomID]*PlacedRoom
	Unfinished map[RoomID]bool  // Rooms with unplaced children (set)
	Doors      []DoorConnection // Door connections between rooms
}

// NewPlacement creates a new empty placement
func NewPlacement() *Placement {
	return &Placement{
		Grid:       make(map[Point]RoomID),
		Rooms:      make(map[RoomID]*PlacedRoom),
		Unfinished: make(map[RoomID]bool),
	}
}

// IsOccupied checks if a position is occupied by any room
func (p *Placement) IsOccupied(pos Point) bool {
	_, ok := p.Grid[pos]
	return ok
}

// GetRoom returns the room at a position, or nil if empty
func (p *Placement) GetRoom(pos Point) *PlacedRoom {
	id, ok := p.Grid[pos]
	if !ok {
		return nil
	}
	return p.Rooms[id]
}

// PlaceRoom adds a room to the placement
func (p *Placement) PlaceRoom(nodeIndex int, shape []Point) *PlacedRoom {
	id := RoomID(nodeIndex + 1) // +1 so room 0 maps to ID 1

	room := &PlacedRoom{
		NodeIndex:     nodeIndex,
		OriginalShape: make([]Point, len(shape)),
		CurrentShape:  make([]Point, len(shape)),
	}
	copy(room.OriginalShape, shape)
	copy(room.CurrentShape, shape)

	p.Rooms[id] = room

	// Mark all tiles in grid
	for _, tile := range shape {
		p.Grid[tile] = id
	}

	return room
}

// ExtendRoom adds a single tile to a room's shape
func (p *Placement) ExtendRoom(room *PlacedRoom, tile Point) {
	id := RoomID(room.NodeIndex + 1)
	room.CurrentShape = append(room.CurrentShape, tile)
	p.Grid[tile] = id
}

// RemoveTile removes a tile from a room (for cleanup)
func (p *Placement) RemoveTile(room *PlacedRoom, tile Point) {
	// Remove from grid
	delete(p.Grid, tile)

	// Remove from current shape
	newShape := make([]Point, 0, len(room.CurrentShape)-1)
	for _, t := range room.CurrentShape {
		if t.X != tile.X || t.Y != tile.Y {
			newShape = append(newShape, t)
		}
	}
	room.CurrentShape = newShape
}

// MarkUnfinished marks a room as having unplaced children
func (p *Placement) MarkUnfinished(nodeIndex int) {
	p.Unfinished[RoomID(nodeIndex+1)] = true
}

// MarkFinished marks a room as having all children placed
func (p *Placement) MarkFinished(nodeIndex int) {
	delete(p.Unfinished, RoomID(nodeIndex+1))
}

// AddDoor adds a door connection between a child room and its parent
func (p *Placement) AddDoor(childPoint, parentPoint Point, childRoomID, parentRoomID int) {
	childDir := calculateDirection(childPoint, parentPoint)
	parentDir := calculateDirection(parentPoint, childPoint)

	p.Doors = append(p.Doors, DoorConnection{
		Point:     childPoint,
		RoomID:    childRoomID,
		Direction: childDir,
	})
	p.Doors = append(p.Doors, DoorConnection{
		Point:     parentPoint,
		RoomID:    parentRoomID,
		Direction: parentDir,
	})
}

// calculateDirection determines the direction from one point to an adjacent point
func calculateDirection(fromPoint, toPoint Point) uint8 {
	dx := toPoint.X - fromPoint.X
	dy := toPoint.Y - fromPoint.Y

	if dy == -1 {
		return DoorNorth
	}
	if dx == 1 {
		return DoorEast
	}
	if dy == 1 {
		return DoorSouth
	}
	if dx == -1 {
		return DoorWest
	}
	return 0
}

// IsUnfinished checks if a room has unplaced children
func (p *Placement) IsUnfinished(nodeIndex int) bool {
	return p.Unfinished[RoomID(nodeIndex+1)]
}

// GetBoundingBox returns the min/max coordinates of all placed tiles
func (p *Placement) GetBoundingBox() (minX, minY, maxX, maxY int) {
	first := true
	for pos := range p.Grid {
		if first {
			minX, maxX = pos.X, pos.X
			minY, maxY = pos.Y, pos.Y
			first = false
			continue
		}
		if pos.X < minX {
			minX = pos.X
		}
		if pos.X > maxX {
			maxX = pos.X
		}
		if pos.Y < minY {
			minY = pos.Y
		}
		if pos.Y > maxY {
			maxY = pos.Y
		}
	}
	return
}
