package transversal

import (
	"testing"
	"time"

	"fogradar/internal/core"
	"fogradar/internal/modules"
)

type provFalso struct{ entradas []EntradaBusqueda }

func (p provFalso) ResumenHub(*core.Settings) (*ResumenModulo, error) { return nil, nil }
func (p provFalso) IndiceBusqueda(*core.Settings) ([]EntradaBusqueda, error) {
	return p.entradas, nil
}

func TestIndiceCompletoReuneLosModulos(t *testing.T) {
	LimpiarCache()
	t.Cleanup(LimpiarCache)
	primero, segundo := string(modules.All[0].Key), string(modules.All[1].Key)
	prev1, prev2 := provider(primero), provider(segundo)
	RegisterProvider(primero, provFalso{[]EntradaBusqueda{{Ticker: "AAA"}, {Ticker: "BBB"}}})
	RegisterProvider(segundo, provFalso{[]EntradaBusqueda{{Ticker: "CCC"}}})
	t.Cleanup(func() {
		if prev1 != nil {
			RegisterProvider(primero, prev1)
		}
		if prev2 != nil {
			RegisterProvider(segundo, prev2)
		}
	})

	s := &core.Settings{DataDir: t.TempDir(), Loc: time.UTC}
	out := IndiceCompleto(s, time.Now())
	var propios []EntradaIndice
	for _, e := range out.Entradas {
		if e.Modulo == primero || e.Modulo == segundo {
			propios = append(propios, e)
		}
	}
	if len(propios) != 3 {
		t.Fatalf("esperaba 3 entradas, hay %d", len(propios))
	}
	if propios[0].Orden != 0 || propios[2].Orden != 1 || propios[2].TituloModulo != modules.All[1].Title {
		t.Errorf("orden/título: %+v", propios)
	}
}
