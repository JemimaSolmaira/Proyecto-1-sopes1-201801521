package main

import (
	"database/sql"
	"log"
	"os"
	"path/filepath"

	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os/exec"
	"os/signal"
	"sort"
	"syscall"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

var (
	defaultDBPath     = "./containers.db"
	defaultIntervalS  = 5
	defaultMaxProcess = 30
)

// Process representa un proceso dentro del snapshot
type Process struct {
	Pid      int    `json:"pid"`
	Comm     string `json:"comm"`
	RssKB    uint64 `json:"rss_kb"`
	VmsizeKB uint64 `json:"vmsize_kb"`
	State    string `json:"state"`
	Utime    uint64 `json:"utime"`
	Stime    uint64 `json:"stime"`
	TsMs     uint64 `json:"ts_ms"`
}

// SysInfo representa la cabecera global y la lista de procesos
type SysInfo struct {
	TotalRAMKB     uint64    `json:"total_ram_kb"`
	FreeRAMKB      uint64    `json:"free_ram_kb"`
	AvailableKB    uint64    `json:"available_kb"`
	RamUsedKB      uint64    `json:"ram_used_kb"`
	TotalProcs     int64     `json:"total_procs"`
	CPUUsagePct    uint64    `json:"cpu_usage_pct"`
	TsMs           uint64    `json:"ts_ms"`
	Procesos       []Process `json:"procesos"`
	RawJSONPresent bool      // no se mapea de JSON; se usa internamente si hace falta
}

// ReadSysinfo lee el archivo proc generado por el módulo y lo parsea en SysInfo.
// path: ruta al archivo, por ejemplo "/proc/sysinfo_so1_201801521".
func ReadSysinfo(path string) (SysInfo, error) {
	var si SysInfo

	f, err := os.Open(path)
	if err != nil {
		return si, fmt.Errorf("no se pudo abrir %s: %w", path, err)
	}
	defer f.Close()

	// Leer todo el contenido (asumimos tamaño razonable)
	data, err := io.ReadAll(f)
	if err != nil {
		return si, fmt.Errorf("error leyendo %s: %w", path, err)
	}

	// Decodificar JSON
	if err := json.Unmarshal(data, &si); err != nil {
		// Si falla, intentamos devolver algo más de contexto
		return si, fmt.Errorf("error parseando JSON de %s: %w\ncontenido recibido:\n%s", path, err, string(data))
	}

	si.RawJSONPresent = true
	return si, nil
}

// pretty print: info general + Top CPU + lista breve
func PrintSysInfo(si SysInfo) {
	ts := time.UnixMilli(int64(si.TsMs))
	fmt.Println("=== SysInfo Snapshot ===")
	fmt.Printf("Timestamp (ms): %d (%s)\n", si.TsMs, ts.UTC().Format(time.RFC3339Nano))
	fmt.Printf("Total RAM:     %d KB\n", si.TotalRAMKB)
	fmt.Printf("Free RAM:      %d KB\n", si.FreeRAMKB)
	fmt.Printf("Available RAM: %d KB\n", si.AvailableKB)
	fmt.Printf("RAM Used:      %d KB\n", si.RamUsedKB)
	fmt.Printf("CPU Usage %%:   %d\n", si.CPUUsagePct)
	fmt.Printf("Total procesos:%d\n", si.TotalProcs)
	fmt.Printf("Procesos reportados en lista: %d\n", len(si.Procesos))
	fmt.Println()

	if len(si.Procesos) == 0 {
		fmt.Println("No hay procesos en el snapshot.")
		return
	}

	// --- TOP N por CPU (utime + stime) ---
	// Primero verificamos si hay *algún* proceso con CPU > 0
	cpuDataAvailable := false
	for _, p := range si.Procesos {
		if p.Utime != 0 || p.Stime != 0 {
			cpuDataAvailable = true
			break
		}
	}

	if cpuDataAvailable {
		procsByCPU := make([]Process, len(si.Procesos))
		copy(procsByCPU, si.Procesos)

		sort.Slice(procsByCPU, func(i, j int) bool {
			ci := procsByCPU[i].Utime + procsByCPU[i].Stime
			cj := procsByCPU[j].Utime + procsByCPU[j].Stime
			return ci > cj // descendente
		})

		topN := 10
		if topN > len(procsByCPU) {
			topN = len(procsByCPU)
		}

		fmt.Printf("=== Top %d procesos por CPU (utime+stime) ===\n", topN)
		for i := 0; i < topN; i++ {
			p := procsByCPU[i]
			totalCPU := p.Utime + p.Stime
			fmt.Printf("%2d) PID:%5d  Name:%-20s  CPUtime:%12d  RSS:%8d KB  VMSZ:%8d KB  State:%1s\n",
				i+1, p.Pid, p.Comm, totalCPU, p.RssKB, p.VmsizeKB, p.State)
		}
		fmt.Println()
	} else {
		fmt.Println("⚠️  Todos los utime/stime vienen en 0; no se puede calcular Top CPU aún.")
		fmt.Println("   (Necesitas que el módulo del kernel llene utime/stime para cada proceso.)")
		fmt.Println()
	}

	// --- Lista breve de procesos (como referencia) ---
	fmt.Println("=== Lista de procesos (primeros 50 mostrados) ===")
	limit := len(si.Procesos)
	if limit > 50 {
		limit = 50
	}
	for i := 0; i < limit; i++ {
		p := si.Procesos[i]
		fmt.Printf("PID:%5d  Name:%-20s State:%1s  RSS:%8d KB  VMSZ:%8d KB  utime:%12d  stime:%12d  ts_ms:%d\n",
			p.Pid, p.Comm, p.State, p.RssKB, p.VmsizeKB, p.Utime, p.Stime, p.TsMs)
	}
	if len(si.Procesos) > 50 {
		fmt.Printf("... (se muestran 50 de %d procesos)\n", len(si.Procesos))
	}
}

func RunBashScript(path string) error {
	cmd := exec.Command("bash", path)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	fmt.Printf("Ejecutando script: %s\n", path)
	return cmd.Run()
}

func main() {

	// Ruta al archivo generado por el módulo del kernel
	defaultPath := "/proc/sysinfo_so1_201801521"

	path := flag.String("path", defaultPath, "Ruta al archivo del módulo")
	flag.Parse()

	// 🔁 Loop infinito que lee cada 20 segundos
	for {
		si, err := ReadSysinfo(*path)
		if err != nil {
			fmt.Println("Error leyendo sysinfo:", err)
		} else {
			PrintSysInfo(si)
		}

		time.Sleep(20 * time.Second)
	}
}

func pendiente() {

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = defaultDBPath
	}

	// crear carpeta si es necesario
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		log.Fatalf("no se pudo crear carpeta para db: %v", err)
	}

	db, err := sql.Open("sqlite3", dbPath+"?_foreign_keys=1")
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	if err := createSchema_system(db); err != nil {
		log.Fatalf("create schema: %v", err)
	}

	if err := createSchema_containers(db); err != nil {
		log.Fatalf("create schema containers: %v", err)
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	// Se define el ticker de 20 segundos
	ticker := time.NewTicker(20 * time.Second)
	// 'defer ticker.Stop()' asegura que el temporizador se detenga correctamente cuando el programa termine.
	defer ticker.Stop()

loop:
	for {
		select {
		case <-ticker.C:

		case <-sigs:
			// Este caso se ejecuta cuando se recibe una señal de interrupción o terminación.
			// Detiene el daemon y sale del bucle.
			log.Println("Daemon detenido.")
			break loop
		}
	}

}

