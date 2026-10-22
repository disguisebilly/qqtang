package battleengine

import "sort"

// NativeMapDestructionMS is CMapElem's state-2 countdown (005ef2cf).
// The object remains in both movement and flame queries until 005eee71 retires it.
const NativeMapDestructionMS uint32 = 500

type pendingMapRetirement struct {
	cell   Cell
	tile   Tile
	cells  []Cell
	at     uint32
	owner  uint16
	bombID uint32
}

type nativeBombRegistration struct {
	cell  Cell
	power byte
}

func (engine *Engine) beginMapRetirement(cell Cell, cells []Cell, at uint32, owner uint16, bombID uint32) (Event, bool) {
	tile, _ := engine.grid.Cell(cell)
	if engine.pendingMapRetirements == nil {
		engine.pendingMapRetirements = make(map[Cell]pendingMapRetirement)
	}
	key := cell
	if tile.ElementWidth != 0 && tile.ElementHeight != 0 {
		key = tile.ElementAnchor
		cells = nil
		for row := int16(0); row < int16(tile.ElementHeight); row++ {
			for col := int16(0); col < int16(tile.ElementWidth); col++ {
				memberCell := Cell{Row: key.Row + row, Col: key.Col + col}
				member, inside := engine.grid.Cell(memberCell)
				if inside && member.MapElementID == tile.MapElementID && member.ElementAnchor == key {
					cells = append(cells, memberCell)
				}
			}
		}
	}
	due := saturatingAdd(at, NativeMapDestructionMS)
	previous, exists := engine.pendingMapRetirements[key]
	if exists && previous.tile.MapElementID == tile.MapElementID && previous.at > due {
		due = previous.at
	}
	engine.pendingMapRetirements[key] = pendingMapRetirement{cell: cell, tile: tile, cells: append([]Cell(nil), cells...), at: due, owner: owner, bombID: bombID}
	if exists {
		return Event{}, false
	}
	return Event{Kind: EventCellDestroyed, TimeMS: at, PlayerID: owner, BombID: bombID, Cell: cell, ObjectID: tile.MapElementID,
		OpenedTraversal: !tile.PlayerPassable(DirectionUp) || !tile.PlayerPassable(DirectionRight) || !tile.PlayerPassable(DirectionDown) || !tile.PlayerPassable(DirectionLeft)}, true
}

func (engine *Engine) expireMapRetirements() []Event {
	if len(engine.pendingMapRetirements) == 0 {
		return nil
	}
	keys := make([]Cell, 0, len(engine.pendingMapRetirements))
	for cell, p := range engine.pendingMapRetirements {
		if p.at <= engine.elapsedMS {
			keys = append(keys, cell)
		}
	}
	sort.Slice(keys, func(i, j int) bool {
		return keys[i].Row < keys[j].Row || (keys[i].Row == keys[j].Row && keys[i].Col < keys[j].Col)
	})
	var events []Event
	for _, key := range keys {
		p := engine.pendingMapRetirements[key]
		delete(engine.pendingMapRetirements, key)
		for _, cell := range p.cells {
			tile, inside := engine.grid.Cell(cell)
			if !inside || tile.MapElementID != p.tile.MapElementID || tile.ElementAnchor != p.tile.ElementAnchor {
				continue
			}
			engine.grid.Cells[engine.gridIndex(cell)] = Tile{Kind: CellOpen, FlamePassable: true}
			// Only the map object retires here. A delivery or confirmed death
			// drop created after the blast belongs to its own object lifetime.
			if event, ok := engine.revealPickupAt(cell, p.owner, p.bombID); ok {
				events = append(events, event)
			}
		}
	}
	return events
}

func (engine *Engine) movePendingMapRetirement(source, target Cell) {
	p, exists := engine.pendingMapRetirements[source]
	if !exists {
		return
	}
	delete(engine.pendingMapRetirements, source)
	dr, dc := target.Row-source.Row, target.Col-source.Col
	p.cell.Row += dr
	p.cell.Col += dc
	p.tile.ElementAnchor = target
	for i := range p.cells {
		p.cells[i].Row += dr
		p.cells[i].Col += dc
	}
	engine.pendingMapRetirements[target] = p
}

