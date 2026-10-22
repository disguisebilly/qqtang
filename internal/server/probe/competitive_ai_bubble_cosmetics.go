package probe

import (
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

// assignCompetitiveAIBubbleCosmetics prepares temporary equipped ITEM_INFO
// entries for the native room projection. The original FA3 consumer resolves
// the bubble appearance from that actor's equipment, not its BombID field.
// This RNG is private to appearance selection and never advances room layout,
// policy sampling, or the combat engine's streams.
func assignCompetitiveAIBubbleCosmetics(projections []competitiveAIRoomProjection, costumes []uint16, forge *craftcatalog.Catalog, itemSeed uint32) {
	if len(projections) == 0 {
		return
	}
	pool := make([]uint16, 0, len(costumes))
	for _, id := range costumes {
		if forge != nil {
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
			if id != 0 && form.Class == craftcatalog.ForgeBomb && form.Level >= 1 && form.Level <= 5 {
				effects = append(effects, id)
			}
		}
		for color := range forge.Colors {
			if color >= craftcatalog.ForgeColorBlack && color <= craftcatalog.ForgeColorPurple {
				colors = append(colors, byte(color))
			}
		}
	}
	sort.Slice(effects, func(i, j int) bool { return effects[i] < effects[j] })
	sort.Slice(colors, func(i, j int) bool { return colors[i] < colors[j] })
	for index := range projections {
		item := game.NewPermanentItemInfo(pool[index%len(pool)], 1)
		item.ItemStatus = game.ItemStatusActive
		item.ItemRoleID = projections[index].RoleID
		if len(effects) != 0 && len(colors) != 0 {
			item.ItemEffect = effects[int(random.next()%uint64(len(effects)))]
			item.ItemColor = colors[int(random.next()%uint64(len(colors)))]
		}
		projections[index].BubbleCosmetic = item
	}
}
