package probe

import (
	"io"
	"testing"

	"qqtang/internal/game/battleengine"
	"qqtang/internal/game/match"
	"qqtang/internal/protocol/game"
)

func TestCompetitiveAINativeBlastRetiresUnlistedIdentityAndIgnoresDuplicate(t *testing.T) {
	participants := []match.CompetitiveParticipant{
		{PlayerID: 1, RoleID: 1, TeamID: 1, Source: match.CompetitiveParticipantHuman},
		{PlayerID: 2, RoleID: 2, TeamID: 1, Source: match.CompetitiveParticipantHuman},
		{PlayerID: 20001, RoleID: 3, TeamID: 2, Source: match.CompetitiveParticipantVirtualAI},
		{PlayerID: 20002, RoleID: 4, TeamID: 2, Source: match.CompetitiveParticipantVirtualAI},
	}
	live, err := newLiveCompetitiveAIRuntime(7, testCompetitiveAIMap(4), game.GameBeginData{GameID: 109, SpawnSeed: 11, ItemSeed: 22}, participants, false,
		battleengine.PolicyFunc(func(o battleengine.Observation, _ []battleengine.Action) (battleengine.Action, error) {
			return battleengine.Action{PlayerID: o.PlayerID}, nil
		}), 100)
	if err != nil {
		t.Fatal(err)
	}
	battle, err := match.NewCompetitiveBattle(109, 2, 1, participants)
	if err != nil {
		t.Fatal(err)
	}
	server := &Server{competitiveAIRuntime: map[uint32]*liveCompetitiveAIRuntime{109: live}, competitiveBattles: map[uint32]*match.CompetitiveBattle{109: battle}, logWriter: io.Discard}
	place := func(cell battleengine.Cell, stamp uint32) uint32 {
		e, err := live.runtime.AcceptHumanBombPlacement(1, cell, 2, 0, stamp)
		if err != nil {
			t.Fatal(err)
		}
		live.bombs[e.BombID] = liveCompetitiveAIBomb{ownerID: 1, placedAt: stamp, position: cell, power: 2}
		return e.BombID
	}
	root := place(battleengine.Cell{Row: 1, Col: 0}, 2990)
	unlisted := place(battleengine.Cell{Row: 1, Col: 1}, 2995)
	survivor := place(battleengine.Cell{Row: 0, Col: 1}, 2998)
	identity := live.bombs[survivor]
	body, err := (game.BombExplodeEvent{PlayerID: 1, ClientTime: 3000, Bombs: []game.ExplodedBomb{{PlayerID: 1, ClientTime: 2990, BombID: 1, Row: 1, Column: 0, RowMin: 1, RowMax: 1, ColumnMin: 0, ColumnMax: 1}}}).MarshalNetworkBinary()
	if err != nil {
		t.Fatal(err)
	}
	packet := game.GameplayDataPackage{PlayerID: 2, GameID: 109, Time: 3000, Messages: []game.BattleMessageData{{DataID: game.NotifyBombExplode, Time: 3000, Sequence: 17, Data: body}}}
	server.recordCompetitiveAIHumanPackages(109, []game.GameplayDataPackage{packet})
	if len(live.runtime.EngineSnapshot().Bombs()) != 3 || len(live.bombs) != 3 {
		t.Fatal("non-authority sender removed bubbles")
	}
	packet.PlayerID = 1
	server.recordCompetitiveAIHumanPackages(109, []game.GameplayDataPackage{packet})
	remaining := live.runtime.EngineSnapshot().Bombs()
	if len(remaining) != 1 || remaining[0].ID != survivor {
		t.Fatalf("mirror retained wrong bubbles: %+v", remaining)
	}
	if _, exists := live.bombs[root]; exists {
		t.Fatal("explicit root identity survived")
	}
	if _, exists := live.bombs[unlisted]; exists {
		t.Fatal("blast-cleared unlisted identity survived")
	}
	if len(live.bombs) != 1 || live.bombs[survivor] != identity {
		t.Fatalf("survivor identity changed: %+v", live.bombs)
	}
	newID := place(battleengine.Cell{Row: 1, Col: 1}, 3001)
	server.recordCompetitiveAIHumanPackages(109, []game.GameplayDataPackage{packet})
	packet.Time = 3001 // Different outer envelope, same authenticated explosion.
	server.recordCompetitiveAIHumanPackages(109, []game.GameplayDataPackage{packet})
	if len(live.runtime.EngineSnapshot().Bombs()) != 2 || len(live.bombs) != 2 || live.bombs[newID].placedAt != 3001 {
		t.Fatal("duplicate old explosion retired a later placement")
	}
}
