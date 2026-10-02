package placement

import (
	"fmt"

	"github.com/kkgams/sdk/go/tilemap"
	"github.com/kkgams/sdk/go/tree"
)

// Random interface for pluggable random number generation
type Random interface {
	Intn(n int) int
}

// GetRoomShapeFunc is a function that returns the shape for a given node
type GetRoomShapeFunc func(node *tree.Node) (RoomShape, error)

// GetExtensionTileFunc is a function that selects which tile to extend with
// Given a list of valid extension tiles, returns the selected one
type GetExtensionTileFunc func(validTiles []Point, rng Random) Point

// DefaultGetExtensionTile randomly selects an extension tile
func DefaultGetExtensionTile(validTiles []Point, rng Random) Point {
	return validTiles[rng.Intn(len(validTiles))]
}

// Generator handles the room placement generation
type Generator struct {
	Tree             *tree.Tree
	GetRoomShape     GetRoomShapeFunc
	GetExtensionTile GetExtensionTileFunc
	Rng              Random
	Placement        *Placement
	Reservations     *ReservationSystem // Legacy - kept for compatibility
	Constraints      *ConstraintSystem  // New constraint propagation system
	MaxPathsPerRoom  int                // Max paths to enumerate per room (default 10)
}

// NewGenerator creates a new generator
func NewGenerator(t *tree.Tree, getRoomShape GetRoomShapeFunc, rng Random) *Generator {
	return &Generator{
		Tree:             t,
		GetRoomShape:     getRoomShape,
		GetExtensionTile: DefaultGetExtensionTile,
		Rng:              rng,
		Placement:        NewPlacement(),
		Reservations:     NewReservationSystem(),
		Constraints:      NewConstraintSystem(),
		MaxPathsPerRoom:  10,
	}
}

// Generate performs the full room placement algorithm
func (g *Generator) Generate() (*Placement, error) {
	return g.generateWithRetry(0)
}

// generateWithRetry attempts placement, retrying if it fails
func (g *Generator) generateWithRetry(attempt int) (*Placement, error) {
	// Reset state for retry
	if attempt > 0 {
		g.Placement = NewPlacement()
		g.Reservations = NewReservationSystem()
		g.Constraints = NewConstraintSystem()
		// Consume random values to create variation on retry
		// Use prime numbers multiplied by attempt for better distribution
		primes := []int{7, 11, 13, 17, 19, 23, 29, 31}
		variationCount := primes[attempt%len(primes)] * (attempt + 1)
		for i := 0; i < variationCount; i++ {
			g.Rng.Intn(100)
		}
	}

	if len(*g.Tree) == 0 {
		return g.Placement, nil
	}

	// Get root node
	root := (*g.Tree)[0]
	rootShape, err := g.GetRoomShape(root)
	if err != nil {
		return nil, err
	}

	// Place root at origin
	rootTiles := rootShape.Translate(Point{0, 0})
	g.Placement.PlaceRoom(0, rootTiles)

	// Check if root has children
	if g.hasChildren(0) {
		g.Placement.MarkUnfinished(0)
		// Initial constraint propagation for root
		if !g.Constraints.Propagate(g.Placement, g.MaxPathsPerRoom) {
			// Should never happen with just root
			return nil, fmt.Errorf("root has no escape path")
		}
	}

	// BFS traversal
	queue := []int{0} // Node indices to process

	for len(queue) > 0 {
		parentIdx := queue[0]
		queue = queue[1:]

		// Get all children of this node, sorted by subtree size (largest first)
		// This gives larger subtrees more space to expand
		children := g.getChildrenSortedBySubtreeSize(parentIdx)

		for i, childIdx := range children {
			isLastChild := i == len(children)-1

			// Place this child
			err := g.placeChildSafe(parentIdx, childIdx, isLastChild)
			if err != nil {
				// Placement failed - retry if we haven't exceeded max attempts
				// Scale max attempts quadratically with tree size (conflicts compound)
				treeSize := len(*g.Tree)
				maxAttempts := 30 + treeSize/2 + treeSize*treeSize/500
				if attempt < maxAttempts {
					return g.generateWithRetry(attempt + 1)
				}
				return nil, fmt.Errorf(
					"failed to place node %d after %d attempts: %v",
					childIdx,
					attempt+1,
					err,
				)
			}

			// IMPORTANT: Mark child as unfinished IMMEDIATELY after placing,
			// so that when placing subsequent siblings, we check that they
			// don't block this child's path to outside.
			if g.hasChildren(childIdx) {
				queue = append(queue, childIdx)
				g.Placement.MarkUnfinished(childIdx)
			}

			// Re-propagate constraints after each child placement
			// This recalculates forced tiles for all unfinished rooms
			treeSize := len(*g.Tree)
			maxAttempts := 30 + treeSize/2 + treeSize*treeSize/500
			if !g.Constraints.Propagate(g.Placement, g.MaxPathsPerRoom) {
				// A room lost all escape paths - retry
				if attempt < maxAttempts {
					return g.generateWithRetry(attempt + 1)
				}
				return nil, fmt.Errorf("propagation failed after placing node %d", childIdx)
			}

			// Check for conflicts (two rooms need the same forced tile)
			if g.Constraints.HasConflict() {
				if attempt < maxAttempts {
					return g.generateWithRetry(attempt + 1)
				}
				return nil, fmt.Errorf("conflict detected after placing node %d", childIdx)
			}
		}

		// Parent is now finished (all children placed)
		g.Placement.MarkFinished(parentIdx)
		// Re-propagate constraints since parent is no longer unfinished
		g.Constraints.Propagate(g.Placement, g.MaxPathsPerRoom)
	}

	return g.Placement, nil
}

