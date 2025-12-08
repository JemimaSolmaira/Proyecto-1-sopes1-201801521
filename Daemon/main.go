package main

import (
	"bytes"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

const (
	sysinfoPath      = "/proc/sysinfo_so1_201801521"
	continfoPath     = "/proc/continfo_so1_201801521"
	dbPath           = "monitoring.db"
	defaultInterval1 = 20 * time.Second
)

var stressCmd *exec.Cmd

// helper: total contenedores eliminados (para container_host_metrics)
func GetTotalDeletedContainers(db *sql.DB) (int, error) {
	row := db.QueryRow(`SELECT COUNT(*) FROM containers WHERE removed_at_ts_ms IS NOT NULL;`)
	var count int
	if err := row.Scan(&count); err != nil {
		return 0, fmt.Errorf("error consultando total contenedores eliminados: %w", err)
	}
	return count, nil
}

// Inicia el stack de Grafana desde docker-compose
func StartGrafanaContainers(composeDir string) error {
	fmt.Println("🚀 Iniciando contenedor de Grafana...")

	cmd := exec.Command("docker", "compose", "up", "-d")
	cmd.Dir = composeDir // carpeta donde está tu docker-compose.yml

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error iniciando Grafana: %w\nSalida:\n%s", err, string(output))
	}

	fmt.Println("✅ Grafana iniciado correctamente.")
	fmt.Println(string(output))
	return nil
}

func IsGrafanaRunning() bool {
	cmd := exec.Command("docker", "ps", "--filter", "name=grafana-sqlite", "--filter", "status=running", "--format", "{{.Names}}")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return len(out) > 0
}

func main() {

	// ====== MANEJO DE CTRL+C / SIGTERM ======
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\n🛑 Señal de parada recibida (Ctrl+C).")

		// ✅ 1. Detener bash de stress
		if stressCmd != nil && stressCmd.Process != nil {
			fmt.Println("⛔ Deteniendo script stress_container.sh...")
			if err := stressCmd.Process.Kill(); err != nil {
				fmt.Println("❌ Error al detener stress bash:", err)
			} else {
				fmt.Println("✅ Stress bash detenido.")
			}
		}

		// ✅ 2. Ejecutar script detener.sh
		fmt.Println("🧹 Ejecutando detener.sh...")
		if err := RunDetenerScript(); err != nil {
			fmt.Println("❌ Error al ejecutar detener.sh:", err)
		}

		fmt.Println("👋 Saliendo del daemon.")
		os.Exit(0)
	}()

	//EJECUCION DE GRAFANA
	composeDir := "/home/jemima/Documentos/proyecto-1/grafana" // ajusta si cambia la ruta

	if !IsGrafanaRunning() {
		if err := StartGrafanaContainers(composeDir); err != nil {
			fmt.Println("No se pudo iniciar Grafana:", err)
		} else {
			fmt.Println("Grafana disponible en: http://localhost:3000")
		}
	} else {
		fmt.Println("Grafana ya estaba corriendo.")
	}

	//EJECUCION DEL SCRIPT PARA CARGAR MODULOS DE KERNEL
	if err := RunInstallModules(); err != nil {
		fmt.Println(" No se pudieron instalar los módulos:", err)
		// decide si quieres terminar aquí o seguir
		// return
	}
	//CRONJOB

	if err := RunStressContainerScript(); err != nil {
		fmt.Println("No se pudo ejecutar stress_container.sh:", err)
		// decide si sigues o no
	}

	//LOOP PRINCIPAL

	fmt.Println("  Monitor + Orquestador de contenedores iniciado")
	fmt.Printf("   Leyendo sysinfo:  %s\n", sysinfoPath)
	fmt.Printf("   Leyendo continfo: %s\n", continfoPath)
	fmt.Printf("   Intervalo:        %s\n", defaultInterval1)
	fmt.Println("   Ctrl+C para detener.")
	fmt.Println()

	// 0) Abrir DB y crear tablas
	db, err := OpenDB(dbPath)
	if err != nil {
		fmt.Println(" Error abriendo DB:", err)
		return
	}
	defer db.Close()

	if err := CreateSystemMetricsTable(db); err != nil {
		fmt.Println(" Error creando system_metrics:", err)
		return
	}
	if err := CreateProcessMetricsTable(db); err != nil {
		fmt.Println("Error creando process_metrics:", err)
		return
	}
	if err := CreateProcessStateSummaryTable(db); err != nil {
		fmt.Println("Error creando process_state_summary:", err)
		return
	}
	if err := CreateContainerHostMetricsTable(db); err != nil {
		fmt.Println("Error creando container_host_metrics:", err)
		return
	}
	if err := CreateContainersTable(db); err != nil {
		fmt.Println(" Error creando containers:", err)
		return
	}
	if err := CreateContainerMetricsTable(db); err != nil {
		fmt.Println("Error creando container_metrics:", err)
		return
	}

	// Para calcular %CPU necesitamos snapshot previo
	var (
		prevSys      SysInfo
		havePrevSys  bool
		prevCont     ContInfoSnapshot
		havePrevCont bool
	)

	numCPUs := runtime.NumCPU()

	for {
		fmt.Println("\n\n========== CICLO DE MONITOREO ==========")
		now := time.Now()
		fmt.Printf("%s\n", now.Format(time.RFC3339))

		// ===== 1) SYSINFO: procesos del sistema =====
		si, err := ReadSysinfo(sysinfoPath)
		if err != nil {
			fmt.Println(" Error leyendo sysinfo:", err)
		} else {
			// 1.a) Mostrar en consola
			PrintSysInfo(si)

			// 1.b) Insertar métricas globales
			if _, err := InsertSystemMetrics(db, si); err != nil {
				fmt.Println(" Error InsertSystemMetrics:", err)
			}

			// 1.c) Calcular %CPU por proceso (si tenemos snapshot previo)
			var cpuPctProc map[int]float64
			if havePrevSys {
				cpuPctProc = BuildProcCpuPct(prevSys, si, numCPUs, 100.0) // HZ=100 (ajusta si tu kernel usa otro)
			}

			// 1.d) Insertar procesos
			if err := InsertProcessMetricsBulk(db, si, cpuPctProc); err != nil {
				fmt.Println("Error InsertProcessMetricsBulk:", err)
			}

			// 1.e) Resumen de estados
			if err := InsertProcessStateSummary(db, si); err != nil {
				fmt.Println("Error InsertProcessStateSummary:", err)
			}

			// Actualizar snapshot previo
			prevSys = si
			havePrevSys = true
		}

		// ===== 2) CONINFO: contenedores =====
		snap, err := ReadContInfo(continfoPath)
		if err != nil {
			fmt.Println("Error leyendo continfo:", err)
		} else {
			// 2.a) Mostrar contenedores detectados
			//PrintContainers(snap)

			// 2.b) Ciclo de vida de contenedores (containers)
			if err := UpsertContainersFromSnapshot(db, snap); err != nil {
				fmt.Println(" Error UpsertContainersFromSnapshot:", err)
			}

			// 2.c) Total contenedores eliminados (acumulado)
			totalDeletedAcc, err := GetTotalDeletedContainers(db)
			if err != nil {
				fmt.Println("Error GetTotalDeletedContainers:", err)
				totalDeletedAcc = 0
			}

			// 2.d) Métricas a nivel host de contenedores
			if _, err := InsertContainerHostMetrics(db, snap, totalDeletedAcc); err != nil {
				fmt.Println(" Error InsertContainerHostMetrics:", err)
			}

			// 2.e) Calcular %CPU por contenedor (si tenemos snapshot previo)
			var cpuPctCont map[string]float64
			if havePrevCont {
				cpuPctCont = BuildContainerCpuPct(prevCont, snap, numCPUs)
			}

			// 2.f) Métricas por contenedor
			if err := InsertContainerMetricsBulk(db, snap, cpuPctCont); err != nil {
				fmt.Println(" Error InsertContainerMetricsBulk:", err)
			}

			// Actualizar snapshot previo
			prevCont = snap
			havePrevCont = true
		}

		// ===== 3) Aplicar reglas de eliminación sobre contenedores stress-* =====
		enforceRules()

		// ===== 4) Esperar siguiente ciclo =====
		time.Sleep(defaultInterval1)
	}
}

