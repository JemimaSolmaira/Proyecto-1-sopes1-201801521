package main

import (
	"database/sql"
	"fmt"
	"strings"
)

func CreateContainerHostMetricsTable(db *sql.DB) error {
	ddl := `
    CREATE TABLE IF NOT EXISTS container_host_metrics (
        id                INTEGER PRIMARY KEY AUTOINCREMENT,
        ts_ms             BIGINT NOT NULL,
        total_ram_kb      BIGINT NOT NULL,
        free_ram_kb       BIGINT NOT NULL,
        used_ram_kb       BIGINT NOT NULL,
        total_containers  INT NOT NULL,
        total_deleted_acc INT NOT NULL,
        created_at        TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    );
    `
	if _, err := db.Exec(ddl); err != nil {
		return fmt.Errorf("error creando tabla container_host_metrics: %w", err)
	}

	idx := `CREATE INDEX IF NOT EXISTS idx_chost_ts ON container_host_metrics(ts_ms);`
	if _, err := db.Exec(idx); err != nil {
		return fmt.Errorf("error creando índice idx_chost_ts: %w", err)
	}

	return nil
}

// insert un snapshot de métricas de host de contenedores.
func InsertContainerHostMetrics(db *sql.DB, snap ContInfoSnapshot, totalDeletedAcc int) (int64, error) {
	contIDs := make(map[string]struct{})
	for _, p := range snap.Procesos {
		if p.ContainerRelated != "yes" {
			continue
		}
		if p.CmdlineOrContID == "" {
			continue
		}
		contIDs[p.CmdlineOrContID] = struct{}{}
	}
	totalContainers := len(contIDs)

	query := `
        INSERT INTO container_host_metrics (
            ts_ms,
            total_ram_kb,
            free_ram_kb,
            used_ram_kb,
            total_containers,
            total_deleted_acc
        ) VALUES (?, ?, ?, ?, ?, ?);
    `
	res, err := db.Exec(
		query,
		snap.TsMs,
		int64(snap.TotalRAMKB),
		int64(snap.FreeRAMKB),
		int64(snap.UsedRAMKB),
		totalContainers,
		totalDeletedAcc,
	)
	if err != nil {
		return 0, fmt.Errorf("error insertando en container_host_metrics: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("no se pudo obtener LastInsertId en container_host_metrics: %w", err)
	}
	return id, nil
}

func CreateContainersTable(db *sql.DB) error {
	ddl := `
    CREATE TABLE IF NOT EXISTS containers (
        id               INTEGER PRIMARY KEY AUTOINCREMENT,
        container_id     VARCHAR(128) NOT NULL,
        first_seen_ts_ms BIGINT NOT NULL,
        last_seen_ts_ms  BIGINT NOT NULL,
        removed_at_ts_ms BIGINT,
        container_type   VARCHAR(32),
        created_at       TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    );
    `
	if _, err := db.Exec(ddl); err != nil {
		return fmt.Errorf("error creando tabla containers: %w", err)
	}

	idx := `CREATE UNIQUE INDEX IF NOT EXISTS idx_containers_cid ON containers(container_id);`
	if _, err := db.Exec(idx); err != nil {
		return fmt.Errorf("error creando índice idx_containers_cid: %w", err)
	}

	return nil
}

func classifyContainerType(containerID string) string {
	// Ajusta a tus nombres reales de contenedores
	if containsIgnoreCase(containerID, "high-cpu") {
		return "HIGH_CPU"
	}
	if containsIgnoreCase(containerID, "high-ram") {
		return "HIGH_RAM"
	}
	if containsIgnoreCase(containerID, "low") {
		return "LOW"
	}
	return "UNKNOWN"
}

func containsIgnoreCase(s, sub string) bool {
	sLower := strings.ToLower(s)
	subLower := strings.ToLower(sub)
	return strings.Contains(sLower, subLower)
}

// UpsertContainersFromSnapshot actualiza la tabla containers según el snapshot actual.
func UpsertContainersFromSnapshot(db *sql.DB, snap ContInfoSnapshot) error {
	// 1) Conjunto de contenedores vivos en este snapshot
	current := make(map[string]struct{})
	for _, p := range snap.Procesos {
		if p.ContainerRelated != "yes" {
			continue
		}
		cid := p.CmdlineOrContID
		if cid == "" {
			continue
		}
		current[cid] = struct{}{}
	}

	if len(current) == 0 {
		return nil
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("error iniciando transacción en UpsertContainersFromSnapshot: %w", err)
	}

	// 2) Upsert para cada contenedor vivo
	for cid := range current {
		res, err := tx.Exec(`
            UPDATE containers
            SET last_seen_ts_ms = ?
            WHERE container_id = ?;
        `, snap.TsMs, cid)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("error haciendo UPDATE en containers para %s: %w", cid, err)
		}

		rows, err := res.RowsAffected()
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("error leyendo RowsAffected en containers: %w", err)
		}

		if rows == 0 {
			ctype := classifyContainerType(cid)
			if _, err := tx.Exec(`
                INSERT INTO containers (
                    container_id,
                    first_seen_ts_ms,
                    last_seen_ts_ms,
                    container_type
                ) VALUES (?, ?, ?, ?);
            `, cid, snap.TsMs, snap.TsMs, ctype); err != nil {
				tx.Rollback()
				return fmt.Errorf("error haciendo INSERT en containers para %s: %w", cid, err)
			}
		}
	}

	// 3) Marcar como removidos los que ya no aparecen en este snapshot
	rows, err := tx.Query(`
        SELECT container_id
        FROM containers
        WHERE removed_at_ts_ms IS NULL;
    `)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("error consultando containers abiertos: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var cid string
		if err := rows.Scan(&cid); err != nil {
			tx.Rollback()
			return fmt.Errorf("error leyendo container_id: %w", err)
		}
		if _, ok := current[cid]; !ok {
			if _, err := tx.Exec(`
                UPDATE containers
                SET removed_at_ts_ms = ?
                WHERE container_id = ? AND removed_at_ts_ms IS NULL;
            `, snap.TsMs, cid); err != nil {
				tx.Rollback()
				return fmt.Errorf("error marcando contenedor %s como removido: %w", cid, err)
			}
		}
	}

	if err := rows.Err(); err != nil {
		tx.Rollback()
		return fmt.Errorf("error iterando containers abiertos: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("error haciendo commit en UpsertContainersFromSnapshot: %w", err)
	}

	return nil
}

