package mapdata

// The installed nine rule-8 maps have empty item tables. This is an explicitly
// local reconstruction from the user's historical item-count list, with native
// IDs/effects verified in Client.exe. It is not an embedded official drop table.
// Scene2 and7 use item802/item807 art in rule8 and control box push distance.
func recoveredBoxWallItemRules() []CompetitiveWallItemRule {
	return []CompetitiveWallItemRule{
		{SceneID: 25, Minimum: 2, Maximum: 2, Probability: 1},  // slow glue pickups
		{SceneID: 23, Minimum: 2, Maximum: 2, Probability: 1},  // banana pickups
		{SceneID: 1, Minimum: 10, Maximum: 16, Probability: 1}, // carried bubbles, not ordinary capacity
		{SceneID: 27, Minimum: 4, Maximum: 4, Probability: 1},  // dart pickups (native pickup grants2 uses)
		{SceneID: 115, Minimum: 1, Maximum: 4, Probability: 1}, // axe transformation
		{SceneID: 3, Minimum: 6, Maximum: 10, Probability: 1},
		{SceneID: 8, Minimum: 2, Maximum: 2, Probability: 1},
		{SceneID: 2, Minimum: 10, Maximum: 15, Probability: 1}, // ordinary glove
		{SceneID: 7, Minimum: 4, Maximum: 4, Probability: 1},   // star glove, upper-strength attribute
		{SceneID: 4, Minimum: 0, Maximum: 1, Probability: 1},   // optional invisible-bubble pickup
	}
}

func (catalog *Catalog) restoreCompetitiveBoxWallItemRules() {
	for id, entry := range catalog.competitiveMaps {
		if entry.Rule != CompetitiveRuleBox || entry.RequiredItemField != 0 || len(entry.WallItemRules) != 0 {
			continue
		}
		entry.WallItemRules = recoveredBoxWallItemRules()
		entry.WallItemSource = CompetitiveWallItemsLocalRuleRecovery
		catalog.competitiveMaps[id] = entry
	}
}

// Rule8's quantities describe a finite stock of pickups, not the ordinary
// three-stat progression floor. Roll every interval including its zero endpoint
// and cap by real hidden box cells. Capacity pressure uses the existing fair
// allocator so later entries cannot be starved by declaration order.
func (entry CompetitiveMap) rollRecoveredBoxWallItems(seed uint32, capacity int) []CompetitiveWallItem {
	if capacity <= 0 {
		return nil
	}
	state := seed ^ entry.ID*0x9E3779B9
	if state == 0 {
		state = 0xA341316C
	}
	next := func() uint32 { state ^= state << 13; state ^= state >> 17; state ^= state << 5; return state }
	quantities := make([]int16, len(entry.WallItemRules))
	for i, r := range entry.WallItemRules {
		if r.Minimum < 0 || r.Maximum < r.Minimum || r.Probability <= 0 {
			continue
		}
		if r.Probability < 1 && float64(next())/4294967296.0 >= float64(r.Probability) {
			continue
		}
		quantities[i] = r.Minimum + int16(next()%uint32(int(r.Maximum)-int(r.Minimum)+1))
		if entry.SuppressCompetitiveWallItem(r.SceneID) {
			quantities[i] = 0
		}
	}
	quantities = allocateWallItemQuantities(quantities, capacity, seed^entry.ID^0xA17E4E5D)
	items := make([]CompetitiveWallItem, 0, len(quantities))
	for i, q := range quantities {
		if q > 0 {
			items = append(items, CompetitiveWallItem{SceneID: entry.WallItemRules[i].SceneID, Quantity: q})
		}
	}
	return items
}
