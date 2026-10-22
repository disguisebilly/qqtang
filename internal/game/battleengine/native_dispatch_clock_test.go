package battleengine

import "testing"

func TestNativeDeliveryUsesConfirmedSceneClockAcrossMapRetirement(t *testing.T) {
	config := testConfig()
	config.Rules.NativeOutcomeAuthority = true
	cell := Cell{Row: 1, Col: 2}
	config.Grid.Cells[7] = Tile{Kind: CellBreakable, Durability: 1, MapElementOccupied: true}
	engine := mustEngine(t, config)
	engine.beginMapRetirement(cell, []Cell{cell}, 3, 2, 0)
	engine.elapsedMS = 500 // next20msmirror frame has not yet run
	drops := []Pickup{{SceneID: SceneBombCapacitySmall, Cell: cell, State: PickupAvailable}}
	if err := engine.ApplyVerifiedPickupDispatch(502, drops); err == nil {
		t.Fatal("accepted a delivery before confirmed native map removal deadline")
	}
	if err := engine.ApplyVerifiedPickupDispatch(504, drops); err != nil {
		t.Fatal("rejected a confirmed delivery solely because mirror frame trails scene clock:", err)
	}
	if engine.grid.Cells[7].Kind != CellBreakable || len(engine.pendingPickupDispatches) != 1 {
		t.Fatal("delivery advanced simulation clock or lost native pending target")
	}
	engine.elapsedMS = 520
	engine.expireMapRetirements()
	if len(engine.pendingPickupDispatches) != 1 {
		t.Fatal("map expiry cancelled authoritative delivery")
	}
}
