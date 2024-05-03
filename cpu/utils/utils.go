package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
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

type ExecutionContext struct {
	Pid            int
	ProgramCounter int
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

var programCounter int

func ProcessSavedPCBFromKernel(w http.ResponseWriter, r *http.Request) {
	// HAGO UN LOG PARA CHEQUEAR RECEPCION
	log.Printf("Recibiendo solicitud de contexto de ejecucuion desde el kernel")

	// GUARDO PCB RECIBIDO EN sendPCB
	var sendPCB ExecutionContext
	err := json.NewDecoder(r.Body).Decode(&sendPCB)
	if err != nil {
		http.Error(w, "Error al decodificar los datos JSON", http.StatusInternalServerError)
		return
	}

	// HAGO UN LOG PARA CHEQUEAR QUE PASO ERRORES
	log.Printf("PCB recibido desde el kernel: %+v", sendPCB)

	// MANDO EL PC DIRECTAMENTE A MEMORIA
	programCounter = int(sendPCB.CpuReg.PC)
	if err := SendPCToMemoria(programCounter); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Convertir el pid PCB recibido de nuevo a JSON para enviarlo en la respuesta
	responseJSON, err := json.Marshal(sendPCB.Pid)
	if err != nil {
		http.Error(w, "Error al serializar el PCB", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(responseJSON)
}

func SendPCToMemoria(pc int) error {
	memoriaURL := "http://localhost:8085/savePC"

	// CREO VARIABLE DONDE GUARDO EL PROGRAM COUNTER
	pcProcess, err := json.Marshal(pc)

	// CHEQUEO ERRORES
	if err != nil {
		return fmt.Errorf("error al serializar el PCB: %v", err)
	}

	// CONFIRMACION DE QUE PASO ERRORES Y SE MANDA SOLICITUD
	log.Println("Enviando Program Counter con valor:", string(pcProcess))

	// CREO VARIABLE resp y err CON EL
	resp, err := http.Post(memoriaURL, "application/json", bytes.NewBuffer(pcProcess))
	if err != nil {
		return fmt.Errorf("error al enviar la solicitud al módulo de memoria: %v", err)
	}
	defer resp.Body.Close()

	// CHEQUEO STATUS CODE CON MI VARIABLE resp
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("error en la respuesta del módulo de memoria: %v", resp.StatusCode)
	}

	//llamada a interpretacion de instruccion
	programCounter += 1

	// SE CHEQUEA CON UN PRINT QUE LA LA MEMORIA RECIBIO CORRECTAMENTE EL pc
	log.Println("Respuesta del módulo de memoria recibida correctamente.")
	return nil
}
