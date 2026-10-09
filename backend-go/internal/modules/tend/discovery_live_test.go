//go:build live

package tend

import (
	"context"
	"testing"
	"time"

	"fogradar/internal/core"
	"fogradar/internal/yahoo"
)

// go test -tags live -run TestDiscoveryLive -v -timeout 15m ./internal/modules/tend
// Consulta Yahoo de verdad (no escribe nada en disco).
func TestDiscoveryLive(t *testing.T) {
	st, err := core.LoadSettings()
	if err != nil {
		t.Fatal(err)
	}
	pol := yahoo.PolicyFromSettings(st)
	c := yahoo.NewClient(yahoo.NewTraffic(pol))
	log := func(f string, a ...any) { t.Logf(f, a...) }
	ctx, cancel := context.WithTimeout(context.Background(), 14*time.Minute)
	defer cancel()
	u, err := DiscoverUniverse(ctx, c, log, log)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Asia %d: %v", len(u.Asia), u.Asia)
	t.Logf("EEUU %d: %v", len(u.US), u.US)
}
