package store

import (
	"sync"
	"time"
)

// aliasCacheTTL es la red de seguridad por si otro proceso escribe en la
// misma base sqlite: las escrituras de este proceso invalidan la caché al
// instante, las ajenas se ven como mucho tras este tiempo.
const aliasCacheTTL = 30 * time.Second

// aliasCache guarda en memoria la lista completa de entity_aliases. Es
// seguro para uso concurrente (sesiones HTTP simultáneas). El contador de
// generación evita que una lectura iniciada antes de una invalidación
// vuelva a poblar la caché con datos viejos.
type aliasCache struct {
	mu       sync.RWMutex
	list     []EntityAlias
	valid    bool
	loadedAt time.Time
	gen      uint64
	now      func() time.Time // inyectable en tests; nil = time.Now
}

func (c *aliasCache) clock() time.Time {
	if c.now != nil {
		return c.now()
	}
	return time.Now()
}

// get devuelve una copia de la lista si la caché es válida y no expiró.
func (c *aliasCache) get() ([]EntityAlias, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if !c.valid || c.clock().Sub(c.loadedAt) > aliasCacheTTL {
		return nil, false
	}
	return append([]EntityAlias(nil), c.list...), true
}

func (c *aliasCache) generation() uint64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.gen
}

// put guarda la lista solo si no hubo invalidaciones desde gen.
func (c *aliasCache) put(gen uint64, list []EntityAlias) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.gen != gen {
		return
	}
	c.list = append([]EntityAlias(nil), list...)
	c.valid = true
	c.loadedAt = c.clock()
}

// invalidate descarta el contenido; debe llamarse tras toda escritura de alias.
func (c *aliasCache) invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.gen++
	c.valid = false
	c.list = nil
}