// syncNativeBombLinks mirrors 005d388a/005d3a9d. Registration or relocation
// updates edges; later wall removal does not rebuild the existing relation.
func (engine *Engine) syncNativeBombLinks() {
	if engine.nativeBombRegistrations == nil && len(engine.bombs) == 0 {
		return
	}
	if engine.nativeBombRegistrations == nil {
		engine.nativeBombRegistrations = map[uint32]nativeBombRegistration{}
		engine.nativeBombLinks = map[uint32][]uint32{}
	}
	alive := make(map[uint32]Bomb, len(engine.bombs))
	for _, b := range engine.bombs {
		alive[b.ID] = b
	}
	for id, registered := range engine.nativeBombRegistrations {
		b, exists := alive[id]
		if exists && registered.cell == b.Cell && registered.power == b.Power {
			continue
		}
		delete(engine.nativeBombRegistrations, id)
		delete(engine.nativeBombLinks, id)
		for root, links := range engine.nativeBombLinks {
			kept := links[:0]
			for _, other := range links {
				if other != id {
					kept = append(kept, other)
				}
			}
			engine.nativeBombLinks[root] = kept
		}
	}
	for _, b := range engine.bombs {
		if _, exists := engine.nativeBombRegistrations[b.ID]; exists {
			continue
		}
		others := make([]uint32, 0, len(engine.nativeBombRegistrations))
		for id := range engine.nativeBombRegistrations {
			others = append(others, id)
		}
		sort.Slice(others, func(i, j int) bool { return others[i] < others[j] })
		engine.nativeBombRegistrations[b.ID] = nativeBombRegistration{b.Cell, b.Power}
		for _, id := range others {
			other := alive[id]
			if engine.nativeBombCanChain(b, other) {
				engine.nativeBombLinks[b.ID] = append(engine.nativeBombLinks[b.ID], id)
			}
			if engine.nativeBombCanChain(other, b) {
				engine.nativeBombLinks[id] = append(engine.nativeBombLinks[id], b.ID)
			}
		}
	}
}

func (engine *Engine) nativeBombCanChain(from, to Bomb) bool {
	if from.Cell == to.Cell {
		return true
	}
	direction := DirectionNone
	distance := int16(0)
	if from.Cell.Row == to.Cell.Row {
		distance = to.Cell.Col - from.Cell.Col
		direction = DirectionRight
		if distance < 0 {
			distance = -distance
			direction = DirectionLeft
		}
	} else if from.Cell.Col == to.Cell.Col {
		distance = to.Cell.Row - from.Cell.Row
		direction = DirectionDown
		if distance < 0 {
			distance = -distance
			direction = DirectionUp
		}
	} else {
		return false
	}
	return distance <= engine.nativeBlastReach(from, direction)
}

// nativeBlastReach is 005d476f: the next cell's bubble index is checked
// before static exit/entry flags. Destructible endpoints are included.
func (engine *Engine) nativeBlastReach(bomb Bomb, direction Direction) int16 {
	dx, dy, ok := direction.delta()
	if !ok || direction == DirectionNone {
		return 0
	}
	previous, _ := engine.grid.Cell(bomb.Cell)
	for distance := int16(1); distance <= int16(bomb.Power); distance++ {
		cell := Cell{Row: bomb.Cell.Row + int16(dy)*distance, Col: bomb.Cell.Col + int16(dx)*distance}
		tile, inside := engine.grid.Cell(cell)
		if !inside {
			return distance - 1
		}
		for _, other := range engine.bombs {
			if other.Cell == cell {
				if _, registered := engine.nativeBombRegistrations[other.ID]; registered {
					return distance
				}
			}
		}
		if !previous.FlamePassableIn(oppositeDirection(direction)) {
			return distance - 1
		}
		if !tile.FlamePassableIn(direction) {
			if tile.Durability > 0 {
				return distance
			}
			return distance - 1
		}
		previous = tile
	}
	return int16(bomb.Power)
}
