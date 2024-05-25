package utils

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/sisoputnfrba/tp-golang/entradasalida/globals"
)

type PruebaMensaje struct {
	Mensaje string `json:"Prueba"`
}

func ConfigurarLogger() {
	logFile, err := os.OpenFile("entradasalida.log", os.O_CREATE|os.O_APPEND|os.O_RDWR, 0666)
	if err != nil {
		panic(err)
	}
	mw := io.MultiWriter(os.Stdout, logFile)
	log.SetOutput(mw)
}

func IniciarConfiguracion(filePath string) *globals.Config {
	var config *globals.Config
	configFile, err := os.Open(filePath)
	if err != nil {
		log.Fatal(err.Error())
	}
	defer configFile.Close()

	jsonParser := json.NewDecoder(configFile)
	jsonParser.Decode(&config)

	return config
}

func Prueba(w http.ResponseWriter, r *http.Request) {

	Prueba := PruebaMensaje{
		Mensaje: "Todo OK IO",
	}

	pruebaResponse, _ := json.Marshal(Prueba)

	w.WriteHeader(http.StatusOK)
	w.Write(pruebaResponse)
}

type Config struct {
	Type         string `json:"type"`
	UnitWorkTime int    `json:"unit_work_time"` // Tiempo por unidad de trabajo
	IPKernel     string `json:"ip_kernel"`      // Dirección IP del Kernel
	PortKernel   int    `json:"port_kernel"`    // Puerto del Kernel
}

// Estructura para la interfaz genérica
type InterfazIO struct {
	Nombre string // Nombre único
	Config Config // Configuración
}

// Método para esperar basado en unidades de trabajo
func (gi *InterfazIO) IO_GEN_SLEEP(n int) time.Duration {
	return time.Duration(n*gi.Config.UnitWorkTime) * time.Millisecond
}

func LoadConfig(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var config Config
	err = json.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}

type Payload struct {
	IO int
}

func Iniciar(w http.ResponseWriter, r *http.Request) {
	log.Printf("Recibiendo solicitud de I/O desde el kernel")
	var payload Payload
	err := json.NewDecoder(r.Body).Decode(&payload)
	if err != nil {
		http.Error(w, "Error al decodificar los datos JSON", http.StatusInternalServerError)
		return
	}

	N := payload.IO

	interfaceName := os.Args[1]
	log.Printf("Nombre de la interfaz: %s", interfaceName)
	pathToConfig := os.Args[2]
	log.Printf("Path al archivo de configuración: %s", pathToConfig)

	config, err := LoadConfig(pathToConfig)
	if err != nil {
		log.Fatalf("Error al cargar la configuración desde '%s': %v", pathToConfig, err)
	}
	gi := &InterfazIO{
		Nombre: interfaceName,
		Config: *config,
	}
	duracion := gi.IO_GEN_SLEEP(N)
	log.Printf("La espera por %d unidades para la interfaz '%s' es de %v\n", N, gi.Nombre, duracion)
	time.Sleep(duracion)
	log.Printf("Termino de esperar por la interfaz genérica '%s' es de %v\n", gi.Nombre, duracion)
}
