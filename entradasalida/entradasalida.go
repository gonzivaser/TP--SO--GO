package main

import (
	"log"
	"net/http"

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

	// configFile := "config.json"
	// config, err := utils.CargarConfiguracion(configFile)
	// if err != nil {
	// 	log.Fatalf("Error al cargar la configuración: %v", err)
	// }

	// Crear la interfaz y pasar la configuración cargada
	// interfaz := utils.InterfazIO{
	// 	Nombre: "Generica",
	// 	Config: config,
	// }

	http.HandleFunc("GET /input", utils.Prueba)
	http.HandleFunc("GET /interfaz", utils.Iniciar)

	// Cargar la configuración desde el archivo

	// Iniciar la interfaz
	//interfaz.Iniciar()
	http.ListenAndServe(":8090", nil)
}
