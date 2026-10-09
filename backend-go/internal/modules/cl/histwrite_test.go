package cl

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"fogradar/internal/core"
)

func settingsEn(t *testing.T, dir string) *core.Settings {
	t.Helper()
	loc, err := time.LoadLocation("America/Santiago")
	if err != nil {
		t.Fatal(err)
	}
	return &core.Settings{Env: "test", DataDir: dir, Loc: loc, Timezone: "America/Santiago"}
}

// volcado devuelve las tablas de la base como texto ordenado (sin stored_at, que es la hora de escritura).
func volcado(t *testing.T, path string) map[string][]string {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(path)+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	out := map[string][]string{}
	for tabla, q := range map[string]string{
		"runs":        "SELECT ts, run_at, archivo, n_tickers FROM runs",
		"ranking":     "SELECT r.ts, k.ticker, k.rank, k.nombre, k.sector, k.industria, k.score_compuesto, k.score_valor, k.score_calidad, k.score_crecimiento, k.score_salud_financiera, k.score_dividendo, k.score_momentum, k.metricas_disponibles, k.grupo_duplicado FROM ranking k JOIN runs r ON r.id = k.run_pk",
		"indicadores": "SELECT r.ts, i.ticker, i.metrica, i.valor FROM indicadores i JOIN runs r ON r.id = i.run_pk",
	} {
		rows, err := db.Query(q)
		if err != nil {
			t.Fatal(err)
		}
		cols, _ := rows.Columns()
		for rows.Next() {
			vals := make([]any, len(cols))
			ptrs := make([]any, len(cols))
			for i := range vals {
				ptrs[i] = &vals[i]
			}
			if err := rows.Scan(ptrs...); err != nil {
				t.Fatal(err)
			}
			parts := make([]string, len(vals))
			for i, v := range vals {
				if f, ok := v.(float64); ok {
					// pandas.read_csv (parser rápido) puede errar 1 ulp: se compara a 12 cifras significativas
					parts[i] = fmt.Sprintf("%.12g", f)
				} else {
					parts[i] = fmt.Sprintf("%v", v)
				}
			}
			out[tabla] = append(out[tabla], strings.Join(parts, "|"))
		}
		rows.Close()
		sort.Strings(out[tabla])
	}
	return out
}

// La history.db que escribe Go es idéntica a la que escribe Python con los mismos CSV.
func TestBackfillIgualAPython(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "cl_stocks"), 0o755); err != nil {
		t.Fatal(err)
	}
	ents, _ := os.ReadDir("testdata")
	for _, e := range ents {
		if strings.HasSuffix(e.Name(), ".csv") {
			b, _ := os.ReadFile(filepath.Join("testdata", e.Name()))
			if err := os.WriteFile(filepath.Join(dir, "cl_stocks", e.Name()), b, 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	s := settingsEn(t, dir)
	msg, err := Backfill(s, t.Logf)
	if err != nil || msg != "backfill: 2 corridas cargadas en history.db" {
		t.Fatal(msg, err)
	}
	got := volcado(t, filepath.Join(dir, "cl_stocks", core.HistDBName))
	want := volcado(t, "testdata/history_py.db")
	for _, tabla := range []string{"runs", "ranking", "indicadores"} {
		if len(got[tabla]) == 0 || len(got[tabla]) != len(want[tabla]) {
			t.Fatalf("%s: %d filas (go) vs %d (python)", tabla, len(got[tabla]), len(want[tabla]))
		}
		for i := range got[tabla] {
			if got[tabla][i] != want[tabla][i] {
				t.Fatalf("%s[%d]:\n go: %s\n py: %s", tabla, i, got[tabla][i], want[tabla][i])
			}
		}
	}
	// idempotente: una segunda pasada no duplica
	if _, err := Backfill(s, t.Logf); err != nil {
		t.Fatal(err)
	}
	if again := volcado(t, filepath.Join(dir, "cl_stocks", core.HistDBName)); len(again["ranking"]) != len(want["ranking"]) {
		t.Fatal("el backfill duplicó filas")
	}
	// y la capa de lectura (la que usa la API) lee lo que escribió Go
	rows, e := core.Filas(filepath.Join(dir, "cl_stocks", core.HistDBName), "SELECT ts FROM runs ORDER BY ts")
	if e != nil || len(rows) != 2 {
		t.Fatal(rows, e)
	}
}
