package main

import (
	"log"
	"net/http"
	"strconv"

	"github.com/sisoputnfrba/tp-golang/kernel/globals"
	"github.com/sisoputnfrba/tp-golang/kernel/utils"
)

func main() {
	utils.ConfigurarLogger()
	globals.ClientConfig = utils.IniciarConfiguracion("config.json")

	if globals.ClientConfig == nil {
		log.Fatalf("No se pudo cargar la configuración")
	}

	puerto := globals.ClientConfig.Puerto

	http.HandleFunc("PUT /process", utils.IniciarProceso)
	http.HandleFunc("DELETE /process/{pid}", utils.FinalizarProceso)
	http.HandleFunc("GET /process/{pid}", utils.EstadoProceso)
	http.HandleFunc("PUT /plani", utils.IniciarPlanificacion)
	http.HandleFunc("DELETE /plani", utils.DetenerPlanificacion)
	http.HandleFunc("GET /process", utils.ListarProcesos)
	http.ListenAndServe(":"+strconv.Itoa(puerto), nil)
}
