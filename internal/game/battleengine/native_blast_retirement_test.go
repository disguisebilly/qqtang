package battleengine

import "testing"

func TestVerifiedNativeBlastRetiresUnlistedBubblesOnlyWithinReportedCross(t *testing.T) {
	for _, knownRoot := range []bool{false, true} {
		config := testConfig()
		config.Grid = testOpenGrid(9, 9)
		config.Rules.NativeOutcomeAuthority = true
		engine := mustEngine(t, config)
		cells := []Cell{{4, 4}, {4, 2}, {4, 6}, {2, 4}, {6, 4}, {2, 2}, {6, 6}, {4, 7}}
		for i, cell := range cells {
			engine.bombs = append(engine.bombs, Bomb{ID: uint32(i + 1), OwnerID: 2, Cell: cell, Power: 8, ExplodeAtMS: 90_000})
		}
		// Stacked and flying objects are still registered in their destination
		// cell in the native manager; cell retirement removes them too.
		engine.bombs = append(engine.bombs, Bomb{ID: 9, OwnerID: 1, Cell: cells[1], Power: 8, FlightUntilMS: 5_000})
		rootID := uint32(0)
		if knownRoot {
			rootID = 1
		}
		events, err := engine.ApplyVerifiedBombExplosion([]VerifiedExplodedBomb{{
			BombID: rootID, OwnerID: 1, Cell: Cell{4, 4},
			BlastRowMin: 2, BlastRowMax: 6, BlastColMin: 2, BlastColMax: 6,
		}}, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(engine.bombs) != 3 || engine.bombs[0].ID != 6 || engine.bombs[1].ID != 7 || engine.bombs[2].ID != 8 {
			t.Fatalf("rootKnown=%v: bubbles outside the cross or inside it were reconciled incorrectly: %+v", knownRoot, engine.bombs)
		}
		if len(events) != 1 || events[0].Kind != EventBombExploded || events[0].BombID != rootID {
			t.Fatalf("cell retirement invented chain explosions: %+v", events)
		}
		if len(engine.flames) != 9 || hasFlame(engine.flames, Cell{4, 7}) || hasFlame(engine.flames, Cell{2, 2}) {
			t.Fatalf("cell retirement expanded the confirmed blast: %+v", engine.flames)
		}
		for _, actor := range engine.actors {
			if actor.State != ActorActive {
				t.Fatalf("cell retirement committed actor outcome: %+v", actor)
			}
		}
	}
}

func TestVerifiedNativeBlastRejectsMalformedBatchBeforeRetiringBubbles(t *testing.T) {
	engine := mustEngine(t, testConfig())
	engine.bombs = []Bomb{{ID: 1, OwnerID: 1, Cell: Cell{1, 1}, Power: 2, ExplodeAtMS: 9000}}
	_, err := engine.ApplyVerifiedBombExplosion([]VerifiedExplodedBomb{
		{BombID: 1, OwnerID: 1, Cell: Cell{1, 1}, BlastRowMin: 1, BlastRowMax: 1, BlastColMin: 1, BlastColMax: 2},
		{OwnerID: 2, Cell: Cell{1, 1}, BlastRowMin: -1, BlastRowMax: 1, BlastColMin: 1, BlastColMax: 2},
	}, nil, nil)
	if err == nil || len(engine.bombs) != 1 || len(engine.flames) != 0 {
		t.Fatalf("malformed batch partly committed: err=%v bombs=%+v flames=%+v", err, engine.bombs, engine.flames)
	}
}
