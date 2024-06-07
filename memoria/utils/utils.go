package utils

import (
	"bufio"
	"bytes"
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

type BodyAdress struct {
	Adress int `json:"adress"`
	Length int `json:"length"`
}

type BodyContent struct {
	Content string `json:"content"`
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
var memory []byte

// Tabla de páginas
var pageTable = make(map[int][]int) // Map de pids con pagina asociada, cuya pagina tiene un marco asociado

// Mutex para manejar la concurrencia
var mu sync.Mutex

// Estructura del proceso
type Process struct {
	PID   int `json:"pid"`
	Pages int `json:"pages,omitempty"`
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
		memory = make([]byte, memorySize)
	} else {
		log.Fatal("ClientConfig is not initialized")
	}
}

type BodyRequestInput struct {
	Input   string `json:"input"`
	Address int    `json:"address"`
	Pid     int    `json:"pid"`
}

var adress int
var length int
var IOaddress int
var IOpid int
var IOinput string

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

	fmt.Fprintln(os.Stdout, []any{"%v\n", mapInstructions[pid]}...)
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
		log.Printf("No hay suficiente espacio en memoria")
	}

	if _, exists := pageTable[pid]; exists { //Verifico si ya existe un proceso con ese pid
		log.Printf("Error: PID %d already has pages assigned", pid)
	} else {
		pageTable[pid] = make([]int, pages) // Creo un slice de paginas para el proceso
		println("Proceso creado")
	}

	fmt.Println(pageTable)

	return nil
}

// Pasado un dato de IO a memoria, le asigno la direccion fisica del dato a la pagina del proceso
func AssignAddressToProcess(pid int, address int) error {
	mu.Lock()
	defer mu.Unlock()

	if _, exists := pageTable[pid]; !exists { // Verifico si el proceso existe
		log.Printf("Process not found")
	}

	pageTable[pid] = append(pageTable[pid], address) // Asigno la direccion fisica al proceso\
	fmt.Println(pageTable)
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
		log.Printf("Proceso no encontrado")
	} else {
		delete(pageTable, pid) //Funcion que viene con map, libera los marcos asignados a un pid
		log.Println("Proceso terminado")
	}

	fmt.Println(pageTable)

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
		log.Printf("Proceso no encontrado")
	}

	currentSize := len(pages)
	if newSize > currentSize { //Comparo el tamaño actual con el nuevo tamaño
		if len(memory)/pageSize < newSize-currentSize { //Verifico si hay suficiente espacio en memoria despues de la ampliacion
			log.Printf("Memoria insuficiente para la ampliación")
		} //A REVISAR ESTE IF
		for i := currentSize; i < newSize; i++ { //Asigno nuevos marcos a la ampliacion
			pageTable[pid] = append(pageTable[pid], i) // Convert i to a slice of int
			fmt.Println("Proceso ampliado")
		}
	} else {
		pageTable[pid] = pageTable[pid][:newSize] //Reduce el tamaño del proceso. :newSize es un slice de 0 a newSize (reduce el tope)
		fmt.Println("Proceso reducido")
	}
	fmt.Println(pageTable)
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
		log.Printf("Process not found")
	}

	if address+size > len(memory) { // Verifico si el acceso a memoria esta dentro de los limites del proceso
		log.Printf("Memory access out of bounds")
	}

	return memory[address : address+size], nil //Devuelvo todos los datos (desde la base hasta la base mas el desplazamiento)
}

func WriteMemoryHandler(w http.ResponseWriter, r *http.Request) {
	var memReq MemoryRequest
	if err := json.NewDecoder(r.Body).Decode(&memReq); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := WriteMemory(memReq.Address, memReq.Data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func WriteMemory(address int, data []byte) error {
	mu.Lock()
	defer mu.Unlock()

	if address+len(data) > len(memory) { //Verifico si la direccion de memoria donde quiero escribir esta dentro de los limites de la memoria
		log.Printf("Memory access out of bounds")
	}

	copy(memory[address:], data) // Funcion que viene con map, copia los datos en la direccion de memoria (data)
	return nil
}

func RecieveInputSTDINFromIO(w http.ResponseWriter, r *http.Request) {
	var inputRecieved BodyRequestInput
	err := json.NewDecoder(r.Body).Decode(&inputRecieved)

	if err != nil {
		http.Error(w, "Error decoding JSON data", http.StatusInternalServerError)
		return
	}

	IOinput = inputRecieved.Input
	IOaddress = inputRecieved.Address
	IOpid = inputRecieved.Pid

	_ = WriteMemory(IOaddress, []byte(IOinput)) //Copio en memoria lo recibido por IO
	AssignAddressToProcess(IOpid, IOaddress)    //Asigno la direccion fisica al proceso

	fmt.Println(memory)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Input recibido correctamente"))
}

func RecieveAdressSTDOUTFromIO(w http.ResponseWriter, r *http.Request) {
	var BodyRequestAdress BodyAdress
	err := json.NewDecoder(r.Body).Decode(&BodyRequestAdress)

	if err != nil {
		http.Error(w, "Error decoding JSON data", http.StatusInternalServerError)
		return
	}

	adress = BodyRequestAdress.Adress
	length = BodyRequestAdress.Length
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Adress recibido correctamente"))
}

func SendContentToIO(content string) error {
	var BodyContent BodyContent
	IOurl := "http://localhost:8090/receiveContentFromMemory" //esto está mal, no está el puerto de IO en el config
	BodyContent.Content = content
	ContentResponseTest, err := json.Marshal(BodyContent)
	if err != nil {
		log.Fatalf("Error al serializar el Input: %v", err)
	}

	log.Println("Enviando solicitud con contenido:", ContentResponseTest)

	resp, err := http.Post(IOurl, "application/json", bytes.NewBuffer(ContentResponseTest))
	if err != nil {
		log.Fatalf("Error al enviar la solicitud al módulo de memoria: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Fatalf("Error en la respuesta del módulo de memoria: %v", resp.StatusCode)
	}

	return nil
}
