package probe

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"qqtang/internal/game/craftcatalog"
	"qqtang/internal/game/itemcatalog"
	"qqtang/internal/protocol/game"
)

func bubbleCosmeticRoster() []competitiveAIRoomProjection {
	result := make([]competitiveAIRoomProjection, 7)
	for index := range result {
		result[index] = competitiveAIRoomProjection{
			UIN: competitiveAIUINBase + uint32(index+20001), PlayerID: uint16(index + 20001),
			RoleID: byte(index + 1), TeamID: byte(index + 2), SeatID: byte(index + 2),
			Nickname: fmt.Sprintf("AI%d", index+1),
		}
	}
	return result
}

var testBubbleCostumeIDs = []uint16{119, 153, 154, 244, 256, 257, 414, 1005}

func TestCompetitiveAIBubbleCostumePoolUsesOnlyCanonicalInventoryIDs(t *testing.T) {
	entries := []itemcatalog.Entry{
		{ID: 153, RegistryCategory: "bomb"}, {ID: 119, RegistryCategory: "bomb"},
		{ID: 119, RegistryCategory: "bomb"}, {ID: 99, RegistryCategory: "cap"},
		{ID: 18, Kind: "bomb"}, {ID: 0, RegistryCategory: "bomb"}, {ID: 65536, RegistryCategory: "bomb"},
	}
	if got := competitiveAIBubbleCostumeIDs(entries); !reflect.DeepEqual(got, []uint16{119, 153}) {
		t.Fatalf("canonical costume pool = %v", got)
	}
}

func TestCompetitiveAIBubbleCosmeticsKeepDistinctStableRoomEquipment(t *testing.T) {
	forge := &craftcatalog.Catalog{
		Items: map[uint16]craftcatalog.ForgeItem{}, Effects: map[byte]craftcatalog.EffectForm{
			16: {Class: craftcatalog.ForgeBomb, Level: 1},
			20: {Class: craftcatalog.ForgeBomb, Level: 5},
			1:  {Class: craftcatalog.ForgeCap, Level: 1},
		}, Colors: map[craftcatalog.ForgeColor]string{
			craftcatalog.ForgeColorWhite: "white", craftcatalog.ForgeColorBlack: "black", craftcatalog.ForgeColorBlue: "blue",
		},
	}
	for _, id := range testBubbleCostumeIDs {
		forge.Items[id] = craftcatalog.ForgeItem{ItemID: id, Class: craftcatalog.ForgeBomb}
	}
	before, first, second := bubbleCosmeticRoster(), bubbleCosmeticRoster(), bubbleCosmeticRoster()
	assignCompetitiveAIBubbleCosmetics(first, testBubbleCostumeIDs, forge, 325)
	assignCompetitiveAIBubbleCosmetics(second, testBubbleCostumeIDs, forge, 325)
	if !reflect.DeepEqual(first, second) {
		t.Fatal("same match seed changed room equipment")
	}
	seen := map[uint16]bool{}
	for index, projection := range first {
		item := projection.BubbleCosmetic
		if seen[item.ItemID] || !item.Active() || item.ItemStatus != game.ItemStatusActive || item.ItemRoleID != projection.RoleID {
			t.Fatalf("invalid or duplicate equipment: %+v", projection)
		}
		seen[item.ItemID] = true
		if (item.ItemEffect != 16 && item.ItemEffect != 20) || (item.ItemColor != 1 && item.ItemColor != 2) {
			t.Fatalf("effect/color escaped native bomb forms: %+v", item)
		}
		notification, err := projection.enterNotification(9)
		if err != nil || len(notification.Player.Items) != 1 || notification.Player.Items[0] != item {
			t.Fatalf("equipment lost on room entry: %+v, %v", notification, err)
		}
		wire, err := notification.MarshalNetworkBinary()
		if err != nil || bytes.Count(wire, item.AppendNetworkBinary(nil)) != 1 {
			t.Fatalf("equipped ITEM_INFO missing from native room wire: %v", err)
		}
		decoded, err := game.ParseItemInfoNetwork(item.AppendNetworkBinary(nil))
		if err != nil || decoded != item {
			t.Fatalf("forged equipment wire mismatch: %+v, %v", decoded, err)
		}
		projection.BubbleCosmetic = game.ItemInfo{}
		if projection != before[index] {
			t.Fatal("cosmetic selection changed room identity/seat/team")
		}
	}
}

