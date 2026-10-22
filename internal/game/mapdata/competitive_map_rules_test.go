package mapdata

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"qqtang/internal/clientdata/sceneelement"
)

func TestMapTransformationRulesDefaultEnabledAndExplicitFalse(t *testing.T) {
	catalog := &Catalog{competitiveMaps: map[uint32]CompetitiveMap{
		211: {ID: 211}, 212: {ID: 212}, 5: {ID: 5},
	}}
	if err := catalog.loadCompetitiveMapRules(filepath.Join(t.TempDir(), CompetitiveMapRulesFile)); err != nil {
		t.Fatal(err)
	}
	for _, contents := range []string{
		`{"schema_version":1,"maps":[]}`,
		`{"schema_version":1,"maps":[{"map_id":211,"map_name":"景阳岗"},{"map_id":212,"transformation_items_enabled":true}]}`,
	} {
		if err := catalog.applyCompetitiveMapRules(strings.NewReader(contents)); err != nil {
			t.Fatal(err)
		}
		for _, entry := range catalog.competitiveMaps {
			if entry.TransformationItemsDisabled {
				t.Fatal("absent/true switch disabled transformations")
			}
		}
	}
	if err := catalog.applyCompetitiveMapRules(strings.NewReader(`{"schema_version":1,"maps":[{"map_id":211,"map_name":"景阳岗","transformation_items_enabled":false}]}`)); err != nil {
		t.Fatal(err)
	}
	if !catalog.competitiveMaps[211].TransformationItemsDisabled || catalog.competitiveMaps[212].TransformationItemsDisabled || catalog.competitiveMaps[5].TransformationItemsDisabled {
		t.Fatal("map override escaped its identity")
	}
}

func TestMapTransformationRulesRejectInvalidFileWithoutPartialMutation(t *testing.T) {
	for _, contents := range []string{
		`{"schema_version":2,"maps":[]}`,
		`{"schema_version":1,"maps":[{"map_id":211,"transformation_items_enabled":"false"}]}`,
		`{"schema_version":1,"maps":[{"map_id":211,"transformation_items_enabled":false},{"map_id":999999}]}`,
		`{"schema_version":1,"maps":[{"map_id":211},{"map_id":211}]}`,
		`{"schema_version":1,"maps":[]} {}`,
		`{"schema_version":1,"maps":[{"map_id":211,"transformation_item_enabled":false}]}`,
	} {
		catalog := &Catalog{competitiveMaps: map[uint32]CompetitiveMap{211: {ID: 211}}}
		if err := catalog.applyCompetitiveMapRules(strings.NewReader(contents)); err == nil {
			t.Fatalf("invalid config accepted: %s", contents)
		}
		if catalog.competitiveMaps[211].TransformationItemsDisabled {
			t.Fatal("invalid config partially changed the map")
		}
	}
}

