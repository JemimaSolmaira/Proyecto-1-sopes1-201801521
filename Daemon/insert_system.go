package main

import (
	"database/sql"
	"fmt"
)

func CreateSystemMetricsTable(db *sql.DB) error {
	ddl := `
    CREATE TABLE IF NOT EXISTS system_metrics (
        id             INTEGER PRIMARY KEY AUTOINCREMENT,
        ts_ms          BIGINT NOT NULL,
        total_ram_kb   BIGINT NOT NULL,
        free_ram_kb    BIGINT NOT NULL,
        available_kb   BIGINT,
        ram_used_kb    BIGINT NOT NULL,
        total_procs    INT NOT NULL,
        cpu_usage_pct  REAL,
        created_at     TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    );
    `
	_, err := db.Exec(ddl)
	if err != nil {
		return fmt.Errorf("error creando tabla system_metrics: %w", err)
	}
	return nil
}

func CreateProcessMetricsTable(db *sql.DB) error {
	ddl := `
    CREATE TABLE IF NOT EXISTS process_metrics (
        id         INTEGER PRIMARY KEY AUTOINCREMENT,
        ts_ms      BIGINT NOT NULL,
        pid        INT NOT NULL,
        comm       TEXT,
        state      CHAR(1),
        rss_kb     BIGINT,
        utime      BIGINT,
        stime      BIGINT,
        cpu_pct    REAL,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    );
    `
	if _, err := db.Exec(ddl); err != nil {
		return fmt.Errorf("error creando tabla process_metrics: %w", err)
	}

	// Índices
	idx1 := `CREATE INDEX IF NOT EXISTS idx_proc_ts ON process_metrics(ts_ms);`
	idx2 := `CREATE INDEX IF NOT EXISTS idx_proc_ts_rss ON process_metrics(ts_ms, rss_kb DESC);`
	idx3 := `CREATE INDEX IF NOT EXISTS idx_proc_ts_cpu ON process_metrics(ts_ms, cpu_pct DESC);`

	if _, err := db.Exec(idx1); err != nil {
		return fmt.Errorf("error creando índice idx_proc_ts: %w", err)
	}
	if _, err := db.Exec(idx2); err != nil {
		return fmt.Errorf("error creando índice idx_proc_ts_rss: %w", err)
	}
	if _, err := db.Exec(idx3); err != nil {
		return fmt.Errorf("error creando índice idx_proc_ts_cpu: %w", err)
	}

	return nil
}

