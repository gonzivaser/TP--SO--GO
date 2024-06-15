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
	Address []int  `json:"address"`
	Length  int    `json:"length"`
	Name    string `json:"name"`
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

/////////////////////////////////////////////////// VARS GLOBALES DE MEMORIA //////////////////////////////////////////////////////////////

var mapInstructions = make(map[int][][]string)

// Tamaño de página y memoria
var pageSize int
var memorySize int

// Mapa de memoria ocupada/libre
var memoryMap []bool // True si la direccion de memoria esta ocupada, False si esta libre

// Espacio de memoria
var memory []byte

// Tabla de páginas
var pageTable = make(map[int][]int) // Map de pids con pagina asociada, cuya pagina tiene un marco asociado

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

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

type BodyFrame struct {
	Frame int `json:"frame"`
}

type BodyRequestPort struct {
	Nombre string `json:"nombre"`
	Port   int    `json:"port"`
}
type interfaz struct {
	Name string
	Port int
}

type BodyRequestInput struct {
	Input   string `json:"input"`
	Address []int  `json:"address"`
	Pid     int    `json:"pid"`
}

type bodyCPUpage struct {
	Pid  int `json:"pid"`
	Page int `json:"page"`
}

type BodyPageTam struct {
	PageTam int `json:"pageTam"`
}

/////////////////////////////////////////////////// VARS GLOBALES ///////////////////////////////////////////////////////////////////////

var addressGLOBAL []int
var lengthGLOBAL int
var IOaddress []int
var IOpid int
var IOinput string
var IOPort int
var CPUpid int
var CPUpage int

var GLOBALnameSTDOUT string
var interfacesGLOBAL []interfaz

/////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

func init() {
	globals.ClientConfig = IniciarConfiguracion("config.json") // tiene que prender la confi cuando arranca

	if globals.ClientConfig != nil {
		pageSize = globals.ClientConfig.PageSize
		memorySize = globals.ClientConfig.MemorySize
		memory = make([]byte, memorySize)
		memoryMap = make([]bool, memorySize)
		SendPageTamToCPU(globals.ClientConfig.PageSize)
		//cantidadFrames := globals.ClientConfig.MemorySize / globals.ClientConfig.PageSize
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
	fmt.Println(memoryMap)

	return nil
}

// Pasado un dato de IO a memoria, le asigno la direccion fisica del dato a la pagina del proceso
func AssignAddressToProcess(pid int, address int) error {
	mu.Lock()
	defer mu.Unlock()

	if _, exists := pageTable[pid]; !exists { // Verifico si el proceso existe
		log.Printf("Process not found")
	}

	if contains(pageTable[pid], address) { // Verifico si la direccion ya fue asignada
		log.Printf("Address already assigned")
	} else {
		memoryMap[address] = true
	}
	fmt.Println(pageTable)
	fmt.Println(memoryMap)
	return nil
}

func contains(slice []int, element int) bool {
	for _, a := range slice {
		if a == element {
			return true
		}
	}
	return false
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
		if addresses, exists := pageTable[pid]; exists {
			for _, address := range addresses {
				memoryMap[address] = false //Marca las addresses del pid como libres
			}
		}
		delete(pageTable, pid) //Funcion que viene con map, libera los marcos asignados a un pid
		log.Println("Proceso terminado")
	}
	fmt.Println(pageTable)
	fmt.Println(memoryMap)

	return nil
}

func ResizeProcessHandler(w http.ResponseWriter, r *http.Request) {
	var process Process
	if err := json.NewDecoder(r.Body).Decode(&process); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	Pid := process.PID
	Pages := process.Pages

	err := ResizeProcess(Pid, Pages)
	if err != nil {
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

	if newSize%pageSize != 0 { //Verifico si el nuevo tamaño es multiplo del tamaño de pagina
		newSize = newSize + pageSize - (newSize % pageSize) //Si no es multiplo, lo redondeo al proximo multiplo
	} else {

	}
	currentSize := len(pages)
	if newSize > currentSize { //Comparo el tamaño actual con el nuevo tamaño
		if len(memory)/pageSize < newSize-currentSize { //Verifico si hay suficiente espacio en memoria despues de la ampliacion
			log.Printf("Memoria insuficiente para la ampliación")
		}
		for i := currentSize; i < newSize/pageSize; i++ { //Asigno nuevos marcos a la ampliacion
			proxLugarLibre := proximosXLugaresLibres(globals.ClientConfig.PageSize)
			if proxLugarLibre != -1 {
				pageTable[pid] = append(pageTable[pid], proxLugarLibre)
				memoryMap[proxLugarLibre] = true
				fmt.Println("Proceso ampliado")
			} else {
				log.Printf("No more free spots in memory")
				break
			}
		}
	} else {
		for i := newSize; i < len(pageTable[pid]); i++ {
			memoryMap[pageTable[pid][i]] = false
		}
		pageTable[pid] = pageTable[pid][:newSize] //Reduce el tamaño del proceso. :newSize es un slice de 0 a newSize (reduce el tope)
		fmt.Println("Proceso reducido")
	}
	fmt.Println(pageTable)
	fmt.Println(memoryMap)
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

	verificarTopeDeMemoria(IOpid, address)

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

	var IOinputMemoria []byte = []byte(IOinput)

	for i := 0; i < len(IOaddress) && len(IOinputMemoria) < globals.ClientConfig.PageSize; i++ {
		_ = WriteMemory(IOaddress[i], IOinputMemoria)
		AssignAddressToProcess(IOpid, IOaddress[i])
	}

	//16*i + 16*(i+1)
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

	addressGLOBAL = BodyRequestAdress.Address
	lengthGLOBAL = BodyRequestAdress.Length
	GLOBALnameSTDOUT = BodyRequestAdress.Name

	var data []byte
	for i := 0; i < len(addressGLOBAL); i++ {
		data, err = ReadMemory(IOpid, addressGLOBAL[i], lengthGLOBAL)
		data = append(data, []byte("\n")...)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	// data := memory[address : address+lengthGLOBAL]
	SendContentToIO(string(data))

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("address recibido correctamente"))
}

func RecievePortOfInterfaceFromKernel(w http.ResponseWriter, r *http.Request) {
	var requestPort BodyRequestPort
	var interfaz interfaz
	err := json.NewDecoder(r.Body).Decode(&requestPort)
	if err != nil {
		http.Error(w, "Error decoding JSON data", http.StatusInternalServerError)
		return
	}
	interfaz.Name = requestPort.Nombre
	interfaz.Port = requestPort.Port

	interfacesGLOBAL = append(interfacesGLOBAL, interfaz)
	log.Printf("Received data: %+v", interfacesGLOBAL)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf("Port received: %d", requestPort.Port)))
}

