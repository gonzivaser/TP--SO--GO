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
	"sync"

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

var mapInstructions = make(map[int][][]string)

// Tamaño de página y memoria
var pageSize int
var memorySize int

// Espacio de memoria
var memory = make([]byte, memorySize)

// Tabla de páginas
var pageTable = make(map[int][]int)

// Mutex para manejar la concurrencia
var mu sync.Mutex

// Estructura del proceso
type Process struct {
	PID   int `json:"pid"`
	Pages int `json:"pages"`
}

// Estructura de la solicitud de lectura/escritura
type MemoryRequest struct {
	PID     int    `json:"pid"`
	Address int    `json:"address"`
	Size    int    `json:"size,omitempty"` //Si es 0, se omite (Util para creacion y terminacion de procesos)
	Data    []byte `json:"data,omitempty"` //Si es 0, se omite Util para creacion y terminacion de procesos)
}

func init() {
	globals.ClientConfig = IniciarConfiguracion("config.json") // tiene que prender la confi cuando arranca

	if globals.ClientConfig != nil {
		pageSize = globals.ClientConfig.PageSize
		memorySize = globals.ClientConfig.MemorySize
	} else {
		log.Fatal("ClientConfig is not initialized")
	}
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
	mapInstructions[pid] = arrInstructions

	fmt.Printf("%v\n", mapInstructions[pid])
	fmt.Println(mapInstructions)
	defer readFile.Close()

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Instructions loaded successfully"))
}

func GetInstruction(w http.ResponseWriter, r *http.Request) {
	queryParams := r.URL.Query()
	pid, _ := strconv.Atoi(queryParams.Get("pid"))
	programCounter, _ := strconv.Atoi(queryParams.Get("programCounter"))
	instruction := mapInstructions[pid][programCounter][0]

	instructionResponse := InstructionResposne{
		Instruction: instruction,
	}
	fmt.Printf("Esto es la instruction %+v\n", instructionResponse)

	json.NewEncoder(w).Encode(instructionResponse)

	w.Write([]byte(instruction))
}

// COMUNICACION
// Creacion de procesos
func CreateProcessHandler(w http.ResponseWriter, r *http.Request) {
	var process Process
	if err := json.NewDecoder(r.Body).Decode(&process); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := CreateProcess(process.PID, process.Pages); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

func CreateProcess(pid int, pages int) error {
	mu.Lock()
	defer mu.Unlock()

	if len(memory)/pageSize < pages { // Verifico si hay suficiente espacio en memoria en base a las paginas solicitadas
		println("No hay suficiente espacio en memoria")
	}

	pageTable[pid] = make([]int, pages)
	for i := 0; i < pages; i++ {
		pageTable[pid][i] = i // Asigno marcos/frames contiguos
	}

	return nil
}

func TerminateProcessHandler(w http.ResponseWriter, r *http.Request) {
	var process Process
	if err := json.NewDecoder(r.Body).Decode(&process); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := TerminateProcess(process.PID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func TerminateProcess(pid int) error {
	mu.Lock()
	defer mu.Unlock()

	if _, exists := pageTable[pid]; !exists {
		println("Proceso no encontrado")
	}

	delete(pageTable, pid) //Funcion que viene con map, libera los marcos asignados a un pid
	return nil
}

func ResizeProcessHandler(w http.ResponseWriter, r *http.Request) {
	var process Process
	if err := json.NewDecoder(r.Body).Decode(&process); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := ResizeProcess(process.PID, process.Pages); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func ResizeProcess(pid int, newSize int) error {
	mu.Lock()
	defer mu.Unlock()

	pages, exists := pageTable[pid]
	if !exists { // Verifico si el proceso existe
		println("Proceso no encontrado")
	}

	currentSize := len(pages)
	if newSize > currentSize { //Comparo el tamaño actual con el nuevo tamaño
		if len(memory)/pageSize < newSize-currentSize { //Verifico si hay suficiente espacio en memoria despues de la ampliacion
			println("Memoria insuficiente para la ampliación")
		} //A REVISAR ESTE IF
		for i := currentSize; i < newSize; i++ { //Asigno nuevos marcos a la ampliacion
			pageTable[pid] = append(pageTable[pid], i)
		}
	} else {
		pageTable[pid] = pageTable[pid][:newSize] //Reduce el tamaño del proceso. :newSize es un slice de 0 a newSize (reduce el tope)
	}

	return nil
}

func ReadMemoryHandler(w http.ResponseWriter, r *http.Request) {
	var memReq MemoryRequest
	if err := json.NewDecoder(r.Body).Decode(&memReq); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	data, err := ReadMemory(memReq.PID, memReq.Address, memReq.Size)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Write(data)
}

func ReadMemory(pid int, address int, size int) ([]byte, error) {
	mu.Lock()
	defer mu.Unlock()

	if _, exists := pageTable[pid]; !exists { // Verifico si el proceso existe
		println("Process not found")
	}

	if address+size > len(memory) { // Verifico si el acceso a memoria esta dentro de los limites del proceso
		println("Memory access out of bounds")
	}

	return memory[address : address+size], nil //Devuelvo todos los datos (desde la base hasta la base mas el desplazamiento)
}

func WriteMemoryHandler(w http.ResponseWriter, r *http.Request) {
	var memReq MemoryRequest
	if err := json.NewDecoder(r.Body).Decode(&memReq); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := WriteMemory(memReq.PID, memReq.Address, memReq.Data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func WriteMemory(pid int, address int, data []byte) error {
	mu.Lock()
	defer mu.Unlock()

	if _, exists := pageTable[pid]; !exists { // Verifico si el proceso existe
		println("Process not found")
	}

	if address+len(data) > len(memory) { //Verifico si la direccion de memoria donde quiero escribir esta dentro de los limites de la memoria
		println("Memory access out of bounds")
	}

	copy(memory[address:], data) // Funcion que viene con map, copia los datos en la direccion de memoria (data)
	return nil
}