// InsertSystemMetrics insert en system_metrics con datos de SysInfo.
func InsertSystemMetrics(db *sql.DB, si SysInfo) (int64, error) {
	ramUsed := int64(si.RamUsedKB)
	if ramUsed == 0 && si.TotalRAMKB > 0 {
		ramUsed = int64(si.TotalRAMKB - si.FreeRAMKB)
	}

	var available *int64
	if si.AvailableKB > 0 {
		v := int64(si.AvailableKB)
		available = &v
	}

	cpuPct := float64(si.CPUUsagePct)

	query := `
        INSERT INTO system_metrics (
            ts_ms,
            total_ram_kb,
            free_ram_kb,
            available_kb,
            ram_used_kb,
            total_procs,
            cpu_usage_pct
        ) VALUES (?, ?, ?, ?, ?, ?, ?);
    `

	res, err := db.Exec(
		query,
		int64(si.TsMs),
		int64(si.TotalRAMKB),
		int64(si.FreeRAMKB),
		available,
		ramUsed,
		int64(si.TotalProcs),
		cpuPct,
	)
	if err != nil {
		return 0, fmt.Errorf("error insertando en system_metrics: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("no se pudo obtener LastInsertId: %w", err)
	}

	return id, nil
}

// InsertProcessMetricsBulk insert de procesos de  SysInfo en process_metrics.
func InsertProcessMetricsBulk(db *sql.DB, si SysInfo, cpuPctMap map[int]float64) error {
	if len(si.Procesos) == 0 {
		return nil // nada que insertar
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("error iniciando transacción para process_metrics: %w", err)
	}

	stmt, err := tx.Prepare(`
        INSERT INTO process_metrics (
            ts_ms,
            pid,
            comm,
            state,
            rss_kb,
            utime,
            stime,
            cpu_pct
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?);
    `)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("error preparando INSERT en process_metrics: %w", err)
	}
	defer stmt.Close()

	for _, p := range si.Procesos {
		// cpu_pct opcional por PID
		var cpuPct interface{} = nil
		if cpuPctMap != nil {
			if val, ok := cpuPctMap[p.Pid]; ok {
				cpuPct = val
			}
		}

		if _, err := stmt.Exec(
			int64(si.TsMs),
			p.Pid,
			p.Comm,
			p.State,
			int64(p.RssKB),
			int64(p.Utime),
			int64(p.Stime),
			cpuPct,
		); err != nil {
			tx.Rollback()
			return fmt.Errorf("error insertando proceso PID=%d en process_metrics: %w", p.Pid, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("error haciendo commit en process_metrics: %w", err)
	}

	return nil
}

func BuildProcCpuPct(prev, curr SysInfo, numCPUs int, hz float64) map[int]float64 {
	result := make(map[int]float64)

	if numCPUs <= 0 {
		return result
	}
	if curr.TsMs <= prev.TsMs {
		return result
	}

	deltaTimeSec := float64(curr.TsMs-prev.TsMs) / 1000.0
	if deltaTimeSec <= 0 {
		return result
	}

	prevTicks := make(map[int]uint64, len(prev.Procesos))
	for _, p := range prev.Procesos {
		prevTicks[p.Pid] = p.Utime + p.Stime
	}

	for _, p := range curr.Procesos {
		currTicks := p.Utime + p.Stime
		oldTicks, ok := prevTicks[p.Pid]
		if !ok {
			continue
		}
		if currTicks <= oldTicks {
			continue
		}

		deltaTicks := float64(currTicks - oldTicks)
		cpuTimeSec := deltaTicks / hz
		cpuPct := (cpuTimeSec / deltaTimeSec) * 100.0 / float64(numCPUs)

		result[p.Pid] = cpuPct
	}

	return result
}

func CreateProcessStateSummaryTable(db *sql.DB) error {
	ddl := `
    CREATE TABLE IF NOT EXISTS process_state_summary (
        ts_ms  BIGINT NOT NULL,
        state  CHAR(1) NOT NULL,
        count  INT NOT NULL
    );
    `
	if _, err := db.Exec(ddl); err != nil {
		return fmt.Errorf("error creando tabla process_state_summary: %w", err)
	}

	idx := `CREATE INDEX IF NOT EXISTS idx_state_ts ON process_state_summary(ts_ms);`
	if _, err := db.Exec(idx); err != nil {
		return fmt.Errorf("error creando índice idx_state_ts: %w", err)
	}

	return nil
}

// InsertProcessStateSummary insert de resumen de estados de procesos para un snapshot.
func InsertProcessStateSummary(db *sql.DB, si SysInfo) error {
	if len(si.Procesos) == 0 {
		return nil
	}

	// Contar procesos por estado
	counts := make(map[string]int)
	for _, p := range si.Procesos {
		state := p.State
		if state == "" {
			state = "?"
		} else if len(state) > 1 {
			state = state[:1]
		}
		counts[state]++
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("error iniciando transacción para process_state_summary: %w", err)
	}

	stmt, err := tx.Prepare(`
        INSERT INTO process_state_summary (
            ts_ms,
            state,
            count
        ) VALUES (?, ?, ?);
    `)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("error preparando INSERT en process_state_summary: %w", err)
	}
	defer stmt.Close()

	for state, cnt := range counts {
		if _, err := stmt.Exec(
			int64(si.TsMs),
			state,
			cnt,
		); err != nil {
			tx.Rollback()
			return fmt.Errorf("error insertando resumen state=%s en process_state_summary: %w", state, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("error haciendo commit en process_state_summary: %w", err)
	}

	return nil
}