// placeChildSafe is like placeChild but returns error instead of panicking
func (g *Generator) placeChildSafe(parentIdx, childIdx int, isLastChild bool) error {
	parent := g.Placement.Rooms[RoomID(parentIdx+1)]
	childShape, err := g.GetRoomShape((*g.Tree)[childIdx])
	if err != nil {
		return err
	}
	childHasChildren := g.hasChildren(childIdx)
	parentRoomID := RoomID(parentIdx + 1)
	childRoomID := RoomID(childIdx + 1)

	// Track extension tiles for cleanup
	extensionTiles := make([]Point, 0)
	originalParentShape := make([]Point, len(parent.CurrentShape))
	copy(originalParentShape, parent.CurrentShape)

	maxIterations := 1000 // Prevent infinite loops
	for iterations := 0; iterations < maxIterations; iterations++ {
		// Find valid positions where child touches parent
		positions := g.Placement.FindTouchingPositions(parent, childShape)

		// Filter positions using constraint system
		positions = g.filterPositionsWithConstraints(
			positions, childShape, childIdx, childHasChildren, parentRoomID, childRoomID,
		)

		if len(positions) > 0 {
			// Pick random valid position
			pos := positions[g.Rng.Intn(len(positions))]
			absoluteTiles := childShape.Translate(pos)

			// Place the child
			g.Placement.PlaceRoom(childIdx, absoluteTiles)

			// Create door connection between child and parent
			g.createDoorConnection(childIdx, parentIdx, absoluteTiles, parent.CurrentShape)

			// Cleanup: find minimal path and remove unnecessary extension tiles
			if len(extensionTiles) > 0 {
				g.cleanupExtensions(parent, originalParentShape, extensionTiles, absoluteTiles)
			}

			return nil
		}

		// No valid position found - extend parent
		extensionTile := g.extendParent(parent)
		if extensionTile == nil {
			return fmt.Errorf("cannot extend parent node %d", parentIdx)
		}
		extensionTiles = append(extensionTiles, *extensionTile)
	}

	return fmt.Errorf("max iterations reached for placing child %d", childIdx)
}

// scoredPosition pairs a position with a quality score
type scoredPosition struct {
	pos   Point
	score int // Higher is better
}

