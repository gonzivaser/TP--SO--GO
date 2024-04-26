package main

import (
	"log"
	"net/http"

	"github.com/sisoputnfrba/tp-golang/kernel/globals"
	"github.com/sisoputnfrba/tp-golang/kernel/utils"
)

func main() {
	utils.ConfigurarLogger()
	globals.ClientConfig = utils.IniciarConfiguracion("config.json")

	if globals.ClientConfig == nil {
		log.Fatalf("No se pudo cargar la configuración")
	}

	http.HandleFunc("PUT /process", utils.IniciarProceso)
	http.HandleFunc("DELETE /process/{pid}", utils.FinalizarProceso)
	http.HandleFunc("GET /process/{pid}", utils.EstadoProceso)
	http.HandleFunc("PUT /plani", utils.IniciarPlanificacion)
	http.HandleFunc("DELETE /plani", utils.DetenerPlanificacion)
	http.HandleFunc("GET /process", utils.ListarProcesos)
	http.HandleFunc("GET /helloWorld", utils.LlamarCPU)
	http.ListenAndServe(globals.ClientConfig.Puerto, nil)
}

//Con el path del endpoint anterior, buscamos el archivo en Memoria (file system de Linux(?))
