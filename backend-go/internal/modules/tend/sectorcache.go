package tend

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Caché persistente de sector y sede (assetProfile) por ticker. El descubrimiento ya no pregunta a Yahoo por
// empresas que conoce: el sector y el país de la sede casi nunca cambian. Es un índice derivado: si el archivo
// se borra o está dañado, se vuelve a consultar todo.

// SectorCacheFile es el archivo de la caché dentro de data/tendencia/.
const SectorCacheFile = "sector_cache.json"

// SectorCacheTTL es la vigencia de cada entrada; var para los tests.
var SectorCacheTTL = 60 * 24 * time.Hour

type sectorEntry struct {
	Sector  string    `json:"sector"`
	Country string    `json:"country,omitempty"` // sede; vacío = Yahoo no la informó
	At      time.Time `json:"at"`
}

// SectorCache guarda sector y sede entre corridas. Un puntero nil es una caché vacía que no guarda nada.
type SectorCache struct {
	path  string
	now   func() time.Time
	mu    sync.Mutex
	m     map[string]sectorEntry
	dirty bool
	// Refresh ignora lo guardado al leer (se consulta todo de nuevo) pero sigue guardando lo consultado.
	Refresh bool
}

// LoadSectorCache lee la caché de path (vacía si no existe o está dañada). now fija el reloj (por defecto time.Now).
func LoadSectorCache(path string, now func() time.Time) *SectorCache {
	if now == nil {
		now = time.Now
	}
	c := &SectorCache{path: path, now: now, m: map[string]sectorEntry{}}
	if b, err := os.ReadFile(path); err == nil {
		var m map[string]sectorEntry
		if json.Unmarshal(b, &m) == nil && m != nil {
			c.m = m
		}
	}
	return c
}

// get devuelve la entrada vigente de un ticker.
func (c *SectorCache) get(symbol string) (sectorEntry, bool) {
	if c == nil || c.Refresh {
		return sectorEntry{}, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.m[symbol]
	if !ok || e.Sector == "" || c.now().Sub(e.At) > SectorCacheTTL {
		return sectorEntry{}, false
	}
	return e, true
}

// put guarda el sector y la sede de un ticker.
func (c *SectorCache) put(symbol, sector, country string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.m[symbol] = sectorEntry{Sector: sector, Country: country, At: c.now()}
	c.dirty = true
	c.mu.Unlock()
}

// Save escribe la caché (de forma atómica) si cambió, sin las entradas vencidas.
func (c *SectorCache) Save() error {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.dirty {
		return nil
	}
	now := c.now()
	for k, e := range c.m {
		if now.Sub(e.At) > SectorCacheTTL {
			delete(c.m, k)
		}
	}
	b, err := json.MarshalIndent(c.m, "", "  ") // las claves salen ordenadas: el archivo es estable
	if err != nil {
		return err
	}
	if c.path == "" {
		return errors.New("la caché de sectores no tiene ruta")
	}
	if err := os.MkdirAll(filepath.Dir(c.path), 0o755); err != nil {
		return err
	}
	tmp := c.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, c.path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	c.dirty = false
	return nil
}
