package battleengine

import "testing"

// Existing phase-level fixtures can finish map timers without advancing
// unrelated synthetic actor deaths. Dedicated tests below exercise real Step.
func finishMapRetirementsForTest(engine *Engine) []Event {
	for _, p := range engine.pendingMapRetirements {
		if p.at > engine.elapsedMS {
			engine.elapsedMS = p.at
		}
	}
	return engine.expireMapRetirements()
}

func TestNativeMapRetirementKeepsMovementPlacementAndDropBlockedFor500ms(t *testing.T) {
	config := testConfig()
	config.Rules.TickMS = 20
	config.Rules.NativeOutcomeAuthority = true
	wall := Cell{Row: 1, Col: 2}
	config.Grid.Cells[7] = Tile{Kind: CellBreakable, MapElementOccupied: true, Durability: 1, MapElementID: 9011, ElementWidth: 1, ElementHeight: 1, ElementAnchor: wall}
	config.Pickups = []Pickup{{SceneID: SceneBombPowerSmall, Cell: wall, State: PickupHidden}}
	engine := mustEngine(t, config)
	_, err := engine.ApplyVerifiedBombExplosionAt(3, []VerifiedExplodedBomb{{OwnerID: 2, Cell: Cell{Row: 0, Col: 2}, BlastRowMin: 0, BlastRowMax: 1, BlastColMin: 2, BlastColMax: 2}}, []VerifiedMapElementHit{{MapElementID: 9011, Cell: wall}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	clone := engine.Clone()
	for engine.elapsedMS < 500 {
		if _, err := engine.Step(nil); err != nil {
			t.Fatal(err)
		}
	}
	if engine.grid.Cells[7].Kind != CellBreakable || engine.pickups[0].State != PickupHidden {
		t.Fatal("rounded event3ms down: wall/drop retired before503ms")
	}
	engine.actors[0].Position = PositionAtCellCenter(wall)
	if _, ok := engine.placeBomb(0); ok {
		t.Fatal("native producer accepted a bubble in a destroying map object")
	}
	if _, err := engine.Step(nil); err != nil {
		t.Fatal(err)
	}
	if engine.elapsedMS != 520 || engine.grid.Cells[7].Kind != CellOpen || engine.pickups[0].State != PickupAvailable {
		t.Fatal("native map/drop did not retire at first world boundary after503ms")
	}
	if _, ok := engine.placeBomb(0); !ok {
		t.Fatal("retired map object still blocked placement")
	}
	if clone.grid.Cells[7].Kind != CellBreakable || len(clone.pendingMapRetirements) != 1 {
		t.Fatal("retirement mutated cloned source")
	}
}

func TestNativeCachedChainDoesNotGrowWhenAnInterveningWallRetires(t *testing.T) {
	config := testConfig()
	config.Grid = testOpenGrid(7, 7)
	config.Participants[0].Spawn = Cell{Row: 0, Col: 0}
	config.Participants[1].Spawn = Cell{Row: 6, Col: 6}
	wall := Cell{Row: 2, Col: 3}
	config.Grid.Cells[17] = Tile{Kind: CellBreakable, MapElementOccupied: true, Durability: 1}
	engine := mustEngine(t, config)
	engine.commitBomb(Bomb{ID: 1, OwnerID: 1, Cell: Cell{Row: 1, Col: 3}, Power: 3, ExplodeAtMS: 600})
	engine.commitBomb(Bomb{ID: 2, OwnerID: 2, Cell: Cell{Row: 4, Col: 3}, Power: 1, ExplodeAtMS: 2000})
	if len(engine.nativeBombLinks[1]) != 0 {
		t.Fatal("wall did not block original cached link")
	}
	engine.beginMapRetirement(wall, []Cell{wall}, 0, 1, 0)
	timeline, err := engine.DangerTimeline(2100)
	if err != nil {
		t.Fatal(err)
	}
	if _, hit := timeline.ImpactAt(Cell{Row: 4, Col: 2}); hit {
		t.Fatal("forecast invented the unlisted bubble's own blast")
	}
	for engine.elapsedMS < 600 {
		if _, err := engine.Step(nil); err != nil {
			t.Fatal(err)
		}
	}
	events, err := engine.Step(nil)
	if err != nil {
		t.Fatal(err)
	}
	explosions := 0
	for _, e := range events {
		if e.Kind == EventBombExploded {
			explosions++
			if e.BombID != 1 {
				t.Fatal("retiring wall rebuilt the old chain")
			}
		}
	}
	if explosions != 1 || len(engine.bombs) != 0 || !hasFlame(engine.flames, Cell{Row: 4, Col: 3}) || hasFlame(engine.flames, Cell{Row: 4, Col: 2}) {
		t.Fatalf("native cached-list/current-cross mismatch was not preserved: %+v", events)
	}
}
