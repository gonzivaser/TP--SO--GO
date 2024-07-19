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
	"time"

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
var memoryMap []bool // True si el FRAME esta ocupado, False si esta libre

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
	Address []int  `json:"address"`
	Size    int    `json:"size,omitempty"` //Si es 0, se omite (Util para creacion y terminacion de procesos)
	Data    []byte `json:"data,omitempty"` //Si es 0, se omite Util para creacion y terminacion de procesos)
	Type    string `json:"type"`
	Port    int    `json:"port,omitempty"`
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

var GLOBALnameIO string
var interfacesGLOBAL []interfaz

/////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

func init() {
	globals.ClientConfig = IniciarConfiguracion("config.json") // tiene que prender la confi cuando arranca

	if globals.ClientConfig != nil {
		pageSize = globals.ClientConfig.PageSize
		memorySize = globals.ClientConfig.MemorySize
		memory = make([]byte, memorySize) //intocable
		memoryMap = make([]bool, memorySize/globals.ClientConfig.PageSize)
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
	time.Sleep(time.Duration(globals.ClientConfig.DelayResponse) * time.Millisecond)
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

	log.Printf("PID: %d - Tamaño: %d", pid, pages)
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
	time.Sleep(time.Duration(globals.ClientConfig.DelayResponse) * time.Millisecond)
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
	/*mu.Lock()
	defer mu.Unlock()*/

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
	time.Sleep(time.Duration(globals.ClientConfig.DelayResponse) * time.Millisecond)
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
	}
	currentSize := len(pages)
	if newSize > currentSize { //Comparo el tamaño actual con el nuevo tamaño
		freespace := counterMemoryFree()
		if freespace < (newSize/pageSize)-currentSize { //Verifico si hay suficiente espacio en memoria despues de la ampliacion
			log.Printf("Out of Memory")
			FinalizarProceso(pid)
			var err1 error
			return err1
		}
		for i := currentSize; i < newSize/pageSize; i++ { //Asigno nuevos marcos a la ampliacion
			indiceLibre := proximoLugarLibre()
			if indiceLibre != -1 {

				pageTable[pid] = append(pageTable[pid], indiceLibre)
				memoryMap[indiceLibre] = true
				//fmt.Println("Proceso ampliado")
				log.Printf("PID: %d - Tamaño Actual: %d - Tamaño a Ampliar: %d", pid, currentSize, newSize)
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
		//fmt.Println("Proceso reducido")
		log.Printf("PID: %d - Tamaño Actual: %d - Tamaño a Reducir: %d", pid, currentSize, newSize)
	}
	fmt.Println(pageTable)
	fmt.Println(memoryMap)
	return nil
}

func counterMemoryFree() int {
	var contador int
	for i := 0; i < len(memoryMap); i++ {
		if !memoryMap[i] {
			contador++
		}
	}
	return contador
}

func ReadMemoryHandler(w http.ResponseWriter, r *http.Request) {
	var memReq MemoryRequest
	time.Sleep(time.Duration(globals.ClientConfig.DelayResponse) * time.Millisecond)
	if err := json.NewDecoder(r.Body).Decode(&memReq); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	data, err := ReadMemory(memReq.PID, memReq.Address, memReq.Size)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if memReq.Type == "CPU" {
		sendDataToCPU(data)
	} else if memReq.Type == "IO" {
		SendContentToIO(string(data), memReq.Port)
	}
	w.Write(data)
}

/*func ReadMemory(pid int, addresses []int, size int) ([]byte, error) {
	mu.Lock()
	defer mu.Unlock()

	if _, exists := pageTable[pid]; !exists {
		return nil, fmt.Errorf("Process with PID %d not found", pid)
	}

	var result []byte
	remainingSize := size

	for _, address := range addresses {
		if address < 0 || address >= len(memory) {
			return nil, fmt.Errorf("memory access out of bounds at address %d", address)
		}

		// Calculate how much we can read from this address
		readSize := min(remainingSize, pageSize-(address%pageSize))

		// Read the data
		result = append(result, memory[address:address+readSize]...)

		remainingSize -= readSize
		if remainingSize <= 0 {
			break
		}
	}

	if len(result) < size {
		return nil, fmt.Errorf("unable to read %d bytes, only %d bytes available", size, len(result))
	}

	return result[:size], nil
}*/

func ReadMemory(pid int, addresses []int, size int) ([]byte, error) {
	mu.Lock()
	defer mu.Unlock()

	if _, exists := pageTable[pid]; !exists {
		return nil, fmt.Errorf("Process with PID %d not found", pid)
	}

	var result []byte
	for _, address := range addresses {
		if address < 0 || address >= len(memory) {
			return nil, fmt.Errorf("memory access out of bounds at address %d", address)
		}
		result = append(result, memory[address])
	}
	return result, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// address : address+size
func sendDataToCPU(content []byte) error {
	CPUurl := fmt.Sprintf("http://localhost:%d/receiveDataFromMemory", globals.ClientConfig.PuertoCPU)
	ContentResponseTest, err := json.Marshal(content)
	if err != nil {
		log.Fatalf("Error al serializar el Input: %v", err)
	}

	log.Println("Enviando solicitud con contenido:", ContentResponseTest)

	resp, err := http.Post(CPUurl, "application/json", bytes.NewBuffer(ContentResponseTest))
	if err != nil {
		log.Fatalf("Error al enviar la solicitud al módulo de memoria: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Fatalf("Error en la respuesta del módulo de memoria: %v", resp.StatusCode)
	}

	return nil
}

func WriteMemoryHandler(w http.ResponseWriter, r *http.Request) {
	var memReq MemoryRequest
	time.Sleep(time.Duration(globals.ClientConfig.DelayResponse) * time.Millisecond)
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

/*func WriteMemory(pid int, addresses []int, data []byte) error {
	mu.Lock()
	defer mu.Unlock()

	pages, exists := pageTable[pid]
	if !exists {
		return fmt.Errorf("Process with PID %d not found", pid)
	}

	dataIndex := 0
	for _, address := range addresses {
		// Verificar que la dirección pertenece al proceso
		pageIndex := address / pageSize
		if pageIndex >= len(pages) {
			return fmt.Errorf("address %d does not belong to process %d", address, pid)
		}

		frame := pages[pageIndex]
		offsetInFrame := address % pageSize

		// Calcular cuánto podemos escribir en este frame
		bytesToWrite := min(len(data)-dataIndex, pageSize-offsetInFrame)

		// Verificar que no excedemos el límite de la memoria
		if frame*pageSize+offsetInFrame+bytesToWrite > len(memory) {
			return fmt.Errorf("memory access out of bounds")
		}

		// Escribir los datos
		copy(memory[frame*pageSize+offsetInFrame:], data[dataIndex:dataIndex+bytesToWrite])

		dataIndex += bytesToWrite

		if dataIndex >= len(data) {
			break
		}
	}

	if dataIndex < len(data) {
		return fmt.Errorf("not all data could be written. Only %d out of %d bytes written", dataIndex, len(data))
	}
	fmt.Println(memory)
	return nil
}*/

func WriteMemory(pid int, addresses []int, data []byte) error {
	mu.Lock()
	defer mu.Unlock()

	if _, exists := pageTable[pid]; !exists {
		return fmt.Errorf("Process with PID %d not found", pid)
	}
	i := 0
	if len(data) >= len(addresses) {
		for _, address := range addresses {
			memory[address] = data[i]
			i++
		}
	} else {
		for _, dato := range data {
			memory[addresses[i]] = dato
			i++
		}
	}
	fmt.Println(memory)
	return nil
}

// STDIN, FSREAD
func RecieveInputFromIO(w http.ResponseWriter, r *http.Request) {
	var inputRecieved BodyRequestInput
	time.Sleep(time.Duration(globals.ClientConfig.DelayResponse) * time.Millisecond)
	err := json.NewDecoder(r.Body).Decode(&inputRecieved)

	if err != nil {
		http.Error(w, "Error decoding JSON data", http.StatusInternalServerError)
		return
	}

	IOinput = inputRecieved.Input
	IOaddress = inputRecieved.Address
	IOpid = inputRecieved.Pid

	log.Printf("Received data: %+v", IOpid)

	var IOinputMemoria []byte = []byte(IOinput)

	err2 := WriteMemory(IOpid, IOaddress, IOinputMemoria)
	if err2 != nil {
		http.Error(w, err2.Error(), http.StatusInternalServerError)
		return
	}

	for i := 0; i < len(IOaddress) && len(IOinputMemoria) < globals.ClientConfig.PageSize; i++ {
		AssignAddressToProcess(IOpid, IOaddress[i])
	}

	//16*i + 16*(i+1)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Input recibido correctamente"))
}

// STDOUT, FSWRITE
/*func RecieveAdressFromIO(w http.ResponseWriter, r *http.Request) {
	var BodyRequestAdress BodyAdress
	time.Sleep(time.Duration(globals.ClientConfig.DelayResponse) * time.Millisecond)
	err := json.NewDecoder(r.Body).Decode(&BodyRequestAdress)

	if err != nil {
		http.Error(w, "Error decoding JSON data", http.StatusInternalServerError)
		return
	}

	addressGLOBAL = BodyRequestAdress.Address
	lengthGLOBAL = BodyRequestAdress.Length
	GLOBALnameIO = BodyRequestAdress.Name

	data, err1 := ReadMemory(IOpid, addressGLOBAL, lengthGLOBAL)
	if err1 != nil {
		http.Error(w, err1.Error(), http.StatusInternalServerError)
		return
	}

	// data := memory[address : address+lengthGLOBAL]
	SendContentToIO(string(data))

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("address recibido correctamente"))
}*/

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

func SendContentToIO(content string, Puerto int) error {
	var BodyContent BodyContent
	BodyContent.Content = content
	IOurl := fmt.Sprintf("http://localhost:%d/receiveContentFromMemory", Puerto)
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
	frame := pageTable[pid][page]
	log.Printf("PID: %d - Pagina: %d - Marco: %d", pid, page, frame)
	bodyFrame.Frame = frame
	FrameResponseTest, err := json.Marshal(bodyFrame)
	if err != nil {
		log.Fatalf("Error al serializar el frame: %v", err)
	}
	log.Println("Enviando solicitud con contenido:", FrameResponseTest)

	resp, err := http.Post(CPUurl, "application/json", bytes.NewBuffer(FrameResponseTest))
	if err != nil {
		log.Fatalf("error al enviar la solicitud al módulo de memoria: %v", err)
	}
	defer resp.Body.Close()
	return nil
}

func proximoLugarLibre() int {
	for i := 0; i < len(memoryMap); i++ {
		if !memoryMap[i] {
			return i
		}
	}
	return -1
}

/*func verificarTopeDeMemoria(pid int, address int) {
	contador := 0
	for i := 0; i < len(pageTable[pid]); i++ {
		if pageTable[pid][i]*globals.ClientConfig.PageSize <= address && address <= ((pageTable[pid][i]+1)*globals.ClientConfig.PageSize)-1 {
			log.Printf("Proceso dentro de espacio memoria")
			contador++
		}
	}
	if contador == 0 {
		log.Printf("Proceso fuera de espacio memoria")
	}
}*/

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

func FinalizarProceso(pid int) {
	kernelURL := fmt.Sprintf("http://localhost:%d/process?pid=%d", globals.ClientConfig.PuertoKernel, pid)
	req, err := http.NewRequest("DELETE", kernelURL, nil)
	if err != nil {
		log.Fatalf("Error al crear la solicitud: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatalf("Error al enviar la solicitud al módulo de kernel: %v", err)
	}
	defer resp.Body.Close()
}
