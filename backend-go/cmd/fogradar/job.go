package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"fogradar/internal/core"
	"fogradar/internal/jobs"
	"fogradar/internal/modules"
	"fogradar/internal/yahoo"
)

// Códigos de salida de `fogradar job`.
const (
	exitOK      = 0
	exitFalla   = 1
	exitUso     = 2
	exitOcupado = 3
)

// jobArgs es el resultado de parsear la línea de comandos de `fogradar job`.
type jobArgs struct {
	module    string
	todos     bool
	programa  string
	solo      string
	offline   bool
	noCache   bool
	trigger   string
	pausa     *float64
	reintento *int
	esperar   *float64
	list      bool
	crontab   bool
	backend   string
	marcadas  map[string]any // opciones propias de módulo, por Dest()
	flagOf    map[string]jobs.Option
}

func parseJobArgs(args []string) (*jobArgs, error) {
	a := &jobArgs{trigger: "cli", backend: "/opt/fog-radar/backend-go", marcadas: map[string]any{}, flagOf: map[string]jobs.Option{}}
	for _, m := range modules.All {
		if p, ok := jobs.Get(string(m.Key)); ok {
			for _, o := range p.Options {
				a.flagOf[o.Flag] = o
			}
		}
	}
	next := func(i *int, flag string) (string, error) {
		if *i+1 >= len(args) {
			return "", fmt.Errorf("%s requiere un valor", flag)
		}
		*i++
		return args[*i], nil
	}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		flag, val, hasVal := strings.Cut(arg, "=")
		get := func() (string, error) {
			if hasVal {
				return val, nil
			}
			return next(&i, flag)
		}
		switch flag {
		case "--todos":
			a.todos = true
		case "--offline":
			a.offline = true
		case "--no-cache":
			a.noCache = true
		case "--list":
			a.list = true
		case "--crontab":
			a.crontab = true
		case "--programa", "--solo", "--trigger", "--backend-dir":
			v, err := get()
			if err != nil {
				return nil, err
			}
			switch flag {
			case "--programa":
				a.programa = v
			case "--solo":
				a.solo = v
			case "--trigger":
				a.trigger = v
			default:
				a.backend = v
			}
		case "--pausa", "--esperar":
			v, err := get()
			if err != nil {
				return nil, err
			}
			f, err := strconv.ParseFloat(v, 64)
			if err != nil || f < 0 {
				return nil, fmt.Errorf("%s: '%s' no es un número válido", flag, v)
			}
			if flag == "--pausa" {
				a.pausa = &f
			} else {
				a.esperar = &f
			}
		case "--reintentos":
			v, err := get()
			if err != nil {
				return nil, err
			}
			n, err := strconv.Atoi(v)
			if err != nil || n < 0 {
				return nil, fmt.Errorf("--reintentos: '%s' no es un entero válido", v)
			}
			a.reintento = &n
		default:
			if strings.HasPrefix(flag, "--") {
				o, ok := a.flagOf[flag]
				if !ok {
					return nil, fmt.Errorf("opción desconocida: %s", flag)
				}
				if o.Kind == jobs.Flag {
					a.marcadas[o.Dest()] = true
					break
				}
				v, err := get()
				if err != nil {
					return nil, err
				}
				c, err := o.Convert(v)
				if err != nil {
					return nil, err
				}
				a.marcadas[o.Dest()] = c
				break
			}
			if a.module != "" {
				return nil, fmt.Errorf("argumento inesperado: %s", arg)
			}
			a.module = arg
		}
	}
	if a.trigger != "cli" && a.trigger != "cron" && a.trigger != "api" {
		return nil, fmt.Errorf("--trigger: '%s' inválido (cli | cron | api)", a.trigger)
	}
	if a.module != "" {
		if _, ok := modules.ByKey(a.module); !ok {
			return nil, fmt.Errorf("módulo desconocido: %s", a.module)
		}
	}
	if a.programa != "" {
		if _, ok := programaPorClave(a.programa); !ok {
			return nil, fmt.Errorf("--programa: '%s' desconocido", a.programa)
		}
	}
	if a.todos && a.programa != "" {
		return nil, errors.New("--todos y --programa son excluyentes")
	}
	if a.offline && a.noCache {
		return nil, errors.New("--offline y --no-cache son excluyentes: uno reutiliza lo descargado y el otro lo vuelve a pedir")
	}
	return a, nil
}

