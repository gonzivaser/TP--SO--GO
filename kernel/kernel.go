package main

import (
	"net/http"

	"github.com/sisoputnfrba/tp-golang/kernel/utils"
)

type PCB struct {
	pid            int
	programCounter int
	quantum        int
	cpuReg         RegisterCPU
}

type RegisterCPU struct {
	PC  uint32
	AX  uint8
	BX  uint8
	CX  uint8
	DX  uint8
	EAX uint32
	EBX uint32
	ECX uint32
	EDX uint32
	SI  uint32
	DI  uint32
}

func main() {
	utils.ConfigurarLogger()
	http.HandleFunc("PUT /process", utils.IniciarProceso)
	http.HandleFunc("DELETE /process/{pid}", utils.FinalizarProceso)
	http.HandleFunc("GET /process/{pid}", utils.EstadoProceso)
	http.HandleFunc("PUT /plani", utils.IniciarPlanificacion)
	http.HandleFunc("DELETE /plani", utils.DetenerPlanificacion)
	http.HandleFunc("GET /process", utils.ListarProcesos)
	http.ListenAndServe(":8080", nil)
}
