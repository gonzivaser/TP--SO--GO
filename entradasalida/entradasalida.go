package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/sisoputnfrba/tp-golang/entradasalida/globals"
	"github.com/sisoputnfrba/tp-golang/entradasalida/utils"
)

func main() {
	utils.ConfigurarLogger()
	globals.ClientConfig = utils.IniciarConfiguracion("config.json")

	if globals.ClientConfig == nil {
		log.Fatalf("No se pudo cargar la configuración")
	}

	utils.ConfigurarLogger()

	http.HandleFunc("GET /input", utils.Prueba)

	configFile := "config.json"

	// Cargar la configuración desde el archivo
	config, err := utils.CargarConfiguracion(configFile)
	if err != nil {
		log.Fatalf("Error al cargar la configuración: %v", err)
	}

	// Crear la interfaz y pasar la configuración cargada
	interfaz := utils.InterfazIO{
		Nombre: "Interfaz_Generica",
		Config: config,
	}

	// Iniciar la interfaz
	interfaz.Iniciar()
	espera := utils.IO_GEN_SLEEP(&interfaz, 10)
	fmt.Printf("Esperando por %v...\n", espera)
	time.Sleep(espera)
	fmt.Println("holaaaaa")
	http.ListenAndServe(globals.ClientConfig.Puerto, nil)
}
