package core

import "testing"

func yahooSettings(t *testing.T, env map[string]string) (*Settings, error) {
	t.Helper()
	return loadSettings(func(k string) (string, bool) { v, ok := env[k]; return v, ok }, nil)
}

func TestYahooAdaptivoPorDefectoEncendido(t *testing.T) {
	s, err := yahooSettings(t, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !s.YahooAdaptativo || s.YahooIntervaloRapidoS != 0.2 {
		t.Errorf("adaptativo=%v rápido=%v, quería true y 0.2", s.YahooAdaptativo, s.YahooIntervaloRapidoS)
	}
}

func TestYahooAdaptivoSeConfiguraPorEntorno(t *testing.T) {
	s, err := yahooSettings(t, map[string]string{"YAHOO_ADAPTATIVO": "false", "YAHOO_INTERVALO_RAPIDO_S": "0.15"})
	if err != nil {
		t.Fatal(err)
	}
	if s.YahooAdaptativo || s.YahooIntervaloRapidoS != 0.15 {
		t.Errorf("adaptativo=%v rápido=%v", s.YahooAdaptativo, s.YahooIntervaloRapidoS)
	}
}

func TestYahooAdaptivoValoresInvalidos(t *testing.T) {
	for _, env := range []map[string]string{
		{"YAHOO_ADAPTATIVO": "quizás"},
		{"YAHOO_INTERVALO_RAPIDO_S": "0"},
		{"YAHOO_INTERVALO_RAPIDO_S": "rápido"},
	} {
		if _, err := yahooSettings(t, env); err == nil {
			t.Errorf("%v: debía fallar", env)
		}
	}
}