func CreateContainerMetricsTable(db *sql.DB) error {
	ddl := `
    CREATE TABLE IF NOT EXISTS container_metrics (
        id            INTEGER PRIMARY KEY AUTOINCREMENT,
        ts_ms         BIGINT NOT NULL,
        container_id  VARCHAR(128) NOT NULL,
        rss_kb        BIGINT NOT NULL,
        cpu_time_ns   BIGINT NOT NULL,
        cpu_pct       REAL NOT NULL,
        created_at    TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    );
    `
	if _, err := db.Exec(ddl); err != nil {
		return fmt.Errorf("error creando tabla container_metrics: %w", err)
	}

	idx1 := `CREATE INDEX IF NOT EXISTS idx_cmetrics_ts ON container_metrics(ts_ms);`
	idx2 := `CREATE INDEX IF NOT EXISTS idx_cmetrics_ts_rss ON container_metrics(ts_ms, rss_kb DESC);`
	idx3 := `CREATE INDEX IF NOT EXISTS idx_cmetrics_ts_cpu ON container_metrics(ts_ms, cpu_pct DESC);`

	if _, err := db.Exec(idx1); err != nil {
		return fmt.Errorf("error creando índice idx_cmetrics_ts: %w", err)
	}
	if _, err := db.Exec(idx2); err != nil {
		return fmt.Errorf("error creando índice idx_cmetrics_ts_rss: %w", err)
	}
	if _, err := db.Exec(idx3); err != nil {
		return fmt.Errorf("error creando índice idx_cmetrics_ts_cpu: %w", err)
	}

	return nil
}