// filterPositionsWithConstraints filters positions using the constraint system
// and scores them to prefer positions that leave more flexibility
func (g *Generator) filterPositionsWithConstraints(
	positions []Point,
	childShape RoomShape,
	childNodeIndex int,
	childHasChildren bool,
	parentRoomID, childRoomID RoomID,
) []Point {
	scored := make([]scoredPosition, 0)

	for _, pos := range positions {
		absoluteTiles := childShape.Translate(pos)

		// Check if any tile would violate constraints (forced tiles)
		if !g.Constraints.CanPlaceTiles(absoluteTiles, childRoomID, parentRoomID) {
			continue
		}

		// Temporarily place child
		for _, tile := range absoluteTiles {
			g.Placement.Grid[tile] = childRoomID
		}

		// Create temporary room for checking
		tempRoom := &PlacedRoom{
			NodeIndex:    childNodeIndex,
			CurrentShape: absoluteTiles,
		}
		g.Placement.Rooms[childRoomID] = tempRoom

		// If child has children, mark as unfinished for constraint checking
		if childHasChildren {
			g.Placement.Unfinished[childRoomID] = true
		}

		// Check all unfinished rooms still have path to outside
		allValid := true
		score := 100 // Base score

		for roomID := range g.Placement.Unfinished {
			room := g.Placement.Rooms[roomID]
			if room == nil {
				continue
			}
			if !g.Placement.HasPathToOutside(room) {
				allValid = false
				break
			}
			// Score: prefer positions that leave other rooms with more free edges
			freeEdges := g.Placement.GetFreeEdges(room)
			score += len(freeEdges) * 5
		}

		// If child has children, it must also have path to outside and room to grow
		if allValid && childHasChildren {
			if !g.Placement.HasPathToOutside(tempRoom) {
				allValid = false
			} else {
				extensionTiles := g.Placement.GetValidExtensionTiles(tempRoom)
				if len(extensionTiles) < 2 {
					allValid = false
				}
				// Score: more extension options is better
				score += len(extensionTiles) * 10
			}
		}

		// Score based on how this position affects other rooms
		if allValid && len(g.Placement.Unfinished) >= 1 {
			for roomID := range g.Placement.Unfinished {
				room := g.Placement.Rooms[roomID]
				if room == nil {
					continue
				}
				freeEdges := g.Placement.GetFreeEdges(room)
				if len(freeEdges) <= 1 {
					// Very constrained - heavy penalty
					score -= 100
				} else if len(freeEdges) == 2 {
					// Somewhat constrained - moderate penalty
					score -= 30
				}
			}
		}

		// Prefer positions that expand outward (closer to bounding box edge)
		// This keeps the interior more open for future placements
		if len(g.Placement.Grid) > 0 {
			minX, minY, maxX, maxY := g.Placement.GetBoundingBox()

			// Calculate how "outward" this position is
			for _, tile := range absoluteTiles {
				// Distance to nearest edge
				distToEdge := tile.X - minX
				if maxX-tile.X < distToEdge {
					distToEdge = maxX - tile.X
				}
				if tile.Y-minY < distToEdge {
					distToEdge = tile.Y - minY
				}
				if maxY-tile.Y < distToEdge {
					distToEdge = maxY - tile.Y
				}

				// Bonus for being near the edge (expanding outward)
				if distToEdge <= 1 {
					score += 15
				}
			}
		}

		// Remove temporary placement
		for _, tile := range absoluteTiles {
			delete(g.Placement.Grid, tile)
		}
		delete(g.Placement.Rooms, childRoomID)
		if childHasChildren {
			delete(g.Placement.Unfinished, childRoomID)
		}

		if allValid {
			scored = append(scored, scoredPosition{pos: pos, score: score})
		}
	}

	if len(scored) == 0 {
		return nil
	}

	// Sort by score (descending) - prefer positions that leave more flexibility
	for i := 0; i < len(scored); i++ {
		for j := i + 1; j < len(scored); j++ {
			if scored[j].score > scored[i].score {
				scored[i], scored[j] = scored[j], scored[i]
			}
		}
	}

	// Return top positions (those with scores within threshold of best)
	minScore := scored[0].score - 50 // Allow positions within 50 points of best
	result := make([]Point, 0)
	for _, sp := range scored {
		if sp.score >= minScore || len(result) == 0 {
			result = append(result, sp.pos)
		}
	}

	return result
}

// extendParent adds one tile to parent, ensuring it doesn't block any unfinished room
// and respects constraints (forced tiles from other rooms)
func (g *Generator) extendParent(parent *PlacedRoom) *Point {
	validExtensions := g.Placement.GetValidExtensionTiles(parent)
	if len(validExtensions) == 0 {
		return nil
	}

	// Filter extensions that don't block any unfinished room
	safeExtensions := make([]Point, 0)
	parentRoomID := RoomID(parent.NodeIndex + 1)

	for _, ext := range validExtensions {
		// Check if this extension is on a forced tile (that's not ours)
		// Parent can use its own forced tiles (it's extending to reach children)
		if !g.Constraints.CanPlaceTiles([]Point{ext}, parentRoomID, parentRoomID) {
			continue
		}

		// Temporarily add extension
		g.Placement.Grid[ext] = parentRoomID

		// Check all unfinished rooms still have path to outside
		allSafe := true
		for roomID := range g.Placement.Unfinished {
			room := g.Placement.Rooms[roomID]
			if room == nil {
				continue
			}
			// Include the extension in parent's shape temporarily
			originalShape := parent.CurrentShape
			parent.CurrentShape = append(parent.CurrentShape, ext)

			if !g.Placement.HasPathToOutside(room) {
				allSafe = false
			}

			parent.CurrentShape = originalShape

			if !allSafe {
				break
			}
		}

		// Remove temporary extension
		delete(g.Placement.Grid, ext)

		if allSafe {
			safeExtensions = append(safeExtensions, ext)
		}
	}

	if len(safeExtensions) == 0 {
		// All extensions would block something - this shouldn't happen
		// if the algorithm is working correctly
		return nil
	}

	// Pick random safe extension
	selected := g.GetExtensionTile(safeExtensions, g.Rng)
	g.Placement.ExtendRoom(parent, selected)

	return &selected
}

