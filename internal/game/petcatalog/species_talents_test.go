package petcatalog

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestRestoredCardlessSpeciesUseNativeInnateIDs(t *testing.T) {
	root := os.Getenv("QQT_AI_TEST_CLIENT_ROOT")
	if root == "" {
		root = filepath.Join("..", "..", "..", "runtime", "client-patched")
		if _, err := os.Stat(filepath.Join(root, "config", "PetCfg.ini")); os.IsNotExist(err) {
			t.Skip("client resources not installed; set QQT_AI_TEST_CLIENT_ROOT to run this integration check")
		}
	}
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	want := map[uint32][]byte{
		25050: {31, 33, 39}, 25040: {34, 39, 41}, 25902: {32, 31, 30},
		25043: {31, 30, 41}, 25054: {46, 34, 39}, 25045: {36, 12, 40},
		25035: {17, 23, 40}, 25042: {17, 45, 14}, 25047: {36, 17, 45},
		25055: {23, 17, 45}, 25905: {40, 23, 13}, 25049: {6, 48, 5},
		25046: {1, 6, 48}, 25048: {4, 3, 2}, 25051: {1, 4, 3}, 25052: {6, 48},
	}
	for _, d := range catalog.Definitions {
		if ids, restored := want[d.PetTypeID]; restored {
			if !slices.Equal(d.InnateSkills, ids) || d.Description == "" || d.TalentSource == "" {
				t.Fatalf("species %d = %+v, want talents %v", d.PetTypeID, d, ids)
			}
			delete(want, d.PetTypeID)
		}
		// Only flavour descriptions were supplied for these species.
		if slices.Contains([]uint32{25053, 25056, 25057, 25058, 25901, 25907, 25908, 25910}, d.PetTypeID) && len(d.InnateSkills) != 0 {
			t.Fatalf("invented talents for %d", d.PetTypeID)
		}
	}
	if len(want) != 0 {
		t.Fatalf("missing restored species: %v", want)
	}
	for _, card := range catalog.CardLinks {
		if card.PetTypeID == 25902 || card.PetTypeID == 25905 || card.PetTypeID == 25050 {
			t.Fatalf("invented inventory card for species %d", card.PetTypeID)
		}
	}
}

func TestExistingCardTalentsRemainAuthoritative(t *testing.T) {
	definitions := []Definition{{PetTypeID: 25050, Name: "格里芬"}}
	cards := []CardLink{{PetTypeID: 25050, Description: "original populated card", InnateSkills: []byte{3}}}
	if err := applySpeciesTalents(definitions, cards, nil); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(definitions[0].InnateSkills, []byte{3}) || definitions[0].Description != cards[0].Description {
		t.Fatalf("original card overwritten: %+v", definitions[0])
	}
}
