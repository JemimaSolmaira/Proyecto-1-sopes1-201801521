package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const (
	desiredLowContainers  = 3
	desiredHighContainers = 2
	defaultInterval       = 10 * time.Second
)

const grafanaContainerName = "grafana-sqlite"

const (
	stressPrefix  = "stress-"
	lowPrefix     = "stress-low-"
	highCPUPrefix = "stress-high-cpu-"
	highRAMPrefix = "stress-high-ram-"
)

type ContainerInfo struct {
	ID    string
	Name  string
	Image string
}

func runCmd(timeout time.Duration, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("error ejecutando %s %v: %v - stderr: %s",
			name, args, err, stderr.String())
	}
	return out.String(), nil
}

func listStressContainers() (lows []ContainerInfo, highsCPU []ContainerInfo, highsRAM []ContainerInfo, err error) {
	out, err := runCmd(5*time.Second,
		"docker", "ps",
		"--filter", "status=running",
		"--filter", "name="+stressPrefix,
		"--format", "{{.ID}} {{.Names}} {{.Image}}",
	)
	if err != nil {
		return nil, nil, nil, err
	}

	scanner := bufio.NewScanner(strings.NewReader(out))
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		id := parts[0]
		name := parts[1]
		image := ""
		if len(parts) >= 3 {
			image = parts[2]
		}

		ci := ContainerInfo{
			ID:    id,
			Name:  name,
			Image: image,
		}

		switch {
		case strings.HasPrefix(name, lowPrefix):
			lows = append(lows, ci)
		case strings.HasPrefix(name, highCPUPrefix):
			highsCPU = append(highsCPU, ci)
		case strings.HasPrefix(name, highRAMPrefix):
			highsRAM = append(highsRAM, ci)
		default:

		}
	}

	if err := scanner.Err(); err != nil {
		return nil, nil, nil, err
	}
	return lows, highsCPU, highsRAM, nil
}

func stopContainer(c ContainerInfo, reason string) {
	fmt.Printf("  -> Deteniendo contenedor %s (%s) [motivo: %s]\n",
		c.Name, c.ID, reason)
	_, err := runCmd(10*time.Second, "docker", "stop", c.ID)
	if err != nil {
		fmt.Printf("     (error al detener %s: %v)\n", c.Name, err)
	}
}

func enforceRules() {
	lows, highsCPU, highsRAM, err := listStressContainers()
	if err != nil {
		fmt.Printf(" Error listando contenedores: %v\n", err)
		return
	}

	lowCount := len(lows)
	highCount := len(highsCPU) + len(highsRAM)

	fmt.Println("===========================================")
	fmt.Println("Estado actual de contenedores stress-*")
	fmt.Printf("  Bajo consumo (low):       %d\n", lowCount)
	fmt.Printf("  Alto consumo CPU:         %d\n", len(highsCPU))
	fmt.Printf("  Alto consumo RAM:         %d\n", len(highsRAM))
	fmt.Printf("  Alto consumo TOTAL (CPU+RAM): %d\n", highCount)
	fmt.Println("===========================================")

	if highCount > desiredHighContainers {
		toKill := highCount - desiredHighContainers
		fmt.Printf("⚠ Hay %d contenedores de ALTO consumo extra, se eliminarán...\n", toKill)

		for _, c := range highsCPU {
			if toKill <= 0 {
				break
			}
			stopContainer(c, "exceso de alto consumo CPU")
			toKill--
		}
		for _, c := range highsRAM {
			if toKill <= 0 {
				break
			}
			stopContainer(c, "exceso de alto consumo RAM")
			toKill--
		}
	} else {
		fmt.Println(" No hay exceso de contenedores de ALTO consumo.")
	}

	lows, highsCPU, highsRAM, err = listStressContainers()
	if err != nil {
		fmt.Printf(" Error listando contenedores tras eliminar altos: %v\n", err)
		return
	}
	lowCount = len(lows)

	if lowCount > desiredLowContainers {
		toKill := lowCount - desiredLowContainers
		fmt.Printf("Hay %d contenedores de BAJO consumo extra, se eliminarán...\n", toKill)

		for _, c := range lows {
			if toKill <= 0 {
				break
			}
			stopContainer(c, "exceso de bajo consumo")
			toKill--
		}
	} else {
		fmt.Println(" No hay exceso de contenedores de BAJO consumo.")
	}

	fmt.Println()
}
