package battleengine

import "testing"

func TestNativeRetiringWallBlocksDartAndAIProjectionUntilRemoval(t *testing.T) {
	config := testConfig()
	cell := Cell{Row: 1, Col: 2}
	config.Grid.Cells[7] = Tile{Kind: CellBreakable, Durability: 1, MapElementOccupied: true}
	engine := mustEngine(t, config)
	engine.actors[0].Facing = DirectionRight
	engine.commitBomb(Bomb{ID: 1, OwnerID: 2, Cell: Cell{Row: 1, Col: 3}, Power: 1, ExplodeAtMS: 3000})
	engine.beginMapRetirement(cell, []Cell{cell}, 0, 2, 0)
	engine.elapsedMS = 499
	engine.expireMapRetirements()
	if _, id, _ := engine.nativeActionProjectileTarget(&engine.actors[0]); id != 0 {
		t.Fatal("dart targeted through a map object still in its native removal countdown")
	}
	blocked, err := engine.ProjectNativeMovement(1, DirectionRight)
	if err != nil {
		t.Fatal(err)
	}
	engine.elapsedMS = 500
	engine.expireMapRetirements()
	if _, id, _ := engine.nativeActionProjectileTarget(&engine.actors[0]); id != 1 {
		t.Fatal("retired map object still blocks dart target")
	}
	open, err := engine.ProjectNativeMovement(1, DirectionRight)
	if err != nil {
		t.Fatal(err)
	}
	if open.End.X <= blocked.End.X {
		t.Fatalf("AI kept stale wall endpoint: before=%+v after=%+v", blocked, open)
	}
}

func TestDartDoesNotRetargetBubbleCreatedAfterNativeCrossRetirement(t *testing.T) {
	config := testConfig()
	config.Rules.NativeOutcomeAuthority = true
	engine := mustEngine(t, config)
	cell := Cell{Row: 1, Col: 2}
	engine.commitBomb(Bomb{ID: 1, OwnerID: 2, Cell: cell, Power: 1, ExplodeAtMS: 3000})
	engine.projectiles = []ActionProjectile{{ID: 1, ActionID: 36, OwnerID: 1, TargetBombID: 1, TargetCell: cell, ResolveAtMS: 100}}
	_, err := engine.ApplyVerifiedBombExplosionAt(20, []VerifiedExplodedBomb{{OwnerID: 2, Cell: Cell{Row: 0, Col: 2}, BlastRowMin: 0, BlastRowMax: 1, BlastColMin: 2, BlastColMax: 2}}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	engine.commitBomb(Bomb{ID: 2, OwnerID: 2, Cell: cell, Power: 1, ExplodeAtMS: 4000})
	engine.elapsedMS = 100
	engine.resolveActionProjectiles()
	if len(engine.projectiles) != 0 || len(engine.bombs) != 1 || engine.bombs[0].ID != 2 || engine.bombs[0].ExplodeAtMS != 4000 {
		t.Fatalf("retired target resurrected or replacement triggered: %+v", engine.bombs)
	}
}

func TestMapRetirementPreservesNewDeliveriesAndConfirmedDrops(t *testing.T) {
	for _, kind := range []string{"landed_field", "pending_field", "death_drop"} {
		t.Run(kind, func(t *testing.T) {
			config := testConfig()
			cell := Cell{Row: 1, Col: 2}
			config.Grid.Cells[7] = Tile{Kind: CellBreakable, Durability: 1, MapElementOccupied: true}
			config.Pickups = []Pickup{{SceneID: SceneBombPowerSmall, Cell: cell, State: PickupHidden}}
			engine := mustEngine(t, config)
			engine.beginMapRetirement(cell, []Cell{cell}, 0, 1, 0)
			switch kind {
			case "landed_field":
				engine.retirePickupsAtCell(cell)
				engine.installAuthoritativeFieldObject(42, cell)
			case "pending_field":
				engine.retirePickupsAtCell(cell)
				engine.pendingPickupDispatches = []pendingPickupDispatch{{Pickup: Pickup{SceneID: 42, Cell: cell, State: PickupAvailable}, ActivateAtMS: 520}}
			case "death_drop":
				engine.installDeathDrops(2, []Pickup{{SceneID: SceneBombCapacitySmall, Cell: cell, State: PickupAvailable}})
			}
			engine.elapsedMS = 500
			engine.expireMapRetirements()
			if kind == "pending_field" {
				if len(engine.pendingPickupDispatches) != 1 {
					t.Fatal("wall retirement cancelled in-flight delivery")
				}
				engine.elapsedMS = 520
				engine.activatePendingPickupDispatches()
			}
			if kind != "death_drop" && (len(engine.fieldObjects) != 1 || engine.fieldObjects[0].ActionID != 42) {
				t.Fatal("wall retirement removed a later delivery")
			}
			available := 0
			for _, p := range engine.pickups {
				if p.State == PickupAvailable {
					available++
					if kind != "death_drop" || p.SceneID != SceneBombCapacitySmall {
						t.Fatal("replaced hidden pickup resurrected")
					}
				}
			}
			if kind == "death_drop" && available != 1 {
				t.Fatal("confirmed death drop lost or duplicated")
			}
		})
	}
}

func TestNativeBlastDoesNotDestroyItemsBehindStillPresentMapObject(t *testing.T) {
	config := testConfig()
	cell := Cell{Row: 1, Col: 2}
	config.Grid.Cells[7] = Tile{Kind: CellBreakable, Durability: 1, MapElementOccupied: true}
	engine := mustEngine(t, config)
	// A later authority-confirmed item can coexist with the mirror's pending
	// map removal; initial offline layouts deliberately reject this state.
	engine.installAuthoritativePickup(Pickup{SceneID: SceneBombPowerSmall, Cell: cell, State: PickupAvailable})
	engine.commitBomb(Bomb{ID: 1, OwnerID: 1, Cell: Cell{Row: 1, Col: 1}, Power: 2, ExplodeAtMS: 0})
	engine.explodeDueBombs()
	if engine.pickups[0].State != PickupAvailable || len(engine.recycledPickupSceneIDs) != 0 {
		t.Fatal("blast queued an item hidden behind a still-present map layer")
	}
	finishMapRetirementsForTest(engine)
	if engine.pickups[0].State != PickupAvailable {
		t.Fatal("map expiry destroyed item without a native item destruction")
	}
}
