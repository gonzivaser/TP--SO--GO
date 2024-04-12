package main

import (
	"net/http"
	"server/utils"
)

func main() {
	http.HandleFunc("PUT /process", utils.IniciarProceso)
	http.HandleFunc("POST /helloworld", utils.HelloWorld)
	//http.HandleFunc("DELETE /process/{pid}", utils.FinalizarProceso)
	//http.HandleFunc("GET /process/{pid}", utils.EstadoProceso)
	//http.HandleFunc("PUT /plani", utils.IniciarPlanificacion)
	//http.HandleFunc("DELETE /plani", utils.DetenerPlanificacion)
	//http.HandleFunc("GET /process", utils.ListarProcesos)
	http.ListenAndServe(":8080", nil)
}
