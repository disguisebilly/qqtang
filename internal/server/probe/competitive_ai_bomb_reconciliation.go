package probe

import "qqtang/internal/game/battleengine"

// reconcileNativeBombLedger removes transport identities for every object the
// native explosion retired, including bubbles cleared by its blast cells but
// absent from its explicit chain list. Surviving movement identities stay intact.
func (runtime *liveCompetitiveAIRuntime) reconcileNativeBombLedger(after *battleengine.Engine) {
	if after == nil {
		return
	}
	alive := make(map[uint32]struct{})
	for _, bomb := range after.Bombs() {
		alive[bomb.ID] = struct{}{}
	}
	for id := range runtime.bombs {
		if _, exists := alive[id]; !exists {
			delete(runtime.bombs, id)
		}
	}
}
