package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
)

type BodyResponseListProcess struct {
	Pid   int    `json:"pid"`
	State string `json:"state"`
}

type BodyResponsePid struct {
	Pid int `json:"pid"`
}

type BodyResponseState struct {
	State string `json:"state"`
}

type BodyRequest struct {
	Path string `json:"path"`
}

type BodyResponsePCB struct { //ESTO NO VA ACA
	Pcb PCB `json:"pcb"`
}

type PCB struct { //ESTO NO VA ACA
	Pid            int
	ProgramCounter int
	Quantum        int
	CpuReg         RegisterCPU
}

type RegisterCPU struct { //ESTO NO VA ACA
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

var pcb1 = PCB{ //ESTO NO VA ACA
	Pid:            1,
	ProgramCounter: 0,
	Quantum:        0,
	CpuReg: RegisterCPU{
		PC:  0,
		AX:  0,
		BX:  0,
		CX:  0,
		DX:  0,
		EAX: 0,
		EBX: 0,
		ECX: 0,
		EDX: 0,
		SI:  0,
		DI:  0,
	},
}

var savedPath BodyRequest

func IniciarProceso(w http.ResponseWriter, r *http.Request) {

	var request BodyRequest
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		http.Error(w, "Error al codificar los datos como JSON", http.StatusInternalServerError)
		return
	}

	savedPath = request

	log.Printf("Datos recibidos: %+v", request)

	BodyResponse := BodyResponsePid{
		Pid: 0,
	}

	pidResponse, _ := json.Marshal(BodyResponse)

	log.Printf("Se crea el proceso %d ", BodyResponse.Pid)

	w.WriteHeader(http.StatusOK)
	w.Write(pidResponse)
}

func FinalizarProceso(w http.ResponseWriter, r *http.Request) {

	pid := r.PathValue("pid")

	log.Printf("Finaliza el proceso %s - Motivo: <SUCCESS / INVALID_RESOURCE / INVALID_WRITE>", pid)

	respuestaOK := fmt.Sprintf("Proceso finalizado:%s", pid)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(respuestaOK))
}

func EstadoProceso(w http.ResponseWriter, r *http.Request) {
	pid := r.PathValue("pid")

	BodyResponse := BodyResponseState{
		State: "EXIT",
	}

	stateResponse, _ := json.Marshal(BodyResponse)

	log.Printf("PID: %s - Estado Anterior: <ESTADO_ANTERIOR> - Estado Actual: %v", pid, BodyResponse.State) // A checkear

	w.WriteHeader(http.StatusOK)
	w.Write(stateResponse)
}

func IniciarPlanificacion(w http.ResponseWriter, r *http.Request) {
	log.Printf("PID: <PID> - Bloqueado por: <INTERFAZ / NOMBRE_RECURSO>") //ESTO NO VA ACA

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Planificación iniciada"))
}

func DetenerPlanificacion(w http.ResponseWriter, r *http.Request) {
	log.Printf("PID: <PID> - Desalojado por fin de Quantum") //ESTO NO VA ACA
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Planificación detenida"))
}

func ListarProcesos(w http.ResponseWriter, r *http.Request) {
	BodyResponse := []BodyResponseListProcess{
		{0, "EXEC"},
		{1, "READY"},
		{2, "BLOCK"},
		{3, "FIN"},
	}

	arrayProcesos, err := json.Marshal(BodyResponse)
	if err != nil {
		http.Error(w, "Error al codificar los datos como JSON", http.StatusInternalServerError)
		return
	}

	for i := 0; i < len(BodyResponse); i++ {
		fmt.Print(BodyResponse[i], "\n")
	}

	log.Print("Cola Ready <COLA>: [<LISTA DE PIDS>]") //ESTO NO VA ACA

	w.WriteHeader(http.StatusOK)
	w.Write(arrayProcesos)
}

func ConfigurarLogger() {
	logFile, err := os.OpenFile("kernel.log", os.O_CREATE|os.O_APPEND|os.O_RDWR, 0666)
	if err != nil {
		panic(err)
	}
	mw := io.MultiWriter(os.Stdout, logFile)
	log.SetOutput(mw)
}

func LlamarCPU(w http.ResponseWriter, r *http.Request) {

	//pcbResponse := BodyResponsePCB{
	//	Pcb: pcb1,
	//}

	pcbResponseTest, _ := json.Marshal(pcb1)

	w.WriteHeader(http.StatusOK)
	w.Write(pcbResponseTest)
}
