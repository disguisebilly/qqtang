package petcatalog

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
)

// Species talents also belong to pets without an itemCFG pet-card entry.
// Keep their identity in the PetTypeID namespace; do not invent inventory IDs.
//
//go:embed species_talents.json
var speciesTalentsJSON []byte

type speciesTalentRestoration struct {
	PetTypeID    uint32   `json:"pet_type_id"`
	TypeName     string   `json:"type_name"`
	ReportedName string   `json:"reported_name"`
	Description  string   `json:"description"`
	TalentNames  []string `json:"talent_names"`
}

func applySpeciesTalents(definitions []Definition, cards []CardLink, skills []SkillDefinition) error {
	byType := make(map[uint32]int, len(definitions))
	for i := range definitions {
		byType[definitions[i].PetTypeID] = i
	}
	for _, card := range cards {
		if i, ok := byType[card.PetTypeID]; ok {
			definitions[i].Description = card.Description
			definitions[i].InnateSkills = append([]byte(nil), card.InnateSkills...)
			if len(card.InnateSkills) > 0 {
				definitions[i].TalentSource = "installed-pet-card-description"
			}
		}
	}
	var table struct {
		SchemaVersion int                        `json:"schema_version"`
		Source        string                     `json:"source"`
		Pets          []speciesTalentRestoration `json:"pets"`
	}
	if err := json.Unmarshal(speciesTalentsJSON, &table); err != nil {
		return fmt.Errorf("decode species talents: %w", err)
	}
	if table.SchemaVersion != 1 {
		return fmt.Errorf("unsupported species talent schema %d", table.SchemaVersion)
	}
	seen := map[uint32]bool{}
	for _, entry := range table.Pets {
		if seen[entry.PetTypeID] {
			return fmt.Errorf("duplicate restored pet type %d", entry.PetTypeID)
		}
		seen[entry.PetTypeID] = true
		i, exists := byType[entry.PetTypeID]
		if !exists {
			continue
		} // A smaller/older installed catalog can omit a species.
		if definitions[i].Name != entry.TypeName {
			return fmt.Errorf("pet %d name %q differs from restored identity %q", entry.PetTypeID, definitions[i].Name, entry.TypeName)
		}
		if len(definitions[i].InnateSkills) != 0 {
			continue
		} // Original populated cards remain authoritative.
		innate, err := deriveInnateSkills("具有天赋："+strings.Join(entry.TalentNames, "，")+"。", skills)
		if err != nil {
			return fmt.Errorf("species %d (%s): %w", entry.PetTypeID, entry.TypeName, err)
		}
		definitions[i].Description = entry.Description
		definitions[i].InnateSkills = innate
		definitions[i].TalentSource = table.Source
	}
	return nil
}
