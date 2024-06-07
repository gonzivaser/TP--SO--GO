package main

import (
	"log"
	"net/http"
	"os"
	"strconv"

	//"github.com/sisoputnfrba/tp-golang/entradasalida/globals"
	"github.com/sisoputnfrba/tp-golang/entradasalida/utils"
)

func main() {
	utils.ConfigurarLogger()

	interfaceName := os.Args[1]
	log.Printf("Nombre de la interfaz: %s", interfaceName)
	pathToConfig := os.Args[2]
	log.Printf("Path al archivo de configuración: %s", pathToConfig)

	config, err := utils.LoadConfig(pathToConfig)
	if err != nil {
		log.Fatalf("Error al cargar la configuración desde '%s': %v", pathToConfig, err)
	}
	utils.SendPort(interfaceName, pathToConfig)
	Puerto := config.Puerto
	//http.HandleFunc("GET /input", utils.Prueba)
	http.HandleFunc("POST /recieveREG", utils.RecieveREG)
	http.HandleFunc("/interfaz", utils.Iniciar)
	http.HandleFunc("/receiveContentFromMemory", utils.ReceiveContentFromMemory)

	// Cargar la configuración desde el archivo

	http.ListenAndServe(":"+strconv.Itoa(Puerto), nil)
}