func createSchema_system(db *sql.DB) error {
	queries := []string{
		// Tabla para métricas agregadas del sistema por tiempo
		`CREATE TABLE IF NOT EXISTS system_metrics (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			ts INTEGER NOT NULL, -- epoch seconds
			total_ram_mb INTEGER,
			free_ram_mb INTEGER,
			total_processes INTEGER,
			ram_used_mb INTEGER
		);`,
		// Tabla con procesos y su consumo en un momento dado
		`CREATE TABLE IF NOT EXISTS process_stats (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			ts INTEGER NOT NULL, -- epoch seconds
			pid INTEGER,
			name TEXT,
			cpu_percent REAL,
			ram_mb INTEGER
		);`,
		`CREATE INDEX IF NOT EXISTS idx_system_metrics_ts ON system_metrics(ts);`,
		`CREATE INDEX IF NOT EXISTS idx_process_stats_ts ON process_stats(ts);`,
	}
	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			return err
		}
	}
	return nil
}

func createSchema_containers(db *sql.DB) error {
	queries := []string{
		// Tabla para métricas agregadas del sistema por tiempo
		`CREATE TABLE IF NOT EXISTS containers_metrics (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			ts INTEGER NOT NULL, -- epoch seconds
			total_ram_mb INTEGER,
			free_ram_mb INTEGER,
			total_containers_deleted INTEGER,
			ram_used_mb INTEGER
		);`,
		// Tabla con procesos y su consumo en un momento dado
		`CREATE TABLE IF NOT EXISTS process_containers_stats (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			ts INTEGER NOT NULL, -- epoch seconds
			pid INTEGER,
			name TEXT,
			cpu_percent REAL,
			ram_mb INTEGER
		);`,
		`CREATE INDEX IF NOT EXISTS idx_system_metrics_ts ON system_metrics(ts);`,
		`CREATE INDEX IF NOT EXISTS idx_process_stats_ts ON process_stats(ts);`,
	}
	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			return err
		}
	}
	return nil
}
