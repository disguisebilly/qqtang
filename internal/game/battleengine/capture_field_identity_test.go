package battleengine

import "testing"

func TestNativeCaptureCarriesFieldIdentityWithoutInventingBodyHit(t *testing.T) {
	for _, transformed := range []bool{false, true} {
		engine := mustEngine(t, testConfig())
		actor := &engine.actors[0]
		if transformed {
			engine.collectPickup(actor, &Pickup{SceneID: 101, State: PickupAvailable, Cell: actor.Position.Cell()})
		}
		engine.fieldObjects = []FieldObject{{ID: 19, ActionID: 41, OwnerID: 2, Cell: actor.Position.Cell()}}
		events := engine.resolveFieldObjectContacts()
		traps := 0
		for _, e := range events {
			if e.Kind != EventActorTrapped {
				continue
			}
			traps++
			if e.ActionID != 41 || e.ObjectID != 19 || e.PlayerID != 2 || e.TargetID != actor.PlayerID || e.BombID != 0 {
				t.Fatalf("lost native field identity: %+v", e)
			}
		}
		want := 1
		if transformed {
			want = 0
		}
		if traps != want {
			t.Fatalf("traps=%d transformed=%t events=%+v", traps, transformed, events)
		}
		if len(engine.resolveFieldObjectContacts()) != 0 {
			t.Fatal("consumed capture was emitted again")
		}
	}
}
