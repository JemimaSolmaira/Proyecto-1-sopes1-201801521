package main

import (
	"encoding/json"
	"fmt"
	"os"

	"time"
)

// CONTENEDORES
type ContInfoSnapshot struct {
	TotalRAMKB uint64        `json:"total_ram_kb"`
	FreeRAMKB  uint64        `json:"free_ram_kb"`
	UsedRAMKB  uint64        `json:"used_ram_kb"`
	TsMs       int64         `json:"ts_ms"`
	Procesos   []ContProcess `json:"procesos"`
}

// ContProcess representa cada entrada de "procesos"
type ContProcess struct {
	Pid              int    `json:"pid"`
	Nombre           string `json:"nombre"`
	CmdlineOrContID  string `json:"cmdline_or_container_id"`
	VSZKB            uint64 `json:"vsz_kb"`
	RSSKB            uint64 `json:"rss_kb"`
	MemPercent       uint64 `json:"mem_percent"`
	CPUTimeNs        uint64 `json:"cpu_time_ns"`
	Estado           string `json:"estado"`
	ContainerRelated string `json:"container_related"`
}

func ReadContInfo(path string) (ContInfoSnapshot, error) {
	var snap ContInfoSnapshot

	data, err := os.ReadFile(path)
	if err != nil {
		return snap, fmt.Errorf("no se pudo leer %s: %w", path, err)
	}

	if err := json.Unmarshal(data, &snap); err != nil {
		return snap, fmt.Errorf("error al parsear JSON de %s: %w", path, err)
	}

	return snap, nil
}

func PrintContainers(snap ContInfoSnapshot) {
	ts := time.UnixMilli(snap.TsMs)

	fmt.Println("======================================")
	fmt.Println("  CONTENEDORES DETECTADOS")
	fmt.Println("======================================")
	fmt.Printf("Timestamp: %d (%s)\n", snap.TsMs, ts.Format(time.RFC3339))
	fmt.Printf("RAM Total: %d KB | Libre: %d KB | Usada: %d KB\n",
		snap.TotalRAMKB, snap.FreeRAMKB, snap.UsedRAMKB)
	fmt.Println()

	count := 0

	for _, p := range snap.Procesos {
		if p.ContainerRelated == "yes" {
			count++
			fmt.Printf("📦 Contenedor #%d\n", count)
			fmt.Printf("   PID:        %d\n", p.Pid)
			fmt.Printf("   Nombre:     %s\n", p.Nombre)
			fmt.Printf("   Cmd/ID:     %s\n", p.CmdlineOrContID)
			fmt.Printf("   RAM (RSS):  %d KB\n", p.RSSKB)
			fmt.Printf("   RAM (VSZ):  %d KB\n", p.VSZKB)
			fmt.Printf("   RAM %%:     %d %%\n", p.MemPercent)
			fmt.Printf("   CPU (ns):   %d\n", p.CPUTimeNs)
			fmt.Printf("   Estado:     %s\n", p.Estado)
			fmt.Println("--------------------------------------")
		}
	}

	if count == 0 {
		fmt.Println("No se detectaron contenedores en este snapshot.")
	} else {
		fmt.Printf("Total de contenedores detectados: %d\n", count)
	}

	fmt.Println("======================================")
}
