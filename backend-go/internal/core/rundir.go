package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"time"
)

// Estructura estándar de data/ (igual para los cuatro módulos):
//
//	data/<modulo>/runs/<YYYYMMDD_HHMMSS>/   una carpeta por corrida con todo lo que produjo (salidas y descargas crudas);
//	                                        run.json la marca como terminada (sin él, la corrida está incompleta)
//	data/<modulo>/cache/                    estado que se reutiliza entre corridas (cachés, series descargadas, universo)
//	data/_sistema/{jobs,logs,locks}/        registro de corridas, log único y candados
const (
	runsSubdir  = "runs"
	cacheSubdir = "cache"
	// RunMarker es el archivo que marca una corrida como terminada.
	RunMarker = "run.json"
	// RunIDLayout es el formato del identificador de corrida (hora de Settings.Loc).
	RunIDLayout = "20060102_150405"
)

var runIDRe = regexp.MustCompile(`^(\d{8}_\d{6})(?:_(\d+))?$`)

// RunsDir es data/<modulo>/runs.
func (s *Settings) RunsDir(m ModuleKey) string { return filepath.Join(s.ModuleDataDir(m), runsSubdir) }

// CacheDir es data/<modulo>/cache.
func (s *Settings) CacheDir(m ModuleKey) string {
	return filepath.Join(s.ModuleDataDir(m), cacheSubdir)
}

// SystemDir es data/_sistema.
func (s *Settings) SystemDir() string { return filepath.Join(s.DataDir, "_sistema") }

// JobsDir es data/_sistema/jobs (registro de corridas).
func (s *Settings) JobsDir() string { return filepath.Join(s.SystemDir(), "jobs") }

// LogsDir es data/_sistema/logs.
func (s *Settings) LogsDir() string { return filepath.Join(s.SystemDir(), "logs") }

// LocksDir es data/_sistema/locks.
func (s *Settings) LocksDir() string { return filepath.Join(s.SystemDir(), "locks") }

// RunMeta es el contenido de run.json.
type RunMeta struct {
	ID         string `json:"id"`
	Module     string `json:"module"`
	StartedAt  string `json:"started_at"`
	FinishedAt string `json:"finished_at,omitempty"`
}

// NewRun crea data/<modulo>/runs/<id> (id = hora actual; con sufijo _n si ya existe) y devuelve su ruta.
func NewRun(s *Settings, m ModuleKey, now time.Time) (string, error) {
	base := s.RunsDir(m)
	if err := os.MkdirAll(base, 0o755); err != nil {
		return "", err
	}
	id := now.In(s.Loc).Format(RunIDLayout)
	dir := filepath.Join(base, id)
	for n := 2; ; n++ {
		if err := os.Mkdir(dir, 0o755); err == nil {
			return dir, nil
		} else if !os.IsExist(err) {
			return "", err
		}
		dir = filepath.Join(base, fmt.Sprintf("%s_%d", id, n))
	}
}

// FinishRun marca la corrida como terminada (escribe run.json al final: es lo último que hace un pipeline).
func FinishRun(dir string, m ModuleKey, inicio, fin time.Time) error {
	meta := RunMeta{ID: filepath.Base(dir), Module: string(m),
		StartedAt: inicio.UTC().Format(time.RFC3339), FinishedAt: fin.UTC().Format(time.RFC3339)}
	b, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, RunMarker), b, 0o644)
}

// IsFinished dice si la carpeta de corrida tiene su run.json.
func IsFinished(dir string) bool {
	st, err := os.Stat(filepath.Join(dir, RunMarker))
	return err == nil && !st.IsDir()
}

// ListRuns lista las corridas terminadas del módulo, la más reciente primero.
func ListRuns(s *Settings, m ModuleKey) []string {
	ents, err := os.ReadDir(s.RunsDir(m))
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range ents {
		d := filepath.Join(s.RunsDir(m), e.Name())
		if e.IsDir() && runIDRe.MatchString(e.Name()) && IsFinished(d) {
			out = append(out, d)
		}
	}
	sort.Slice(out, func(i, j int) bool { return runOrder(filepath.Base(out[i])) > runOrder(filepath.Base(out[j])) })
	return out
}

// LatestRun es la corrida terminada más reciente ("" si no hay).
func LatestRun(s *Settings, m ModuleKey) string {
	if r := ListRuns(s, m); len(r) > 0 {
		return r[0]
	}
	return ""
}

// PreviousRun es la corrida terminada anterior a `dir` ("" si no hay).
func PreviousRun(s *Settings, m ModuleKey, dir string) string {
	runs := ListRuns(s, m)
	for i, r := range runs {
		if filepath.Base(r) == filepath.Base(dir) && i+1 < len(runs) {
			return runs[i+1]
		}
	}
	return ""
}

// RunTime es la hora codificada en el id de una carpeta de corrida (nil si no es válido).
func RunTime(dir string, loc *time.Location) *time.Time {
	m := runIDRe.FindStringSubmatch(filepath.Base(dir))
	if m == nil {
		return nil
	}
	t, err := time.ParseInLocation(RunIDLayout, m[1], loc)
	if err != nil {
		return nil
	}
	return &t
}

// runOrder ordena ids: primero la hora y, a igual hora, el sufijo _n numérico.
func runOrder(id string) string {
	m := runIDRe.FindStringSubmatch(id)
	if m == nil {
		return id
	}
	n := 1
	if m[2] != "" {
		fmt.Sscan(m[2], &n)
	}
	return fmt.Sprintf("%s_%06d", m[1], n)
}
