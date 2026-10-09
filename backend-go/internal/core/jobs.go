package core

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gofrs/flock"
	"github.com/google/uuid"
)

// StaleAfter: una corrida "running" más vieja que esto se considera colgada.
const StaleAfter = 6 * time.Hour

// JobRecord es una línea de data/_jobs/<modulo>.jsonl.
type JobRecord struct {
	ID         string   `json:"id"`
	Module     string   `json:"module"`
	Status     string   `json:"status" enum:"running,success,error,stale"`
	Trigger    string   `json:"trigger" enum:"cli,cron,api" default:"cli" required:"false"`
	StartedAt  Fecha    `json:"started_at"`
	FinishedAt *Fecha   `json:"finished_at" nullable:"true" required:"false"`
	DurationS  *float64 `json:"duration_s" nullable:"true" required:"false"`
	Message    *string  `json:"message" nullable:"true" required:"false"`
}

// JobRegistry lee el registro de jobs. En la Fase 1 es solo lectura: Python
// sigue escribiéndolo (los pipelines aún no se migran).
type JobRegistry struct {
	Dir string
	Now func() time.Time
}

// NewJobRegistry crea un lector sobre data/_jobs.
func NewJobRegistry(dir string) *JobRegistry { return &JobRegistry{Dir: dir, Now: time.Now} }

func (r *JobRegistry) read(module string) []JobRecord {
	f, err := os.Open(r.path(module))
	if err != nil {
		return nil
	}
	defer f.Close()
	var out []JobRecord
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var rec JobRecord
		if err := json.Unmarshal(line, &rec); err != nil {
			continue // línea inválida: se ignora, como en Python
		}
		if rec.Trigger == "" {
			rec.Trigger = "cli"
		}
		out = append(out, rec)
	}
	return out
}

// History devuelve las ejecuciones del módulo (la más reciente primero), una por id;
// la última línea de cada id gana.
func (r *JobRegistry) History(module string, limit int) []JobRecord {
	byID := map[string]JobRecord{}
	for _, rec := range r.read(module) {
		byID[rec.ID] = rec
	}
	list := make([]JobRecord, 0, len(byID))
	for _, rec := range byID {
		list = append(list, rec)
	}
	sort.SliceStable(list, func(i, j int) bool { return list[i].StartedAt.After(list[j].StartedAt.Time) })
	if limit < len(list) {
		list = list[:limit]
	}
	now := r.Now()
	for i := range list {
		if list[i].Status == "running" && now.Sub(list[i].StartedAt.Time) > StaleAfter {
			list[i].Status = "stale"
		}
	}
	return list
}

// Latest es la ejecución más reciente, o nil.
func (r *JobRegistry) Latest(module string) *JobRecord {
	if h := r.History(module, 1); len(h) > 0 {
		return &h[0]
	}
	return nil
}

// LastSuccess es la última ejecución exitosa (entre las 200 más recientes), o nil.
func (r *JobRegistry) LastSuccess(module string) *JobRecord {
	for _, rec := range r.History(module, 200) {
		if rec.Status == "success" {
			return &rec
		}
	}
	return nil
}

// MaxBytes / MaxLineas: rotación del registro (al pasar de MaxBytes se conservan las últimas MaxLineas).
const (
	MaxBytes  = 512 * 1024
	MaxLineas = 1000
)

func (r *JobRegistry) path(module string) string { return filepath.Join(r.Dir, module+".jsonl") }

// Append agrega una línea al registro del módulo bajo el lock de archivo (el mismo <archivo>.lock que usa
// Python, así ambos backends pueden escribir en la misma carpeta de datos).
func (r *JobRegistry) Append(rec JobRecord) error {
	if err := os.MkdirAll(r.Dir, 0o755); err != nil {
		return err
	}
	p := r.path(rec.Module)
	lock := flock.New(p + ".lock")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ok, err := lock.TryLockContext(ctx, 50*time.Millisecond)
	if err != nil || !ok {
		return fmt.Errorf("no se pudo tomar el lock de %s: %w", filepath.Base(p), err)
	}
	defer lock.Unlock() //nolint:errcheck

	line, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if st, err := os.Stat(p); err == nil && st.Size() > MaxBytes {
		return compactar(p)
	}
	return nil
}

// compactar deja las últimas MaxLineas líneas (el registro se lee entero en cada request).
func compactar(p string) error {
	b, err := os.ReadFile(p)
	if err != nil {
		return err
	}
	lines := strings.SplitAfter(string(b), "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) > MaxLineas {
		lines = lines[len(lines)-MaxLineas:]
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, []byte(strings.Join(lines, "")), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

// IsRunning dice si la última ejecución del módulo sigue «running».
func (r *JobRegistry) IsRunning(module string) bool {
	rec := r.Latest(module)
	return rec != nil && rec.Status == "running"
}

// Track envuelve una ejecución de pipeline y registra su resultado: una línea «running» al empezar y
// otra «success»/«error» al terminar, con el mismo id. fn puede fijar rec.Message.
func (r *JobRegistry) Track(module, trigger string, fn func(rec *JobRecord) error) (*JobRecord, error) {
	rec := &JobRecord{ID: strings.ReplaceAll(uuid.NewString(), "-", "")[:12], Module: module, Status: "running",
		Trigger: trigger, StartedAt: Fecha{Time: time.Now().UTC()}}
	if err := r.Append(*rec); err != nil {
		return rec, err
	}
	t0 := time.Now()
	err := fn(rec)
	if err != nil {
		rec.Status = "error"
		if rec.Message == nil {
			m := err.Error()
			rec.Message = &m
		}
	} else {
		rec.Status = "success"
	}
	fin := Fecha{Time: time.Now().UTC()}
	rec.FinishedAt = &fin
	d := math.Round(time.Since(t0).Seconds()*100) / 100
	rec.DurationS = &d
	if aerr := r.Append(*rec); aerr != nil && err == nil {
		err = aerr
	}
	return rec, err
}

// ErrOcupado: otra secuencia de pipelines tiene el lock global.
var ErrOcupado = errors.New("Ya hay una corrida de pipelines en curso (lock data/_jobs/.pipelines.lock).")

// LockPipelines toma el lock global entre procesos (data/_jobs/.pipelines.lock), esperando hasta wait.
// Se libera solo si el proceso muere. Devuelve el unlock.
func LockPipelines(s *Settings, wait time.Duration) (func(), error) {
	if err := os.MkdirAll(s.JobsDir(), 0o755); err != nil {
		return nil, err
	}
	lock := flock.New(filepath.Join(s.JobsDir(), ".pipelines.lock"))
	ok, err := lock.TryLock() // un intento inmediato aunque wait sea 0
	if err != nil {
		return nil, err
	}
	if !ok && wait > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), wait)
		defer cancel()
		ok, err = lock.TryLockContext(ctx, 250*time.Millisecond)
		if err != nil && !errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
	}
	if !ok {
		return nil, ErrOcupado
	}
	return func() { _ = lock.Unlock() }, nil
}