// aplicarSinCache agrega a cada módulo las opciones que le hacen ignorar sus cachés (--no-cache), sin pisar
// las que el usuario haya puesto a mano.
func aplicarSinCache(opciones map[string]map[string]any, mods []string) {
	for _, m := range mods {
		extra, ok := jobs.OpcionesSinCache[m]
		if !ok {
			continue
		}
		merged := map[string]any{}
		for k, v := range extra {
			merged[k] = v
		}
		for k, v := range opciones[m] {
			merged[k] = v
		}
		opciones[m] = merged
	}
}

func programaPorClave(k string) (core.Programa, bool) {
	for _, p := range core.Programas {
		if p.Key == k {
			return p, true
		}
	}
	return core.Programa{}, false
}

// runJob implementa `fogradar job …` y devuelve el código de salida.
func runJob(args []string, stdout, stderr io.Writer) int {
	a, err := parseJobArgs(args)
	if err != nil {
		fmt.Fprintln(stderr, "fogradar job:", err)
		return exitUso
	}
	if a.crontab {
		fmt.Fprint(stdout, Crontab(a.backend, "/var/log/fog-radar"))
		return exitOK
	}
	s, err := core.LoadSettings()
	if err != nil {
		fmt.Fprintln(stderr, "fogradar job:", err)
		return exitUso
	}
	if a.list || (a.module == "" && !a.todos && a.programa == "") {
		listar(s, stdout)
		return exitOK
	}

	// Qué módulos correr.
	var mods []string
	switch {
	case a.module != "":
		if a.todos || a.programa != "" || a.solo != "" {
			fmt.Fprintln(stderr, "Indica un módulo o --todos/--programa, no ambos.")
			return exitUso
		}
		mods = []string{a.module}
	case a.programa != "":
		p, _ := programaPorClave(a.programa)
		mods = append(mods, p.Modulos...)
	default:
		mods = append(mods, jobs.OrdenTodos...)
		if a.solo != "" {
			pedidos := map[string]bool{}
			var desconocidos []string
			for _, m := range strings.Split(a.solo, ",") {
				if m = strings.TrimSpace(m); m != "" {
					pedidos[m] = true
					if _, ok := modules.ByKey(m); !ok {
						desconocidos = append(desconocidos, m)
					}
				}
			}
			if len(desconocidos) > 0 {
				fmt.Fprintf(stderr, "Módulos desconocidos: %s\n", strings.Join(desconocidos, ", "))
				return exitUso
			}
			mods = mods[:0]
			for _, m := range jobs.OrdenTodos {
				if pedidos[m] {
					mods = append(mods, m)
				}
			}
		}
	}

	// Banderas propias de cada módulo: solo con un módulo.
	opciones := map[string]map[string]any{}
	if a.module != "" {
		p, _ := jobs.Get(a.module)
		permitidas := map[string]bool{}
		for _, o := range p.Options {
			permitidas[o.Dest()] = true
		}
		var invalidas []string
		for d := range a.marcadas {
			if !permitidas[d] {
				invalidas = append(invalidas, d)
			}
		}
		if len(invalidas) > 0 {
			sort.Strings(invalidas)
			fmt.Fprintf(stderr, "Opciones no válidas para '%s': %s\n", a.module, strings.Join(invalidas, ", "))
			return exitUso
		}
		opciones[a.module] = a.marcadas
	} else if len(a.marcadas) > 0 {
		fmt.Fprintln(stderr, "Las banderas de módulo (p. ej. --skip-fetch) van con un solo módulo; usa --offline.")
		return exitUso
	}
	if a.noCache {
		aplicarSinCache(opciones, mods)
	}
	omitir := map[string]string{}
	if a.offline {
		for _, m := range mods {
			if off, ok := jobs.OpcionesOffline[m]; ok {
				merged := map[string]any{}
				for k, v := range opciones[m] {
					merged[k] = v
				}
				for k, v := range off {
					merged[k] = v
				}
				opciones[m] = merged
			} else {
				omitir[m] = "sin modo offline (siempre descarga de Yahoo)"
			}
		}
	}

	log, closeLog := newLogger(s, stderr, "job")
	defer closeLog()
	log.Info("job: arranque", "args", strings.Join(args, " "), "trigger", a.trigger, "modulos", strings.Join(mods, ","),
		"offline", a.offline, "no_cache", a.noCache, "data_dir", s.DataDir, "yahoo_concurrencia", s.YahooConcurrencia)
	tr := yahoo.NewTraffic(yahoo.PolicyFromSettings(s))
	tr.SetLogger(func(f string, args ...any) { log.Warn(fmt.Sprintf(f, args...)) })
	env := &jobs.Env{S: s, Traffic: tr, Yahoo: yahoo.NewClient(tr), Log: log}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cron := a.trigger == "cron"
	so := jobs.SecuenciaOpts{Trigger: a.trigger, Opciones: opciones, Omitir: omitir}
	if a.pausa != nil {
		d := time.Duration(*a.pausa * float64(time.Second))
		so.Pausa = &d
	}
	switch {
	case a.reintento != nil:
		so.Reintentos = a.reintento
	case !cron:
		zero := 0
		so.Reintentos = &zero
	}
	switch {
	case a.esperar != nil:
		so.EsperarLock = time.Duration(*a.esperar * float64(time.Second))
	case cron:
		so.EsperarLock = time.Duration(s.PipelineLockEsperaS * float64(time.Second))
	}

	res, err := jobs.CorrerSecuencia(ctx, env, mods, so)
	if errors.Is(err, core.ErrOcupado) {
		fmt.Fprintln(stderr, err.Error())
		return exitOcupado
	}
	if err != nil {
		fmt.Fprintln(stderr, "fogradar job:", err)
		return exitFalla
	}
	fmt.Fprintln(stdout, "\nResumen de la corrida:\n"+jobs.Resumen(res))
	for _, r := range res {
		if r.Estado == jobs.EstadoError {
			return exitFalla
		}
	}
	return exitOK
}

