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
	http.HandleFunc("GET /input", utils.Prueba)
	http.ListenAndServe(globals.ClientConfig.Puerto, nil)
}
