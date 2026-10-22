package probe

import (
	"fmt"
	"slices"
	"sort"

	"qqtang/internal/game/craftcatalog"
	"qqtang/internal/game/itemcatalog"
	"qqtang/internal/protocol/game"
)

// Use canonical inventory identities, not resource IDs or XML-only names.
// Compute the immutable pool once at server startup from the client catalog.
func competitiveAIBubbleCostumeIDs(entries []itemcatalog.Entry) []uint16 {
	seen := make(map[uint16]bool)
	var result []uint16
	for _, entry := range entries {
		if entry.RegistryCategory != "bomb" || entry.ID == 0 || entry.ID > 65535 || seen[uint16(entry.ID)] {
			continue
		}
		id := uint16(entry.ID)
		seen[id] = true
		result = append(result, id)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}

func resolveCompetitiveAIBubbleCostumes(entries []itemcatalog.Entry, allowed []uint16) ([]uint16, error) {
	canonical := competitiveAIBubbleCostumeIDs(entries)
	known := make(map[uint16]bool, len(canonical))
	// Zero is a configuration choice for the plain default bubble. It never
	// becomes an equipped inventory item.
	known[0] = true
	for _, id := range canonical {
		known[id] = true
	}
	seen := make(map[uint16]bool, len(allowed))
	result := make([]uint16, 0, len(allowed))
	for _, id := range allowed {
		if !known[id] {
			return nil, fmt.Errorf("competitive_ai.bubble_costume_item_ids contains non-bubble inventory ID %d", id)
		}
		if seen[id] {
			return nil, fmt.Errorf("competitive_ai.bubble_costume_item_ids repeats inventory ID %d", id)
		}
		seen[id] = true
		result = append(result, id)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result, nil
}

func (style CompetitiveAIConfig) validateBubbleCostumeStyleLists() error {
	for _, list := range []struct {
		name   string
		values []uint16
	}{
		{"bubble_costume_color_ids", style.BubbleCostumeColorIDs},
		{"bubble_costume_effect_levels", style.BubbleCostumeEffectLevels},
	} {
		seen := map[uint16]bool{}
		for _, value := range list.values {
			if value > 5 {
				return fmt.Errorf("competitive_ai.%s value %d is outside 0..5", list.name, value)
			}
			if seen[value] {
				return fmt.Errorf("competitive_ai.%s repeats value %d", list.name, value)
			}
			seen[value] = true
		}
	}
	return nil
}

// assignCompetitiveAIBubbleCosmetics prepares temporary equipped ITEM_INFO
// entries for the native room projection. The original FA3 consumer resolves
// the bubble appearance from that actor's equipment, not its BombID field.
// This RNG is private to appearance selection and never advances room layout,
// policy sampling, or the combat engine's streams.
func assignCompetitiveAIBubbleCosmetics(projections []competitiveAIRoomProjection, costumes []uint16, forge *craftcatalog.Catalog, itemSeed uint32, style CompetitiveAIConfig) {
	for index := range projections {
		projections[index].BubbleCosmetic = game.ItemInfo{}
	}
	if len(projections) == 0 {
		return
	}
	pool := make([]uint16, 0, len(costumes))
	for _, id := range costumes {
		if id != 0 && forge != nil {
			if definition, listed := forge.Items[id]; !listed || definition.Class != craftcatalog.ForgeBomb {
				continue
			}
		}
		pool = append(pool, id)
	}
	if len(pool) == 0 {
		return
	}
	random := competitiveAIPRNG{state: uint64(itemSeed) ^ 0x42425542424C4501}
	for index := len(pool) - 1; index > 0; index-- {
		other := int(random.next() % uint64(index+1))
		pool[index], pool[other] = pool[other], pool[index]
	}
	var effects, colors []byte
	if forge != nil {
		for id, form := range forge.Effects {
			if id != 0 && form.Class == craftcatalog.ForgeBomb && form.Level >= 1 && form.Level <= 5 &&
				slices.Contains(style.BubbleCostumeEffectLevels, uint16(form.Level)) {
				effects = append(effects, id)
			}
		}
		if slices.Contains(style.BubbleCostumeEffectLevels, 0) {
			effects = append(effects, 0)
		}
		for color := range forge.Colors {
			if color <= craftcatalog.ForgeColorPurple && slices.Contains(style.BubbleCostumeColorIDs, uint16(color)) {
				colors = append(colors, byte(color))
			}
		}
	}
	sort.Slice(effects, func(i, j int) bool { return effects[i] < effects[j] })
	sort.Slice(colors, func(i, j int) bool { return colors[i] < colors[j] })
	for index := range projections {
		id := pool[index%len(pool)]
		if id == 0 {
			continue
		}
		item := game.NewPermanentItemInfo(id, 1)
		item.ItemStatus = game.ItemStatusActive
		item.ItemRoleID = projections[index].RoleID
		if len(colors) != 0 {
			item.ItemColor = colors[int(random.next()%uint64(len(colors)))]
		}
		// AI effects require a selected forge color. Keep the three allowlists
		// independent: an empty color list or a selected zero simply suppresses
		// the effect without rejecting a non-empty effect-level list.
		if item.ItemColor != 0 && len(effects) != 0 {
			item.ItemEffect = effects[int(random.next()%uint64(len(effects)))]
		}
		projections[index].BubbleCosmetic = item
	}
}