func SendContentToIO(content string) error {
	var interfazEncontrada interfaz // Asume que Interfaz es el tipo de tus interfaces
	interfazEncontrada.Name = GLOBALnameSTDOUT

	for _, interfaz := range interfacesGLOBAL {
		if interfaz.Name == GLOBALnameSTDOUT {
			interfazEncontrada = interfaz
			break
		}
	}
	var BodyContent BodyContent
	BodyContent.Content = content
	IOurl := fmt.Sprintf("http://localhost:%d/receiveContentFromMemory", interfazEncontrada.Port)
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

func GetPageFromCPU(w http.ResponseWriter, r *http.Request) {
	var bodyCPUpage1 bodyCPUpage
	err := json.NewDecoder(r.Body).Decode(&bodyCPUpage1)
	if err != nil {
		http.Error(w, "Error decoding JSON data", http.StatusInternalServerError)
		return
	}
	CPUpid = bodyCPUpage1.Pid
	CPUpage = bodyCPUpage1.Page

	sendFrameToCPU(CPUpid, CPUpage)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Page recibido correctamente"))
}

func sendFrameToCPU(pid int, page int) error {
	var bodyFrame BodyFrame
	CPUurl := fmt.Sprintf("http://localhost:%d/recieveFrame", globals.ClientConfig.PuertoCPU)
	frame := pageTable[pid][page-1]
	bodyFrame.Frame = frame
	FrameResponseTest, err := json.Marshal(bodyFrame)
	if err != nil {
		log.Fatalf("Error al serializar el frame: %v", err)
	}
	log.Println("Enviando solicitud con contenido:", FrameResponseTest)

	resp, err := http.Post(CPUurl, "application/json", nil)
	if err != nil {
		log.Fatalf("error al enviar la solicitud al módulo de memoria: %v", err)
	}
	defer resp.Body.Close()
	return nil
}

func proximosXLugaresLibres(x int) int {
	contadorLugares := 0
	for i, libre := range memoryMap {
		if !libre {
			contadorLugares++
			if contadorLugares == x {
				return i - x + 1 // Devuelvo el indicie del primer lugar libre
			}
		} else {
			contadorLugares = 0 // Reinicio el contador si encuentro un lugar ocupado
		}
	}
	return -1 // Si no hay espacios libres, devuelvo -1
}

func verificarTopeDeMemoria(pid int, address int) {
	contador := 0
	for i := 0; i < len(pageTable[pid]); i++ {
		if pageTable[pid][i]*globals.ClientConfig.PageSize <= address && address <= ((i+1)*pageSize)-1 {
			log.Printf("Proceso dentro de espacio memoria")
			contador++
		}
	}
	if contador == 0 {
		log.Printf("Proceso fuera de espacio memoria")
	}
}

func SendPageTamToCPU(tamPage int) {
	CPUurl := fmt.Sprintf("http://localhost:%d/recievePageTam", globals.ClientConfig.PuertoCPU)
	var body BodyPageTam
	body.PageTam = tamPage
	PageTamResponseTest, err := json.Marshal(body)
	if err != nil {
		log.Fatalf("Error al serializar el tamPage: %v", err)
	}
	log.Println("Enviando solicitud con contenido:", PageTamResponseTest)

	resp, err := http.Post(CPUurl, "application/json", bytes.NewBuffer(PageTamResponseTest))
	if err != nil {
		log.Fatalf("error al enviar la solicitud al módulo de memoria: %v", err)
	}
	defer resp.Body.Close()
}
