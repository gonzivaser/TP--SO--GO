package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"log"
	"net/http"
	"os"

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
	Tipo           string `json:"type"`
	Port           int    `json:"port"`
	UnidadDeTiempo int    `json:"unit_work_time"`
	IPKernel       string `json:"ip_kernel"`
}

// InterfazIO es la estructura para la interfaz de entrada/salida
type InterfazIO struct {
	Nombre string
	Config Config
}

// Carga la configuración desde un archivo JSON
func CargarConfiguracion(configFile string) (Config, error) {
	data, err := os.ReadFile(configFile)
	if err != nil {
		return Config{}, fmt.Errorf("error al leer el archivo: %v", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return Config{}, fmt.Errorf("error al deserializar JSON: %v", err)
	}

	return config, nil
}

// Inicia la interfaz
func (io *InterfazIO) Iniciar() {
	fmt.Printf("Interfaz '%s' iniciada en :%d %d '%s'\n", io.Nombre, io.Config.Port, io.Config.UnidadDeTiempo, io.Config.IPKernel)
}

func IO_GEN_SLEEP(io *InterfazIO, N int) time.Duration {
	return time.Duration(N*io.Config.UnidadDeTiempo) * time.Millisecond
}

//time.Sleep(time.Duration(io.Config.UnidadDeTiempo) * time.Millisecond(n))