// OpenDB abre (o crea) el archivo SQLite
func OpenDB(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("no se pudo abrir la DB %s: %w", dbPath, err)
	}

	// Verificamos conexión
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("error haciendo ping a la DB: %w", err)
	}

	return db, nil
}

func RunInstallModules() error {
	scriptPath := "../bash/install_modules.sh" // relativo a la carpeta Daemon

	cmd := exec.Command("bash", scriptPath)
	// Si quieres poner timeout:
	// ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	// defer cancel()
	// cmd := exec.CommandContext(ctx, "bash", scriptPath)

	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	fmt.Println(" Ejecutando script de instalación de módulos:", scriptPath)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("error ejecutando %s: %w\nstderr:\n%s", scriptPath, err, stderr.String())
	}

	fmt.Println("Script de instalación finalizado.")
	if out.Len() > 0 {
		fmt.Println("Salida:")
		fmt.Println(out.String())
	}
	return nil
}

// Variable global (debe estar FUERA de cualquier función, al inicio del archivo):
// var stressCmd *exec.Cmd

func RunStressContainerScript() error {
	scriptPath := "../cronjob/stress_container.sh" // ruta relativa a /Daemon

	// ✅ Usamos la variable global, NO una local
	stressCmd = exec.Command("bash", scriptPath)

	var out bytes.Buffer
	var stderr bytes.Buffer
	stressCmd.Stdout = &out
	stressCmd.Stderr = &stderr

	fmt.Println("Ejecutando script de estrés de contenedores:", scriptPath)

	// ✅ Start() para que NO bloquee el main
	if err := stressCmd.Start(); err != nil {
		// si falla al arrancar, limpiamos la global
		stressCmd = nil
		return fmt.Errorf("error iniciando %s: %w\nstderr:\n%s", scriptPath, err, stderr.String())
	}

	// ✅ Esperamos en segundo plano, solo para loguear cuando termine
	go func() {
		if err := stressCmd.Wait(); err != nil {
			fmt.Println("⚠️ stress_container.sh terminó con error:", err)
		} else {
			fmt.Println("✅ Script stress_container.sh finalizado.")
		}

		if out.Len() > 0 {
			fmt.Println("Salida script stress_container.sh:")
			fmt.Println(out.String())
		}

		// cuando termina, limpiamos referencia
		stressCmd = nil
	}()

	return nil
}

// Ejecuta el bash que detiene los contenedores de estrés
func RunDetenerScript() error {
	scriptPath := "../cronjob/detener.sh"

	cmd := exec.Command("bash", scriptPath)

	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	fmt.Println("🧹 Ejecutando script de limpieza de contenedores:", scriptPath)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("error ejecutando %s: %w\nstderr:\n%s", scriptPath, err, stderr.String())
	}

	fmt.Println("Script detener.sh finalizado.")
	if out.Len() > 0 {
		fmt.Println("Salida:")
		fmt.Println(out.String())
	}
	return nil
}