// cleanupExtensions removes unnecessary extension tiles, keeping only the minimal path
// from original parent shape to the placed child
func (g *Generator) cleanupExtensions(
	parent *PlacedRoom,
	originalShape []Point,
	extensionTiles []Point,
	childTiles []Point,
) {
	// Build set of original parent tiles
	originalSet := make(map[Point]bool)
	for _, p := range originalShape {
		originalSet[p] = true
	}

	// Build set of extension tiles
	extensionSet := make(map[Point]bool)
	for _, p := range extensionTiles {
		extensionSet[p] = true
	}

	// Build set of child tiles
	childSet := make(map[Point]bool)
	for _, p := range childTiles {
		childSet[p] = true
	}

	// Find minimal path from original parent to child through extension tiles
	// BFS from all original parent edges
	type pathNode struct {
		point Point
		path  []Point // extension tiles in path
	}

	visited := make(map[Point]bool)
	queue := make([]pathNode, 0)

	// Start from edges of original parent that touch extension tiles
	for _, p := range originalShape {
		for _, n := range p.Neighbors() {
			if extensionSet[n] && !visited[n] {
				visited[n] = true
				queue = append(queue, pathNode{point: n, path: []Point{n}})
			}
		}
	}

	var minimalPath []Point

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		// Check if we reached the child
		for _, n := range current.point.Neighbors() {
			if childSet[n] {
				// Found path to child!
				minimalPath = current.path
				goto foundPath
			}
		}

		// Continue through extension tiles
		for _, n := range current.point.Neighbors() {
			if extensionSet[n] && !visited[n] {
				visited[n] = true
				newPath := make([]Point, len(current.path)+1)
				copy(newPath, current.path)
				newPath[len(current.path)] = n
				queue = append(queue, pathNode{point: n, path: newPath})
			}
		}
	}

foundPath:
	if minimalPath == nil {
		// Child is directly adjacent to original parent, no extensions needed
		minimalPath = []Point{}
	}

	// Build set of tiles to keep
	keepSet := make(map[Point]bool)
	for _, p := range minimalPath {
		keepSet[p] = true
	}

	// Remove extension tiles not in minimal path
	for _, ext := range extensionTiles {
		if !keepSet[ext] {
			g.Placement.RemoveTile(parent, ext)
		}
	}
}

// createDoorConnection finds adjacent tiles between child and parent and creates a door
func (g *Generator) createDoorConnection(childIdx, parentIdx int, childTiles, parentTiles []Point) {
	childRoomID := childIdx + 1
	parentRoomID := parentIdx + 1

	// Build set of parent tiles for quick lookup
	parentSet := make(map[Point]bool)
	for _, pt := range parentTiles {
		parentSet[pt] = true
	}

	// Find first adjacent pair
	for _, childTile := range childTiles {
		for _, neighbor := range childTile.Neighbors() {
			if parentSet[neighbor] {
				g.Placement.AddDoor(childTile, neighbor, childRoomID, parentRoomID)
				return
			}
		}
	}
}

// hasChildren checks if a node has children in the tree
func (g *Generator) hasChildren(nodeIdx int) bool {
	for _, node := range *g.Tree {
		if node.ParentId == nodeIdx && g.Tree.IndexOf(node) != nodeIdx {
			return true
		}
	}
	return false
}

// getChildrenIndices returns indices of all children of a node
func (g *Generator) getChildrenIndices(parentIdx int) []int {
	children := make([]int, 0)
	for i, node := range *g.Tree {
		if node.ParentId == parentIdx && i != parentIdx {
			children = append(children, i)
		}
	}
	return children
}

