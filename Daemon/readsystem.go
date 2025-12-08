package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"time"
)

// PROCESOS DEL SISTEMA
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

type SysInfo struct {
	TotalRAMKB     uint64    `json:"total_ram_kb"`
	FreeRAMKB      uint64    `json:"free_ram_kb"`
	AvailableKB    uint64    `json:"available_kb"`
	RamUsedKB      uint64    `json:"ram_used_kb"`
	TotalProcs     int64     `json:"total_procs"`
	CPUUsagePct    uint64    `json:"cpu_usage_pct"`
	TsMs           uint64    `json:"ts_ms"`
	Procesos       []Process `json:"procesos"`
	RawJSONPresent bool
}

func ReadSysinfo(path string) (SysInfo, error) {
	var si SysInfo

	f, err := os.Open(path)
	if err != nil {
		return si, fmt.Errorf("no se pudo abrir %s: %w", path, err)
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		return si, fmt.Errorf("error leyendo %s: %w", path, err)
	}

	if err := json.Unmarshal(data, &si); err != nil {
		return si, fmt.Errorf("error parseando JSON de %s: %w\ncontenido recibido:\n%s", path, err, string(data))
	}

	si.RawJSONPresent = true
	return si, nil
}

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
			return ci > cj
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
		fmt.Println("Todos los utime/stime vienen en 0; no se puede calcular Top CPU aún.")
		fmt.Println("   (Necesitas que el módulo del kernel llene utime/stime para cada proceso.)")
		fmt.Println()
	}

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