func InsertContainerMetricsBulk(db *sql.DB, snap ContInfoSnapshot, cpuPctMap map[string]float64) error {
	if len(snap.Procesos) == 0 {
		return nil
	}

	fmt.Println("========== InsertContainerMetricsBulk ==========")
	fmt.Println("TS Snapshot:", snap.TsMs)
	fmt.Println("Total procesos en snapshot:", len(snap.Procesos))

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("error iniciando transacción para container_metrics: %w", err)
	}

	stmt, err := tx.Prepare(`
        INSERT INTO container_metrics (
            ts_ms,
            container_id,
            rss_kb,
            cpu_time_ns,
            cpu_pct
        ) VALUES (?, ?, ?, ?, ?);
    `)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("error preparando INSERT en container_metrics: %w", err)
	}
	defer stmt.Close()

	insertCount := 0

	for _, p := range snap.Procesos {

		if !isStressProcess(p) {
			continue
		}

		if p.CmdlineOrContID == "" {
			fmt.Printf("⚠️ Proceso stress con Cmdline vacío, se omite. PID=%d Nombre=%s\n", p.Pid, p.Nombre)
			continue
		}

		cid := normalizeStressContainerID(p)

		// %CPU (si existe en el mapa)
		cpuPct := 0.0
		if cpuPctMap != nil {
			if val, ok := cpuPctMap[p.CmdlineOrContID]; ok {
				cpuPct = val
			}
		}

		fmt.Printf(
			"💾 Insertando en container_metrics: PID=%d cid=%s Nombre=%s RSS_KB=%d CPU_NS=%d cpuPct=%.2f\n",
			p.Pid,
			cid,
			p.Nombre,
			p.RSSKB,
			p.CPUTimeNs,
			cpuPct,
		)

		if _, err := stmt.Exec(
			snap.TsMs,
			cid,
			int64(p.RSSKB),
			int64(p.CPUTimeNs),
			cpuPct,
		); err != nil {
			tx.Rollback()
			return fmt.Errorf("error insertando métricas de contenedor %s (PID=%d): %w",
				cid, p.Pid, err)
		}

		insertCount++
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("error haciendo commit en container_metrics: %w", err)
	}

	fmt.Println("✅ InsertContainerMetricsBulk completado. Filas insertadas:", insertCount)
	return nil
}

func normalizeStressContainerID(p ContProcess) string {
	cid := p.CmdlineOrContID

	if strings.Contains(cid, "--cpu") || strings.Contains(p.Nombre, "stress-ng-cpu") {
		return "stress-high-cpu"
	}

	if strings.Contains(cid, "--vm") || strings.Contains(p.Nombre, "stress-ng-vm") {
		return "stress-high-ram"
	}

	return "stress-low"
}

func isStressProcess(p ContProcess) bool {
	fmt.Println("🔎 Evaluando proceso:")
	fmt.Println("   PID:     ", p.Pid)
	fmt.Println("   Nombre:  ", p.Nombre)
	fmt.Println("   Cmdline: ", p.CmdlineOrContID)

	// 1) Nombre del binario dentro del contenedor
	if strings.Contains(p.Nombre, "stress-ng") {
		fmt.Println("✅ Detectado por Nombre: stress-ng")
		return true
	}

	// 2) ID / nombre del contenedor (como lo creas en el bash)
	if strings.Contains(p.CmdlineOrContID, "stress-cpu") {
		fmt.Println("✅ Detectado por Cmdline: stress-cpu")
		return true
	}

	if strings.Contains(p.CmdlineOrContID, "stress-ram") {
		fmt.Println("✅ Detectado por Cmdline: stress-ram")
		return true
	}

	if strings.Contains(p.CmdlineOrContID, "stress-low") {
		fmt.Println("✅ Detectado por Cmdline: stress-low")
		return true
	}

	// ❌ No es proceso stress
	fmt.Println("❌ No es proceso de stress")
	return false
}

func BuildContainerCpuPct(prev, curr ContInfoSnapshot, numCPUs int) map[string]float64 {
	result := make(map[string]float64)

	if numCPUs <= 0 || curr.TsMs <= prev.TsMs {
		return result
	}

	deltaTimeSec := float64(curr.TsMs-prev.TsMs) / 1000.0
	if deltaTimeSec <= 0 {
		return result
	}

	prevCPU := make(map[string]int64)
	for _, p := range prev.Procesos {
		if p.ContainerRelated != "yes" {
			continue
		}
		cid := p.CmdlineOrContID
		if cid == "" {
			continue
		}
		prevCPU[cid] += int64(p.CPUTimeNs)
	}

	currCPU := make(map[string]int64)
	for _, p := range curr.Procesos {
		if p.ContainerRelated != "yes" {
			continue
		}
		cid := p.CmdlineOrContID
		if cid == "" {
			continue
		}
		currCPU[cid] += int64(p.CPUTimeNs)
	}

	for cid, currVal := range currCPU {
		prevVal, ok := prevCPU[cid]
		if !ok || currVal <= prevVal {
			continue
		}
		deltaNs := float64(currVal - prevVal)
		cpuTimeSec := deltaNs / 1e9

		cpuPct := (cpuTimeSec / deltaTimeSec) * 100.0 / float64(numCPUs)
		result[cid] = cpuPct
	}

	return result
}
