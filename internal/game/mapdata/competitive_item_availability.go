package mapdata

import "qqtang/internal/clientdata/sceneelement"

// SuppressCompetitiveWallItem combines the map's configurable transformation
// policy and the existing ordinary-rule oxygen policy. Native catalogs and
// pickup handlers remain intact for recorded games.
func (entry CompetitiveMap) SuppressCompetitiveWallItem(sceneID uint32) bool {
	if entry.TransformationItemsDisabled && sceneelement.IsTransformationPickup(sceneelement.ID(sceneID)) {
		return true
	}
	return entry.NativeRule == 1 && (sceneID == 26 || sceneID == 62)
}