func TestMapTransformationFilterCoversNativeSpeciesAndPreservesOtherRandomDraws(t *testing.T) {
	entry := CompetitiveMap{ID: 211, NativeRule: 1, Rule: CompetitiveRuleOrdinary,
		HiddenItemCells: make([]CompetitiveCell, 100)}
	entry.WallItemRules = []CompetitiveWallItemRule{
		{SceneID: 1, Minimum: 8, Maximum: 8, Probability: 1},
		{SceneID: 23, Minimum: 1, Maximum: 4, Probability: .6},
	}
	for _, id := range []uint32{101, 104, 107, 108, 109, 110, 114, 115} {
		entry.WallItemRules = append(entry.WallItemRules, CompetitiveWallItemRule{SceneID: id, Minimum: 1, Maximum: 3, Probability: .7})
	}
	entry.WallItemRules = append(entry.WallItemRules, CompetitiveWallItemRule{SceneID: 27, Minimum: 1, Maximum: 4, Probability: .6})
	disabled := entry
	disabled.TransformationItemsDisabled = true
	for seed := uint32(1); seed <= 128; seed++ {
		// Ample capacity isolates RNG preservation from intended reallocation.
		original := entry.RollWallItems(seed, 4, 100, 50)
		var expected []CompetitiveWallItem
		for _, item := range original {
			if !sceneelement.IsTransformationPickup(sceneelement.ID(item.SceneID)) {
				expected = append(expected, item)
			}
		}
		if got := disabled.RollWallItems(seed, 4, 100, 50); !reflect.DeepEqual(got, expected) {
			t.Fatalf("seed %d unrelated drops/rng changed: got=%+v want=%+v", seed, got, expected)
		}
		for _, boss := range []bool{false, true} {
			plan := disabled.PlanCompetitiveSupply(seed, []uint16{1, 2, 3, 4}, boss)
			for _, item := range append(plan.WallItems, plan.DelayedItems...) {
				if sceneelement.IsTransformationPickup(sceneelement.ID(item.SceneID)) {
					t.Fatalf("disabled transformation reached final supply: %+v", item)
				}
			}
		}
	}
	box := CompetitiveMap{ID: 1401, NativeRule: 8, Rule: CompetitiveRuleBox,
		WallItemSource: CompetitiveWallItemsLocalRuleRecovery, WallItemRules: recoveredBoxWallItemRules()}
	if !containsTransformation(box.RollWallItems(1, 4, 100, 50)) {
		t.Fatal("default box policy removed axe transformation")
	}
	box.TransformationItemsDisabled = true
	if containsTransformation(box.RollFinalCompetitiveOrdinaryWallItems(1, 4, 100, 50)) {
		t.Fatal("recovered box path ignored disabled switch")
	}
}

func containsTransformation(items []CompetitiveWallItem) bool {
	for _, item := range items {
		if sceneelement.IsTransformationPickup(sceneelement.ID(item.SceneID)) {
			return true
		}
	}
	return false
}

func TestDisabledTransformationsAreFilteredBeforeCapacityAllocation(t *testing.T) {
	for _, box := range []bool{false, true} {
		entry := CompetitiveMap{ID: 211, NativeRule: 1, Rule: CompetitiveRuleOrdinary,
			TransformationItemsDisabled: true,
			WallItemRules: []CompetitiveWallItemRule{
				{SceneID: 101, Minimum: 1, Maximum: 1, Probability: 1},
				{SceneID: 23, Minimum: 1, Maximum: 1, Probability: 1},
			}}
		if box {
			entry.NativeRule, entry.Rule = 8, CompetitiveRuleBox
			entry.WallItemSource = CompetitiveWallItemsLocalRuleRecovery
		}
		for seed := uint32(1); seed <= 64; seed++ {
			got := entry.RollWallItems(seed, 4, 1, 0)
			want := []CompetitiveWallItem{{SceneID: 23, Quantity: 1}}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("box=%t seed=%d disabled item consumed sole cell: %+v", box, seed, got)
			}
		}
	}
}

func TestMapTransformationRulesOptionalFileIsActuallyRead(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "data"), 0755); err != nil {
		t.Fatal(err)
	}
	contents := `{"schema_version":1,"maps":[{"map_id":211,"map_name":"景阳岗","transformation_items_enabled":false}]}`
	path := filepath.Join(root, "data", CompetitiveMapRulesFile)
	if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}
	catalog := &Catalog{competitiveMaps: map[uint32]CompetitiveMap{211: {ID: 211}}}
	if err := catalog.loadCompetitiveMapRules(path); err != nil {
		t.Fatal(err)
	}
	if !catalog.competitiveMaps[211].TransformationItemsDisabled {
		t.Fatal("config file did not reach map policy")
	}
	if got := DefaultCompetitiveMapRulesPath(filepath.Join(root, "runtime", "client-patched")); got != path {
		t.Fatalf("release data path=%s, want %s", got, path)
	}
}