func listar(s *core.Settings, w io.Writer) {
	for _, m := range modules.All {
		estado := fmt.Sprintf("pendiente (Fase %d)", m.MigrationPhase)
		if _, ok := jobs.Get(string(m.Key)); ok {
			estado = "listo"
		}
		fmt.Fprintf(w, "%-10s %-28s %s\n", m.Key, m.Title, estado)
	}
	ahora := time.Now().In(s.Loc)
	fmt.Fprintln(w, "\nProgramación (producción):")
	for _, p := range core.Programas {
		fmt.Fprintf(w, "  %-8s %-32s %-36s próxima: %s\n", p.Key, p.Descripcion(), strings.Join(p.Modulos, " → "),
			p.Proxima(ahora).Format("2006-01-02 15:04"))
	}
}

// Crontab es el contenido del crontab del servidor backend.
func Crontab(backendDir, logDir string) string {
	var b strings.Builder
	b.WriteString("# Fog Radar Quant — corridas batch (generado con: fogradar job --crontab)\n")
	b.WriteString("# Horas de America/Santiago. CRON_TZ lo respeta cronie; con cron de Debian/Ubuntu\n")
	b.WriteString("# la zona del servidor debe ser America/Santiago (timedatectl set-timezone America/Santiago).\n")
	b.WriteString("CRON_TZ=America/Santiago\nSHELL=/bin/bash\nPATH=/usr/local/bin:/usr/bin:/bin\n\n")
	for _, p := range core.Programas {
		fmt.Fprintf(&b, "# %s: %s (%s)\n", p.Titulo, p.Descripcion(), strings.Join(p.Modulos, " → "))
		fmt.Fprintf(&b, "%s cd %s && ./fogradar job --programa %s --trigger cron >> %s/pipelines-%s.log 2>&1\n",
			p.Cron(), backendDir, p.Key, logDir, p.Key)
	}
	return b.String()
}
