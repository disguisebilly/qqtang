package mapdata

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestBoxRecoveryOnlyFillsMissingRule8Tables(t *testing.T) {
	kept := []CompetitiveWallItemRule{{SceneID: 3, Minimum: 1, Maximum: 1, Probability: 1}}
	c := &Catalog{competitiveMaps: map[uint32]CompetitiveMap{
		1401: {ID: 1401, NativeRule: 8, Rule: CompetitiveRuleBox},
		1402: {ID: 1402, NativeRule: 8, Rule: CompetitiveRuleBox, WallItemRules: kept, WallItemSource: CompetitiveWallItemsEmbedded},
		1403: {ID: 1403, NativeRule: 8, Rule: CompetitiveRuleBox, RequiredItemField: 1},
		1:    {ID: 1, NativeRule: 1, Rule: CompetitiveRuleOrdinary},
	}}
	c.restoreCompetitiveBoxWallItemRules()
	if c.competitiveMaps[1401].WallItemSource != CompetitiveWallItemsLocalRuleRecovery || len(c.competitiveMaps[1401].WallItemRules) != 10 {
		t.Fatal("empty box map not restored")
	}
	if !reflect.DeepEqual(c.competitiveMaps[1402].WallItemRules, kept) || c.competitiveMaps[1402].WallItemSource != CompetitiveWallItemsEmbedded {
		t.Fatal("original table overwritten")
	}
	if len(c.competitiveMaps[1403].WallItemRules) != 0 || len(c.competitiveMaps[1].WallItemRules) != 0 {
		t.Fatal("recovery escaped rule/field scope")
	}
}

func TestInstalledBoxSupplyIsFiniteReproducibleAndContainsBothGloves(t *testing.T) {
	root := os.Getenv("QQT_AI_TEST_CLIENT_ROOT")
	if root == "" {
		root = filepath.Join("..", "..", "..", "runtime", "client-patched")
		if _, err := os.Stat(filepath.Join(root, "map", "mapDesc.py")); os.IsNotExist(err) {
			t.Skip("client resources not installed; set QQT_AI_TEST_CLIENT_ROOT to run this integration check")
		}
	}
	c, err := LoadCatalog(root)
	if err != nil {
		t.Fatal(err)
	}
	for id := uint32(1401); id <= 1409; id++ {
		e, ok := c.CompetitiveMap(id)
		if !ok || e.WallItemSource != CompetitiveWallItemsLocalRuleRecovery {
			t.Fatalf("map %d: %+v", id, e)
		}
		optionalSeen := map[int]bool{}
		for seed := uint32(1); seed <= 256; seed++ {
			got := e.PlanCompetitiveSupply(seed, []uint16{7, 8, 9, 10, 11, 12, 13, 14}, false)
			if !reflect.DeepEqual(got, e.PlanCompetitiveSupply(seed, []uint16{7, 8, 9, 10, 11, 12, 13, 14}, false)) {
				t.Fatal("same seed changed supply")
			}
			q := competitiveWallItemQuantities(got.WallItems)
			if competitiveWallItemTotal(got.WallItems) > len(e.HiddenItemCells) || len(got.DelayedItems) != 0 {
				t.Fatalf("unbounded supply on %d", id)
			}
			for _, r := range recoveredBoxWallItemRules() {
				if q[r.SceneID] < int(r.Minimum) || q[r.SceneID] > int(r.Maximum) {
					t.Fatalf("map %d seed%d scene%d quantity%d outside%d..%d", id, seed, r.SceneID, q[r.SceneID], r.Minimum, r.Maximum)
				}
			}
			optionalSeen[q[4]] = true
		}
		if !optionalSeen[0] || !optionalSeen[1] {
			t.Fatalf("optional pickup did not cover0/1 on %d: %v", id, optionalSeen)
		}
		if len(e.RollWallItems(1, 8, 1, 0)) != 1 {
			t.Fatal("tiny physical capacity was not enforced")
		}
	}
}
