package main

import (
	"net/http"

	"github.com/sisoputnfrba/tp-golang/kernel/utils"
)

func main() {
	utils.ConfigurarLogger()
	http.HandleFunc("PUT /process", utils.IniciarProceso)
	http.HandleFunc("DELETE /process/{pid}", utils.FinalizarProceso)
	http.HandleFunc("GET /process/{pid}", utils.EstadoProceso)
	http.HandleFunc("PUT /plani", utils.IniciarPlanificacion)
	http.HandleFunc("DELETE /plani", utils.DetenerPlanificacion)
	http.HandleFunc("GET /process", utils.ListarProcesos)
	http.HandleFunc("GET /helloWorld", utils.LlamarCPU)
	http.ListenAndServe(":8080", nil)
}
