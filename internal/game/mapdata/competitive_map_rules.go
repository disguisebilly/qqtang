package mapdata

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// CompetitiveMapRulesFile is server-owned data, separate from native resources.
const CompetitiveMapRulesFile = "qqt_competitive_map_rules.json"

// DefaultCompetitiveMapRulesPath follows the release layout:
// <root>/runtime/client-patched and <root>/data. It is independent of cwd.
func DefaultCompetitiveMapRulesPath(clientRoot string) string {
	return filepath.Clean(filepath.Join(clientRoot, "..", "..", "data", CompetitiveMapRulesFile))
}

type CompetitiveMapRules struct {
	SchemaVersion uint32               `json:"schema_version"`
	Maps          []CompetitiveMapRule `json:"maps"`
}

type CompetitiveMapRule struct {
	MapID   uint32 `json:"map_id"`
	MapName string `json:"map_name"`
	// Only an explicit false suppresses transformation drops. Omitted fields,
	// omitted maps, and an absent optional config retain existing behavior.
	TransformationItemsEnabled *bool `json:"transformation_items_enabled,omitempty"`
}

func (catalog *Catalog) loadCompetitiveMapRules(path string) error {
	if path == "" {
		return nil
	}
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("open competitive map rules: %w", err)
	}
	defer file.Close()
	if err := catalog.applyCompetitiveMapRules(file); err != nil {
		return fmt.Errorf("load %s: %w", path, err)
	}
	return nil
}

func (catalog *Catalog) applyCompetitiveMapRules(reader io.Reader) error {
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	var rules CompetitiveMapRules
	if err := decoder.Decode(&rules); err != nil {
		return fmt.Errorf("decode competitive map rules: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("competitive map rules must contain one JSON object")
	}
	if rules.SchemaVersion != 1 {
		return fmt.Errorf("competitive map rules schema_version=%d, want 1", rules.SchemaVersion)
	}
	disabled := make(map[uint32]bool, len(rules.Maps))
	for _, rule := range rules.Maps {
		if _, ok := catalog.competitiveMaps[rule.MapID]; !ok {
			return fmt.Errorf("competitive map rules: unknown map_id %d (%s)", rule.MapID, rule.MapName)
		}
		if _, exists := disabled[rule.MapID]; exists {
			return fmt.Errorf("competitive map rules: duplicate map_id %d (%s)", rule.MapID, rule.MapName)
		}
		disabled[rule.MapID] = rule.TransformationItemsEnabled != nil && !*rule.TransformationItemsEnabled
	}
	// Validate the entire file before changing the catalog. The name is a
	// human-readable label; the numeric map identity controls the override.
	for id, entry := range catalog.competitiveMaps {
		entry.TransformationItemsDisabled = disabled[id]
		catalog.competitiveMaps[id] = entry
	}
	return nil
}
