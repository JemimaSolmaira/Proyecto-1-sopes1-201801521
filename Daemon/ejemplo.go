package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// Estructuras para los datos de monitoreo
type RAMInfo struct {
	Total      int64 `json:"total"`
	Libre      int64 `json:"libre"`
	Uso        int64 `json:"uso"`
	Porcentaje int64 `json:"porcentaje"`
	Timestamp  int64 `json:"timestamp"`
}

type CPUInfo struct {
	PorcentajeUso int64 `json:"porcentajeUso"`
	Timestamp     int64 `json:"timestamp"`
}

type SystemMetrics struct {
	RAM RAMInfo `json:"ram"`
	CPU CPUInfo `json:"cpu"`
}

// Canales para comunicación entre goroutines
type Channels struct {
	RAMChan   chan RAMInfo
	CPUChan   chan CPUInfo
	ErrorChan chan error
	StopChan  chan bool
}

// Configuración del agente (ya no contiene endpoints)
type Config struct {
	MonitorInterval time.Duration
	RAMProcFile     string
	CPUProcFile     string
	MaxRetries      int
}

// Agente de monitoreo principal
type MonitoringAgent struct {
	config   Config
	channels Channels
	wg       sync.WaitGroup
}

// main
func iniciar_monitoreo() {
	// Obtener configuración desde variables de entorno (solo para host/port si se necesitara)
	// Ahora no se usan endpoints, pero dejamos la obtención por compatibilidad si la quieres reactivar
	apiHost := os.Getenv("API_HOST")
	if apiHost == "" {
		apiHost = "monitor_api"
	}
	apiPort := os.Getenv("API_PORT")
	if apiPort == "" {
		apiPort = "3001"
	}
	_ = apiHost
	_ = apiPort

	// Configuración del agente
	config := Config{
		MonitorInterval: 5 * time.Second,
		RAMProcFile:     "/proc/ram_so1_201801521",
		CPUProcFile:     "/proc/cpu_so1_201801521",
		MaxRetries:      3,
	}

	// Inicializar canales
	channels := Channels{
		RAMChan:   make(chan RAMInfo, 10),
		CPUChan:   make(chan CPUInfo, 10),
		ErrorChan: make(chan error, 10),
		StopChan:  make(chan bool, 5),
	}

	// Crear agente de monitoreo
	agent := &MonitoringAgent{
		config:   config,
		channels: channels,
	}

	log.Println("🚀 Iniciando Agente de Monitoreo de Sistema (salida por consola)")
	log.Printf("📊 Intervalo de monitoreo: %v", config.MonitorInterval)

	// Iniciar agente
	agent.Start()

	// Manejar señales de sistema para cierre graceful
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Esperar señal de cierre
	<-sigChan
	log.Println("🛑 Recibida señal de cierre, deteniendo agente...")

	// Detener agente
	agent.Stop()
	log.Println("✅ Agente de monitoreo detenido correctamente")
}

// Iniciar todas las goroutines del agente
func (ma *MonitoringAgent) Start() {
	log.Println("🔄 Iniciando goroutines de monitoreo...")

	// Iniciar monitores
	ma.wg.Add(1)
	go ma.monitorRAM()

	ma.wg.Add(1)
	go ma.monitorCPU()

	// Iniciar "enviadores" — ahora solo imprimen en consola
	ma.wg.Add(1)
	go ma.printRAM()

	ma.wg.Add(1)
	go ma.printCPU()

	// Iniciar manejador de errores
	ma.wg.Add(1)
	go ma.handleErrors()

	log.Println("✅ Todas las goroutines iniciadas correctamente")
}

// Detener todas las goroutines
func (ma *MonitoringAgent) Stop() {
	log.Println("🔄 Deteniendo goroutines...")

	// Enviar señal de stop a todas las goroutines
	// enviamos tantas señales como goroutines consumidoras esperamos (seguro)
	for i := 0; i < 5; i++ {
		select {
		case ma.channels.StopChan <- true:
		default:
		}
	}

	// Esperar que todas las goroutines terminen
	ma.wg.Wait()

	// Cerrar canales (solo si estamos seguros de que no quedarán escritores)
	close(ma.channels.RAMChan)
	close(ma.channels.CPUChan)
	close(ma.channels.ErrorChan)
	close(ma.channels.StopChan)

	log.Println("✅ Todas las goroutines detenidas")
}

func (ma *MonitoringAgent) monitorRAM() {
	defer ma.wg.Done()
	ticker := time.NewTicker(ma.config.MonitorInterval)
	defer ticker.Stop()

	log.Println("📊 Iniciando monitoreo de RAM")

	for {
		select {
		case <-ticker.C:
			ramInfo, err := ma.readRAMInfo()
			if err != nil {
				select {
				case ma.channels.ErrorChan <- fmt.Errorf("error leyendo RAM: %v", err):
				default:
				}
				continue
			}

			select {
			case ma.channels.RAMChan <- ramInfo:
				log.Printf("📥 Capturada métrica RAM: %d%% usado (%d/%d KB)",
					ramInfo.Porcentaje, ramInfo.Uso, ramInfo.Total)
			default:
				log.Println("⚠️ Canal RAM lleno, descartando datos")
			}

		case <-ma.channels.StopChan:
			log.Println("🛑 Deteniendo monitoreo de RAM")
			return
		}
	}
}

