package battleengine

import "sort"

func (engine *Engine) placeBomb(actorIndex int) (Event, bool) {
	bomb, ok := engine.prospectiveBomb(actorIndex, true)
	if !ok {
		return Event{}, false
	}
	return engine.commitBomb(bomb), true
}

// placeBombOnSnapshotEmptyCell is used by Step after it has evaluated native
// local occupancy against the pre-placement battlefield. FUN_005b08e0 checks
// the local cell before producing 0xFA3, while the remote 0xFA3 consumer at
// FUN_006084bf creates the bubble without repeating that occupancy check.
// Consequently, different players that all pass their local check in one
// input frame may produce multiple bubbles in one cell (the native 叠炮 race).
func (engine *Engine) placeBombOnSnapshotEmptyCell(actorIndex int) (Event, bool) {
	bomb, ok := engine.prospectiveBomb(actorIndex, false)
	if !ok {
		return Event{}, false
	}
	return engine.commitBomb(bomb), true
}

// prospectiveBomb applies the same placement/capacity rules as the native
// producer without mutating engine state. requireEmpty distinguishes ordinary
// sequential placement from Step's already-validated simultaneous stack race.
func (engine *Engine) prospectiveBomb(actorIndex int, requireEmpty bool) (Bomb, bool) {
	actor := &engine.actors[actorIndex]
	cell := actor.Position.Cell()
	if requireEmpty && engine.bombAt(cell) >= 0 {
		return Bomb{}, false
	}
	// The original local producer rejects placement unless the actor's scene
	// cell is an ordinary open grid cell. Native pass/up-wall can temporarily
	// render an actor over scenery, but it does not turn that scenery into a
	// legal bubble cell.
	tile, inside := engine.grid.Cell(cell)
	if !inside || tile.Kind != CellOpen || tile.MapElementOccupied {
		return Bomb{}, false
	}
	capacity := engine.actorCapabilities(actor, actor.Facing).EffectiveBombCapacity
	if engine.activeBombCount(actor.PlayerID) >= int(capacity) {
		return Bomb{}, false
	}
	return Bomb{
		ID: engine.nextBombID, OwnerID: actor.PlayerID, Cell: cell, Power: actor.BombPower,
		// 005df7d0 requires a negative remaining fuse; zero is still armed.
		ExplodeAtMS:     saturatingAdd(saturatingAdd(engine.elapsedMS, engine.rules.BombFuseMS), 1),
		SceneFourEffect: engine.sceneFourEffectActive(actor),
	}, true
}

func (engine *Engine) commitBomb(bomb Bomb) Event {
	engine.nextBombID++
	engine.bombs = append(engine.bombs, bomb)
	engine.syncNativeBombLinks()
	return Event{
		Kind: EventBombPlaced, TimeMS: engine.elapsedMS, PlayerID: bomb.OwnerID,
		BombID: bomb.ID, Cell: bomb.Cell,
	}
}

func (engine *Engine) explodeDueBombs() []Event {
	return engine.explodeDueBombsMatching(nil)
}

func (engine *Engine) explodeDueBombsMatching(acceptRoot func(Bomb) bool) []Event {
	engine.syncNativeBombLinks()
	queue := make([]uint32, 0)
	var scheduled map[uint32]bool
	for _, bomb := range engine.bombs {
		if bomb.EffectiveExplodeAtMS() <= engine.elapsedMS && (acceptRoot == nil || acceptRoot(bomb)) {
			if scheduled == nil {
				scheduled = make(map[uint32]bool)
			}
			queue = append(queue, bomb.ID)
			scheduled[bomb.ID] = true
		}
	}
	if len(queue) == 0 {
		return []Event{}
	}
	// 005d4937 expands the saved relation before calculating any current
	// blast bounds. A newly opened route must not invent a new chain edge.
	for cursor := 0; cursor < len(queue); cursor++ {
		root := queue[cursor]
		for _, id := range engine.nativeBombLinks[root] {
			if scheduled[id] {
				continue
			}
			index := engine.bombIndexByID(id)
			if index < 0 {
				continue
			}
			scheduled[id] = true
			queue = append(queue, id)
			if engine.bombs[index].TriggeredByBombID == 0 {
				engine.bombs[index].TriggeredByBombID = root
			}
		}
	}
	exploded := make(map[uint32]bool)
	events := make([]Event, 0)
	batch := blastBatch{wallSeen: make(map[Cell]bool), objectSeen: make(map[Cell]bool)}
	clearedCells := map[Cell]bool{}
	for len(queue) > 0 {
		bombID := queue[0]
		queue = queue[1:]
		bombIndex := engine.bombIndexByID(bombID)
		if bombIndex < 0 || exploded[bombID] {
			continue
		}
		bomb := engine.bombs[bombIndex]
		exploded[bombID] = true
		blastCells := engine.blastCells(bomb, scheduled, &queue, &batch)
		rowMin, rowMax := bomb.Cell.Row, bomb.Cell.Row
		colMin, colMax := bomb.Cell.Col, bomb.Cell.Col
		for _, cell := range blastCells {
			if cell.Row < rowMin {
				rowMin = cell.Row
			}
			if cell.Row > rowMax {
				rowMax = cell.Row
			}
			if cell.Col < colMin {
				colMin = cell.Col
			}
			if cell.Col > colMax {
				colMax = cell.Col
			}
		}
		events = append(events, Event{
			Kind: EventBombExploded, TimeMS: engine.elapsedMS, PlayerID: bomb.OwnerID,
			BombID: bomb.ID, TriggeredByBombID: bomb.TriggeredByBombID, Cell: bomb.Cell,
			BlastRowMin: rowMin, BlastRowMax: rowMax, BlastColMin: colMin, BlastColMax: colMax,
		})
		for _, cell := range blastCells {
			clearedCells[cell] = true
			engine.addBombFlame(cell, bomb.OwnerID, bomb.ID)
		}
	}
	// Native FA4 computes the entire due/chain batch against the old static
	// layers. Remove old visible objects before revealing this batch's drops.
	events = append(events, engine.commitBlastBatch(batch)...)
	if len(exploded) == 0 {
		return events
	}
	keptBombs := engine.bombs[:0]
	for _, bomb := range engine.bombs {
		if !exploded[bomb.ID] && !clearedCells[bomb.Cell] {
			keptBombs = append(keptBombs, bomb)
		}
	}
	engine.bombs = keptBombs
	engine.syncNativeBombLinks()
	return events
}