func TestCompetitiveAIBubbleCosmeticsRespectAvailableCatalog(t *testing.T) {
	roster := bubbleCosmeticRoster()
	assignCompetitiveAIBubbleCosmetics(roster, testBubbleCostumeIDs, nil, 2)
	if roster[0].BubbleCosmetic.ItemID == 0 || roster[0].BubbleCosmetic.ItemEffect != 0 {
		t.Fatal("missing optional forge catalog prevented basic appearance")
	}
	restricted := &craftcatalog.Catalog{Items: map[uint16]craftcatalog.ForgeItem{
		119: {Class: craftcatalog.ForgeCap}, 153: {Class: craftcatalog.ForgeBomb},
	}}
	assignCompetitiveAIBubbleCosmetics(roster, testBubbleCostumeIDs, restricted, 2)
	for _, projection := range roster {
		if projection.BubbleCosmetic.ItemID != 153 {
			t.Fatal("selected an unavailable or wrong-category item")
		}
	}
	empty := bubbleCosmeticRoster()
	assignCompetitiveAIBubbleCosmetics(empty, testBubbleCostumeIDs, &craftcatalog.Catalog{}, 2)
	if !reflect.DeepEqual(empty, bubbleCosmeticRoster()) {
		t.Fatal("empty cosmetic pool changed room state")
	}
	assignCompetitiveAIBubbleCosmetics(nil, testBubbleCostumeIDs, restricted, 2)
}

func TestInstalledAIBubbleCosmeticsAndAllBubbleForgeEntries(t *testing.T) {
	root := os.Getenv("QQT_AI_TEST_CLIENT_ROOT")
	if root == "" {
		root = filepath.Join("..", "..", "..", "runtime", "client-patched")
		if _, err := os.Stat(filepath.Join(root, "object", "itemCFG.py")); os.IsNotExist(err) {
			t.Skip("client resources not installed; set QQT_AI_TEST_CLIENT_ROOT to run this integration check")
		}
	}
	forge, err := craftcatalog.LoadForge(root, filepath.Join("..", "..", "..", "data", "qqt_forge_rules.json"))
	if err != nil {
		t.Fatal(err)
	}
	_, registry, err := itemcatalog.LoadItemRegistry(root)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := itemcatalog.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	costumes := competitiveAIBubbleCostumeIDs(entries)
	byID := map[uint32]itemcatalog.RegistryEntry{}
	bombCount := 0
	for _, item := range registry {
		byID[item.ItemID] = item
		if item.Category == "bomb" {
			bombCount++
			if definition, ok := forge.Items[uint16(item.ItemID)]; !ok || definition.Class != craftcatalog.ForgeBomb {
				t.Fatalf("canonical bubble %d/%s missing from bomb forge list", item.ItemID, item.Name)
			}
		}
	}
	if bombCount == 0 || len(costumes) != bombCount {
		t.Fatalf("canonical bubble pool mismatch: pool=%d registry=%d", len(costumes), bombCount)
	}
	readResource := func(relative string) {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(root, "object", filepath.FromSlash(relative)))
		if os.IsNotExist(err) {
			archive, archiveErr := itemcatalog.OpenObjectArchive(root)
			if archiveErr != nil {
				t.Fatal(archiveErr)
			}
			data, err = archive.Read(relative)
		}
		if err != nil || len(data) == 0 {
			t.Fatalf("missing native appearance resource %s: %v", relative, err)
		}
	}
	for _, id := range costumes {
		item := byID[uint32(id)]
		if item.Category != "bomb" {
			t.Fatalf("AI appearance %d is not a canonical bubble", id)
		}
		readResource(fmt.Sprintf("bomb/bomb%d_stand.img", item.ResourceID))
	}
	for level := 1; level <= 5; level++ {
		for _, color := range []string{"black", "blue", "red", "green", "purple"} {
			readResource(fmt.Sprintf("avatareffect/effect/bomb%d_%s.eff", level, color))
		}
	}
	levels, colors, skins := map[byte]bool{}, map[byte]bool{}, map[uint16]bool{}
	for seed := uint32(1); seed <= 1024; seed++ {
		roster := bubbleCosmeticRoster()
		assignCompetitiveAIBubbleCosmetics(roster, costumes, forge, seed)
		seen := map[uint16]bool{}
		for _, projection := range roster {
			item := projection.BubbleCosmetic
			form := forge.Effects[item.ItemEffect]
			if form.Class != craftcatalog.ForgeBomb || form.Level < 1 || form.Level > 5 || item.ItemColor < 1 || item.ItemColor > 5 {
				t.Fatalf("invalid native forge selection: %+v", item)
			}
			if seen[item.ItemID] {
				t.Fatal("same-match bubble costumes repeat")
			}
			seen[item.ItemID] = true
			levels[form.Level], colors[item.ItemColor], skins[item.ItemID] = true, true, true
		}
	}
	if len(levels) != 5 || len(colors) != 5 || len(skins) != len(costumes) {
		t.Fatalf("selection did not vary: levels=%v colors=%v skins=%v", levels, colors, skins)
	}
	t.Logf("all %d canonical bubbles forgeable; %d AI skins and 25 color/level resources present", bombCount, len(skins))
}