// Goroutine para monitorear CPU (simula lectura)
// Aquí debes reemplazar readCPUInfo() por tu implementación real
func (ma *MonitoringAgent) monitorCPU() {
	defer ma.wg.Done()
	ticker := time.NewTicker(ma.config.MonitorInterval)
	defer ticker.Stop()

	log.Println("📊 Iniciando monitoreo de CPU")

	for {
		select {
		case <-ticker.C:
			cpuInfo, err := ma.readCPUInfo()
			if err != nil {
				select {
				case ma.channels.ErrorChan <- fmt.Errorf("error leyendo CPU: %v", err):
				default:
				}
				continue
			}

			select {
			case ma.channels.CPUChan <- cpuInfo:
				log.Printf("📥 Capturada métrica CPU: %d%% uso", cpuInfo.PorcentajeUso)
			default:
				log.Println("⚠️ Canal CPU lleno, descartando datos")
			}

		case <-ma.channels.StopChan:
			log.Println("🛑 Deteniendo monitoreo de CPU")
			return
		}
	}
}

// Antes: sendRAMToAPI -> Ahora: printRAM (imprime en consola)
func (ma *MonitoringAgent) printRAM() {
	defer ma.wg.Done()

	log.Println("🌐 Iniciando impresor de métricas RAM (consola)")

	for {
		select {
		case ramInfo := <-ma.channels.RAMChan:
			// Convertir a JSON para salida consistente
			b, err := json.MarshalIndent(ramInfo, "", "  ")
			if err != nil {
				ma.channels.ErrorChan <- fmt.Errorf("error formateando RAM a JSON: %v", err)
				continue
			}
			fmt.Println("=== Métrica RAM (consola) ===")
			fmt.Println(string(b))

		case <-ma.channels.StopChan:
			log.Println("🛑 Deteniendo impresor de RAM")
			return
		}
	}
}

// Antes: sendCPUToAPI -> Ahora: printCPU (imprime en consola)
func (ma *MonitoringAgent) printCPU() {
	defer ma.wg.Done()

	log.Println("🌐 Iniciando impresor de métricas CPU (consola)")

	for {
		select {
		case cpuInfo := <-ma.channels.CPUChan:
			// Convertir a JSON para salida consistente
			b, err := json.MarshalIndent(cpuInfo, "", "  ")
			if err != nil {
				ma.channels.ErrorChan <- fmt.Errorf("error formateando CPU a JSON: %v", err)
				continue
			}
			fmt.Println("=== Métrica CPU (consola) ===")
			fmt.Println(string(b))

		case <-ma.channels.StopChan:
			log.Println("🛑 Deteniendo impresor de CPU")
			return
		}
	}
}

// Goroutine para manejar errores
func (ma *MonitoringAgent) handleErrors() {
	defer ma.wg.Done()

	log.Println("🚨 Iniciando manejador de errores")

	for {
		select {
		case err := <-ma.channels.ErrorChan:
			log.Printf("❌ Error: %v", err)

		case <-ma.channels.StopChan:
			log.Println("🛑 Deteniendo manejador de errores")
			return
		}
	}
}

// Leer información de RAM desde el módulo del kernel
func (ma *MonitoringAgent) readRAMInfo() (RAMInfo, error) {
	data, err := ioutil.ReadFile(ma.config.RAMProcFile)
	if err != nil {
		return RAMInfo{}, fmt.Errorf("no se pudo leer %s: %v", ma.config.RAMProcFile, err)
	}

	var ramInfo RAMInfo
	err = json.Unmarshal(data, &ramInfo)
	if err != nil {
		return RAMInfo{}, fmt.Errorf("error parsing JSON de RAM: %v", err)
	}

	// Agregar timestamp
	ramInfo.Timestamp = time.Now().Unix()

	return ramInfo, nil
}

// Leer información de CPU desde el módulo del kernel
func (ma *MonitoringAgent) readCPUInfo() (CPUInfo, error) {
	data, err := ioutil.ReadFile(ma.config.CPUProcFile)
	if err != nil {
		return CPUInfo{}, fmt.Errorf("no se pudo leer %s: %v", ma.config.CPUProcFile, err)
	}

	var cpuInfo CPUInfo
	err = json.Unmarshal(data, &cpuInfo)
	if err != nil {
		return CPUInfo{}, fmt.Errorf("error parsing JSON de CPU: %v", err)
	}

	// Agregar timestamp
	cpuInfo.Timestamp = time.Now().Unix()

	return cpuInfo, nil
}

// Función de utilidad para verificar si los archivos /proc existen
func (ma *MonitoringAgent) CheckProcFiles() error {
	// Verificar archivo de RAM
	if _, err := ioutil.ReadFile(ma.config.RAMProcFile); err != nil {
		return fmt.Errorf("archivo RAM no disponible (%s): %v. ¿Está el módulo del kernel cargado?",
			ma.config.RAMProcFile, err)
	}

	// Verificar archivo de CPU
	if _, err := ioutil.ReadFile(ma.config.CPUProcFile); err != nil {
		return fmt.Errorf("archivo CPU no disponible (%s): %v. ¿Está el módulo del kernel cargado?",
			ma.config.CPUProcFile, err)
	}

	return nil
}

// Función para probar la lectura de los módulos (útil para debugging)
func (ma *MonitoringAgent) TestReading() {
	fmt.Println("🧪 Probando lectura de módulos del kernel...")

	// Probar RAM
	ramInfo, err := ma.readRAMInfo()
	if err != nil {
		fmt.Printf("❌ Error leyendo RAM: %v\n", err)
	} else {
		fmt.Printf("✅ RAM OK: %+v\n", ramInfo)
	}

	// Probar CPU
	cpuInfo, err := ma.readCPUInfo()
	if err != nil {
		fmt.Printf("❌ Error leyendo CPU: %v\n", err)
	} else {
		fmt.Printf("✅ CPU OK: %+v\n", cpuInfo)
	}
}