func (engine *Engine) bombIndexByID(bombID uint32) int {
	for index := range engine.bombs {
		if engine.bombs[index].ID == bombID {
			return index
		}
	}
	return -1
}

type blastCellHit struct {
	cell Cell
	bomb Bomb
}

type blastBatch struct {
	walls, objects       []blastCellHit
	wallSeen, objectSeen map[Cell]bool
}

func (batch *blastBatch) hit(cell Cell, bomb Bomb, wall bool) {
	seen, hits := batch.objectSeen, &batch.objects
	if wall {
		seen, hits = batch.wallSeen, &batch.walls
	}
	if !seen[cell] {
		seen[cell] = true
		*hits = append(*hits, blastCellHit{cell: cell, bomb: bomb})
	}
}

func (engine *Engine) commitBlastBatch(batch blastBatch) []Event {
	var events []Event
	for _, hit := range batch.objects {
		events = append(events, engine.destroyBlastObjectsAtCell(hit.cell, hit.bomb.OwnerID, hit.bomb.ID)...)
	}
	for _, hit := range batch.walls {
		cell, bomb := hit.cell, hit.bomb
		index := engine.gridIndex(cell)
		tile := engine.grid.Cells[index]
		if tile.Durability > 1 {
			engine.grid.Cells[index].Durability--
			events = append(events, Event{Kind: EventCellDamaged, TimeMS: engine.elapsedMS, PlayerID: bomb.OwnerID, BombID: bomb.ID, Cell: cell, ObjectID: tile.MapElementID})
		} else {
			if event, started := engine.beginMapRetirement(cell, []Cell{cell}, engine.elapsedMS, bomb.OwnerID, bomb.ID); started {
				events = append(events, event)
			}
		}
	}
	return events
}

func (engine *Engine) blastCells(bomb Bomb, _ map[uint32]bool, _ *[]uint32, batch *blastBatch) []Cell {
	result := []Cell{bomb.Cell}
	if tile, inside := engine.grid.Cell(bomb.Cell); inside && tile.Kind == CellOpen && !tile.MapElementOccupied {
		batch.hit(bomb.Cell, bomb, false)
	}
	for _, direction := range []Direction{DirectionUp, DirectionRight, DirectionDown, DirectionLeft} {
		dx, dy, _ := direction.delta()
		reach := engine.nativeBlastReach(bomb, direction)
		for distance := int16(1); distance <= reach; distance++ {
			cell := Cell{Row: bomb.Cell.Row + int16(dy)*distance, Col: bomb.Cell.Col + int16(dx)*distance}
			tile, _ := engine.grid.Cell(cell)
			result = append(result, cell)
			if tile.Durability > 0 {
				batch.hit(cell, bomb, true)
			}
			// 005d4937 queues visible item destruction only when both static
			// map layers are empty. A destroying wall still occupies its layer.
			if tile.Kind == CellOpen && !tile.MapElementOccupied {
				batch.hit(cell, bomb, false)
			}
		}
	}
	return result
}

func (engine *Engine) addFlame(cell Cell, ownerID uint16) {
	engine.addBombFlame(cell, ownerID, 0)
}

func (engine *Engine) addBombFlame(cell Cell, ownerID uint16, bombID uint32) {
	expires := saturatingAdd(engine.elapsedMS, engine.rules.FlameDurationMS)
	// The list is kept in cell order. Insert one cell instead of sorting the
	// complete list after every arm pixel in every speculative explosion.
	index := sort.Search(len(engine.flames), func(i int) bool {
		other := engine.flames[i].Cell
		return other.Row > cell.Row || (other.Row == cell.Row && other.Col >= cell.Col)
	})
	if index < len(engine.flames) && engine.flames[index].Cell == cell {
		engine.flames[index].ImpactAtMS = engine.elapsedMS
		if engine.flames[index].ExpiresAtMS < expires {
			engine.flames[index].ExpiresAtMS = expires
		}
		engine.flames[index].OwnerID = ownerID
		engine.flames[index].BombID = bombID
		return
	}
	engine.flames = append(engine.flames, Flame{})
	copy(engine.flames[index+1:], engine.flames[index:len(engine.flames)-1])
	engine.flames[index] = Flame{Cell: cell, OwnerID: ownerID, BombID: bombID, ImpactAtMS: engine.elapsedMS, ExpiresAtMS: expires}
}

func (engine *Engine) expireFlames() {
	kept := engine.flames[:0]
	for _, flame := range engine.flames {
		if flame.ExpiresAtMS > engine.elapsedMS {
			kept = append(kept, flame)
		}
	}
	engine.flames = kept
}
