package main

func (b *Broker) storeGrantLocked(g Grant) {
	if g.RegistryID != "" {
		for k, existing := range b.grants {
			if existing.Service == g.Service && existing.RegistryID == g.RegistryID {
				delete(b.grants, k)
			}
		}
		b.grants[grantKey("reg:"+g.RegistryID, g.Service)] = g
		return
	}
	b.grants[grantKey(g.EntityID, g.Service)] = g
}

func (b *Broker) purgeGrantLocked(entity, service, liveReg string) bool {
	removed := false
	g, ok := b.lookupGrantLocked(entity, service, liveReg)
	if ok {
		for k, existing := range b.grants {
			sameReg := g.RegistryID != "" && existing.RegistryID == g.RegistryID && existing.Service == service
			sameEntity := existing.EntityID == entity && existing.Service == service
			sameOld := existing.EntityID == g.EntityID && existing.Service == service
			if sameReg || sameEntity || sameOld {
				delete(b.grants, k)
				removed = true
			}
		}
	}
	if liveReg != "" {
		delete(b.grants, grantKey("reg:"+liveReg, service))
		for k, existing := range b.grants {
			if existing.Service == service && existing.RegistryID == liveReg {
				delete(b.grants, k)
				removed = true
			}
		}
		removed = true // live proven identity was resolved; treat matching purge as attempted
	}
	if _, ok := b.grants[grantKey(entity, service)]; ok {
		delete(b.grants, grantKey(entity, service))
		removed = true
	}
	return removed
}