// countDescendants returns the total number of descendants of a node
func (g *Generator) countDescendants(nodeIdx int) int {
	count := 0
	children := g.getChildrenIndices(nodeIdx)
	for _, childIdx := range children {
		count++                               // Count the child itself
		count += g.countDescendants(childIdx) // Plus all its descendants
	}
	return count
}

// getChildrenSortedBySubtreeSize returns children sorted by subtree size
// Children with NO descendants (leaf nodes) go first - they don't need escape paths
// Children with descendants go last - sorted by size (smallest first)
func (g *Generator) getChildrenSortedBySubtreeSize(parentIdx int) []int {
	children := g.getChildrenIndices(parentIdx)
	if len(children) <= 1 {
		return children
	}

	// Calculate subtree sizes and separate leaves from non-leaves
	type childWithSize struct {
		idx  int
		size int
	}
	leaves := make([]int, 0)
	nonLeaves := make([]childWithSize, 0)

	for _, childIdx := range children {
		size := g.countDescendants(childIdx)
		if size == 0 {
			leaves = append(leaves, childIdx)
		} else {
			nonLeaves = append(nonLeaves, childWithSize{idx: childIdx, size: size})
		}
	}

	// Sort non-leaves by size (ascending - smallest subtrees first)
	for i := 0; i < len(nonLeaves); i++ {
		for j := i + 1; j < len(nonLeaves); j++ {
			if nonLeaves[j].size < nonLeaves[i].size {
				nonLeaves[i], nonLeaves[j] = nonLeaves[j], nonLeaves[i]
			}
		}
	}

	// Combine: leaves first, then non-leaves by size
	result := make([]int, 0, len(children))
	result = append(result, leaves...)
	for _, ws := range nonLeaves {
		result = append(result, ws.idx)
	}
	return result
}

// ToTileMap converts the placement to a TileMap
func (p *Placement) ToTileMap() *tilemap.TileMap {
	if len(p.Grid) == 0 {
		return tilemap.NewTileMap()
	}

	// Get bounding box
	minX, minY, maxX, maxY := p.GetBoundingBox()
	width := maxX - minX + 1
	height := maxY - minY + 1
	offset := Point{X: minX, Y: minY}

	// Create rooms layer from final occupied tiles, including extensions.
	roomLayer := tilemap.NewTileLayer(width, height)
	roomLayer.Props["name"] = "rooms"
	for pos, roomID := range p.Grid {
		x := pos.X - minX
		y := pos.Y - minY
		idx := y*width + x
		roomLayer.Data[idx] = uint32(roomID)
	}

	// Create doors layer.
	doorLayer := generateDoorLayer(p.Doors, width, height, offset)

	// Create original rooms layer from each room's initially placed shape,
	// excluding extension tiles added while creating corridors/connections.
	originalRoomsLayer := generateOriginalRoomsLayer(p.Rooms, width, height, offset)

	tm := tilemap.NewTileMap()
	tm.Layers = append(tm.Layers, *roomLayer)
	tm.Layers = append(tm.Layers, *doorLayer)
	tm.Layers = append(tm.Layers, *originalRoomsLayer)

	return tm
}

// generateDoorLayer creates door layer data from door connections.
func generateDoorLayer(doors []DoorConnection, width, height int, offset Point) *tilemap.TileLayer {
	layer := tilemap.NewTileLayer(width, height)
	layer.Props["name"] = "doors"
	layer.Props["type"] = "doors"

	for _, door := range doors {
		x := door.Point.X - offset.X
		y := door.Point.Y - offset.Y
		idx := y*width + x
		if idx >= 0 && idx < len(layer.Data) {
			layer.Data[idx] |= uint32(door.Direction)
		}
	}

	return layer
}

// generateOriginalRoomsLayer creates a layer from original placed room shapes.
func generateOriginalRoomsLayer(rooms map[RoomID]*PlacedRoom, width, height int, offset Point) *tilemap.TileLayer {
	layer := tilemap.NewTileLayer(width, height)
	layer.Props["name"] = "original-rooms"

	for roomID, room := range rooms {
		for _, tile := range room.OriginalShape {
			x := tile.X - offset.X
			y := tile.Y - offset.Y
			idx := y*width + x
			if idx >= 0 && idx < len(layer.Data) {
				layer.Data[idx] = uint32(roomID)
			}
		}
	}

	return layer
}
