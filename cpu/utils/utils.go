package utils

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/sisoputnfrba/tp-golang/cpu/globals"
)

type PruebaMensaje struct {
	Mensaje string `json:"Prueba"`
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

func ConfigurarLogger() {
	logFile, err := os.OpenFile("cpu.log", os.O_CREATE|os.O_APPEND|os.O_RDWR, 0666)
	if err != nil {
		panic(err)
	}
	mw := io.MultiWriter(os.Stdout, logFile)
	log.SetOutput(mw)
}

func IniciarConfiguracion(filePath string) *globals.Config {
	var config *globals.Config
	configFile, err := os.Open(filePath)
	if err != nil {
		log.Fatal(err.Error())
	}
	defer configFile.Close()

	jsonParser := json.NewDecoder(configFile)
	jsonParser.Decode(&config)

	return config
}

func Prueba(w http.ResponseWriter, r *http.Request) {

	Prueba := PruebaMensaje{
		Mensaje: "Todo OK CPU",
	}

	pruebaResponse, _ := json.Marshal(Prueba)

	w.WriteHeader(http.StatusOK)
	w.Write(pruebaResponse)
}

func ProcessSavedPCBFromKernel(w http.ResponseWriter, r *http.Request) {
	log.Printf("Recibiendo solicitud de path desde el kernel")
	var sendedPCB PCB
	err := json.NewDecoder(r.Body).Decode(&sendedPCB)
	if err != nil {
		http.Error(w, "Error al decodificar los datos JSON", http.StatusInternalServerError)
		return
	}
	log.Printf("PCB recibido desde el kernel: %+v", sendedPCB)

	// Convertir el PCB recibido de nuevo a JSON para enviarlo en la respuesta
	responseJSON, err := json.Marshal(sendedPCB)
	if err != nil {
		http.Error(w, "Error al serializar el PCB", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(responseJSON)
}
