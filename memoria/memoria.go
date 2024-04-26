package main

import (
	"log"
	"net/http"

	"github.com/sisoputnfrba/tp-golang/memoria/globals"
	"github.com/sisoputnfrba/tp-golang/memoria/utils"
)

func main() {
	utils.ConfigurarLogger()
	globals.ClientConfig = utils.IniciarConfiguracion("config.json")

	if globals.ClientConfig == nil {
		log.Fatalf("No se pudo cargar la configuración")
	}

	http.HandleFunc("GET /savedPath/{path}", utils.ProcessSavedPathFromKernel)
	http.ListenAndServe(":8085", nil)
}
