package utils

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/sisoputnfrba/tp-golang/memoria/globals"
)

type BodyRequest struct {
	Path string `json:"path"`
}

type InstructionResposne struct {
	Instruction string `json:"instruction"`
}

type PCB struct {
	// programCounter, Quantum int
	Pid    int
	CpuReg RegisterCPU
}

type RegisterCPU struct {
	PC, EAX, EBX, ECX, EDX, SI, DI uint32
	AX, BX, CX, DX                 uint8
}

type BodyRequestInput struct {
	Input string `json:"input"`
}

var m = make(map[int][][]string)

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

func ConfigurarLogger() {
	logFile, err := os.OpenFile("memoria.log", os.O_CREATE|os.O_APPEND|os.O_RDWR, 0666)
	if err != nil {
		panic(err)
	}
	mw := io.MultiWriter(os.Stdout, logFile)
	log.SetOutput(mw)
}

func SetInstructionsFromFileToMap(w http.ResponseWriter, r *http.Request) {
	// m[pcb.Pid] = readInstructions(path pcb.programCounter)
	queryParams1 := r.URL.Query()
	pid, _ := strconv.Atoi(queryParams1.Get("pid"))
	queryParams2 := r.URL.Query()
	path := queryParams2.Get("path")

	readFile, _ := os.Open(path)
	// Ensure file is closed even on errors

	// Create a new scanner for line-by-line reading
	fileScanner := bufio.NewScanner(readFile)
	fileScanner.Split(bufio.ScanLines)

	var arrInstructions [][]string
	for fileScanner.Scan() {
		//Esta linea lee los codigos
		arrInstructions = append(arrInstructions, []string{fileScanner.Text()})
	}
	m[pid] = arrInstructions

	fmt.Printf("%v\n", m[pid])
	fmt.Println(m)
	defer readFile.Close()

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Instructions loaded successfully"))
}

func GetInstruction(w http.ResponseWriter, r *http.Request) {
	queryParams := r.URL.Query()
	pid, _ := strconv.Atoi(queryParams.Get("pid"))
	programCounter, _ := strconv.Atoi(queryParams.Get("programCounter"))
	instruction := m[pid][programCounter][0]

	instructionResponse := InstructionResposne{
		Instruction: instruction,
	}
	fmt.Printf("Esto es la instruction %+v\n", instructionResponse)

	json.NewEncoder(w).Encode(instructionResponse)

	w.Write([]byte(instruction))
}

func RecieveInputSTDINFromIO(w http.ResponseWriter, r *http.Request) {
	var inputRecieved BodyRequestInput
	err := json.NewDecoder(r.Body).Decode(&inputRecieved)

	if err != nil {
		http.Error(w, "Error decoding JSON data", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Input recibido correctamente"))
}

func RecieveAdressSTDOUTFromIO(w http.ResponseWriter, r *http.Request) {
	var body BodyRequest
	err := json.NewDecoder(r.Body).Decode(&body)

	if err != nil {
		http.Error(w, "Error decoding JSON data", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Adress recibido correctamente"))
}
