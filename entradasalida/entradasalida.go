package main

import (
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/sisoputnfrba/tp-golang/entradasalida/globals"
	"github.com/sisoputnfrba/tp-golang/entradasalida/utils"
)

func main() {
	utils.ConfigurarLogger()
	globals.ClientConfig = utils.IniciarConfiguracion("config.json")

	if globals.ClientConfig == nil {
		log.Fatalf("No se pudo cargar la configuración")
	}

	puerto := globals.ClientConfig.Puerto
	interfaceName := os.Args[1]
	log.Printf("Nombre de la interfaz: %s", interfaceName)
	pathToConfig := os.Args[2]
	log.Printf("Path al archivo de configuración: %s", pathToConfig)

	//http.HandleFunc("GET /input", utils.Prueba)
	http.HandleFunc("/interfaz", utils.Iniciar)

	// Cargar la configuración desde el archivo

	http.ListenAndServe(":"+strconv.Itoa(puerto), nil)
}
