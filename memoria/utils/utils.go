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

type bodyCPUpage struct {
	Pid  int `json:"pid"`
	Page int `json:"page"`
}

type BodyPageTam struct {
	PageTam int `json:"pageTam"`
}

/////////////////////////////////////////////////// VARS GLOBALES DE MEMORIA //////////////////////////////////////////////////////////////

var (
	global_instructionsMap = make(map[int][][]string)

	// Tamaño de página y memoria
	pageSize int

	//memorySize int // Dudoso si es necesario

	// Mapa de memoria ocupada/libre
	global_memory []bool // True si el FRAME esta ocupado, False si esta libre

	// Espacio de memoria
	memory []byte

	// Tabla de páginas
	global_pageTable = make(map[int][]int) // Map de pids con pagina asociada, cuya pagina tiene un marco asociado

	// Mutex para manejar la concurrencia
	mutexMemory sync.Mutex

	// No deberian ser globales
	//CPUpid  int
	//CPUpage int
)

func init() {
	globals.ClientConfig = IniciarConfiguracion(os.Args[1]) // tiene que prender la confi cuando arranca

	if globals.ClientConfig != nil {
		pageSize = globals.ClientConfig.PageSize
		memorySize := globals.ClientConfig.MemorySize
		memory = make([]byte, memorySize) //intocable
		global_memory = make([]bool, memorySize/globals.ClientConfig.PageSize)
		sendPageTamToCPU(globals.ClientConfig.PageSize)
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

/*------------------------------Instrucciones---------------------------------*/

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
	global_instructionsMap[pid] = arrInstructions

	defer readFile.Close()

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Instructions loaded successfully"))
}

func GetInstructionFromCPU(w http.ResponseWriter, r *http.Request) {
	queryParams := r.URL.Query()
	pid, _ := strconv.Atoi(queryParams.Get("pid"))
	programCounter, _ := strconv.Atoi(queryParams.Get("programCounter"))
	instruction := global_instructionsMap[pid][programCounter][0]

	time.Sleep(time.Duration(globals.ClientConfig.DelayResponse) * time.Millisecond)

	instructionResponse := InstructionResposne{
		Instruction: instruction,
	}

	json.NewEncoder(w).Encode(instructionResponse)

	w.Write([]byte(instruction))
}

/*------------------------------Creación de proceso---------------------------------*/

func CreateProcessHandler(w http.ResponseWriter, r *http.Request) {
	var process Process
	time.Sleep(time.Duration(globals.ClientConfig.DelayResponse) * time.Millisecond)
	if err := json.NewDecoder(r.Body).Decode(&process); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := createProcess(process.PID, process.Pages); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

func createProcess(pid int, pages int) error {
	mutexMemory.Lock()
	defer mutexMemory.Unlock()

	if len(memory)/pageSize < pages { // Verifico si hay suficiente espacio en memoria en base a las paginas solicitadas
		finishProcessToKernel(pid)
	}

	if _, exists := global_pageTable[pid]; exists { //Verifico si ya existe un proceso con ese pid
		log.Printf("Error: PID %d already has pages assigned", pid)
	} else {
		global_pageTable[pid] = make([]int, pages) // Creo un slice de paginas para el proceso
	}

	log.Printf("PID: %d - Tamaño: %d", pid, pages)

	return nil
}

/*------------------------------Finalización de proceso---------------------------------*/

func TerminateProcessHandler(w http.ResponseWriter, r *http.Request) {
	pidStr := r.URL.Query().Get("pid")
	if pidStr == "" {
		http.Error(w, "PID no especificado", http.StatusBadRequest)
		return
	}
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		http.Error(w, "PID debe ser un número", http.StatusBadRequest)
		return
	}

	if err := terminateProcess(pid); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func terminateProcess(pid int) error {
	/*mu.Lock()
	defer mu.Unlock()*/

	if _, exists := global_pageTable[pid]; !exists {
		log.Printf("Proceso no encontrado")
		return nil
	} else {
		if addresses, exists := global_pageTable[pid]; exists {
			for _, address := range addresses {
				global_memory[address] = false //Marca las addresses del pid como libres
			}
		}
		log.Printf("PID: %d - Tamaño: %d", pid, len(global_pageTable[pid]))
		delete(global_pageTable, pid) //Funcion que viene con map, libera los marcos asignados a un pid
	}
	return nil
}

func finishProcessToKernel(pid int) {
	kernelURL := fmt.Sprintf("http://%s:%d/process?pid=%d&motivo=OUT_OF_MEMORY", globals.ClientConfig.IpKernel, globals.ClientConfig.PuertoKernel, pid)
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

/*------------------------------Resize---------------------------------*/

func ResizeProcessHandler(w http.ResponseWriter, r *http.Request) {
	var process Process
	time.Sleep(time.Duration(globals.ClientConfig.DelayResponse) * time.Millisecond)
	if err := json.NewDecoder(r.Body).Decode(&process); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	Pid := process.PID
	Pages := process.Pages

	err := resizeProcess(Pid, Pages)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func resizeProcess(pid int, newSize int) error {
	mutexMemory.Lock()
	defer mutexMemory.Unlock()

	pages, exists := global_pageTable[pid]
	if !exists { // Verifico si el proceso existe
		log.Printf("Proceso no encontrado")
	}

	if newSize%pageSize != 0 { //Verifico si el nuevo tamaño es multiplo del tamaño de pagina
		newSize = newSize + pageSize - (newSize % pageSize) //Si no es multiplo, lo redondeo al proximo multiplo
	}
	currentSize := len(pages)
	if newSize/pageSize > currentSize { //Comparo el tamaño actual con el nuevo tamaño
		log.Printf("PID: %d - Tamaño Actual: %d - Tamaño a Ampliar: %d", pid, currentSize, newSize)
		freespace := counterMemoryFree()
		if freespace < (newSize/pageSize)-currentSize { //Verifico si hay suficiente espacio en memoria despues de la ampliacion
			finishProcessToKernel(pid)
			return nil
		}

		for i := currentSize; i < newSize/pageSize; i++ { //Asigno nuevos marcos a la ampliacion
			indiceLibre := nextFreeSpace()
			if indiceLibre != -1 {

				global_pageTable[pid] = append(global_pageTable[pid], indiceLibre)
				global_memory[indiceLibre] = true

			} else {
				break
			}
		}
	} else {
		for i := newSize / pageSize; i < len(global_pageTable[pid]); i++ {
			global_memory[global_pageTable[pid][i]] = false
		}
		global_pageTable[pid] = global_pageTable[pid][:newSize/pageSize] //Reduce el tamaño del proceso. :newSize es un slice de 0 a newSize (reduce el tope)
		//fmt.Println("Proceso reducido")
		log.Printf("PID: %d - Tamaño Actual: %d - Tamaño a Reducir: %d", pid, currentSize, newSize)
	}
	return nil
}

func counterMemoryFree() int {
	var contador int
	for i := 0; i < len(global_memory); i++ {
		if !global_memory[i] {
			contador++
		}
	}
	return contador
}

func nextFreeSpace() int {
	for i := 0; i < len(global_memory); i++ {
		if !global_memory[i] {
			return i
		}
	}
	return -1
}

/*------------------------------STDOUT---------------------------------*/

func ReadMemoryHandler(w http.ResponseWriter, r *http.Request) {
	var memReq MemoryRequest
	time.Sleep(time.Duration(globals.ClientConfig.DelayResponse) * time.Millisecond)
	if err := json.NewDecoder(r.Body).Decode(&memReq); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	data, err := readMemory(memReq.PID, memReq.Address, memReq.Size)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if memReq.Type == "CPU" {
		sendDataToCPU(data)
	} else if memReq.Type == "IO" {
		sendContentToIO(string(data), memReq.Port)
	}
	w.Write(data)
}

func readMemory(pid int, addresses []int, size int) ([]byte, error) {
	mutexMemory.Lock()
	defer mutexMemory.Unlock()

	if _, exists := global_pageTable[pid]; !exists {
		return nil, fmt.Errorf("Process with PID %d not found", pid)
	}

	var result []byte
	for _, address := range addresses {
		if address < 0 || address >= len(memory) {
			return nil, fmt.Errorf("memory access out of bounds at address %d", address)
		}
		result = append(result, memory[address])
	}
	log.Printf("PID: %d - Accion: LEER - Direccion fisica: %d - Tamaño %d", pid, addresses[0], size)
	return result, nil
}

func sendDataToCPU(content []byte) error {
	CPUurl := fmt.Sprintf("http://%s:%d/receiveDataFromMemory", globals.ClientConfig.IpCPU, globals.ClientConfig.PuertoCPU)
	ContentResponseTest, err := json.Marshal(content)
	if err != nil {
		log.Fatalf("Error al serializar el Input: %v", err)
	}

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

func sendContentToIO(content string, Puerto int) error {
	var BodyContent BodyContent
	BodyContent.Content = content
	IOurl := fmt.Sprintf("http://%s:%d/receiveContentFromMemory", globals.ClientConfig.IpEntradaSalida, Puerto)
	ContentResponseTest, err := json.Marshal(BodyContent)
	if err != nil {
		log.Fatalf("Error al serializar el Input: %v", err)
	}

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

/*------------------------------STDIN---------------------------------*/

func WriteMemoryHandler(w http.ResponseWriter, r *http.Request) {
	var memReq MemoryRequest
	time.Sleep(time.Duration(globals.ClientConfig.DelayResponse) * time.Millisecond)
	if err := json.NewDecoder(r.Body).Decode(&memReq); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := writeMemory(memReq.PID, memReq.Address, memReq.Data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func writeMemory(pid int, addresses []int, data []byte) error {
	mutexMemory.Lock()
	defer mutexMemory.Unlock()

	if _, exists := global_pageTable[pid]; !exists {
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
	log.Printf("PID: %d - Accion: ESCRIBIR - Direccion fisica: %d - Tamaño %d", pid, addresses[0], len(addresses))
	return nil
}

func GetPageFromCPU(w http.ResponseWriter, r *http.Request) {
	var bodyCPUpage1 bodyCPUpage
	err := json.NewDecoder(r.Body).Decode(&bodyCPUpage1)
	if err != nil {
		http.Error(w, "Error decoding JSON data", http.StatusInternalServerError)
		return
	}
	CPUpid := bodyCPUpage1.Pid
	CPUpage := bodyCPUpage1.Page

	sendFrameToCPU(CPUpid, CPUpage)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Page recibido correctamente"))
}

func sendFrameToCPU(pid int, page int) error {
	var bodyFrame BodyFrame
	CPUurl := fmt.Sprintf("http://%s:%d/recieveFrame", globals.ClientConfig.IpCPU, globals.ClientConfig.PuertoCPU)

	if page > len(global_pageTable[pid]) {
		finishProcessToKernel(pid)
		return nil
	}
	frame := global_pageTable[pid][page]
	log.Printf("PID: %d - Pagina: %d - Marco: %d", pid, page, frame)
	bodyFrame.Frame = frame
	FrameResponseTest, err := json.Marshal(bodyFrame)
	if err != nil {
		log.Fatalf("Error al serializar el frame: %v", err)
	}

	resp, err := http.Post(CPUurl, "application/json", bytes.NewBuffer(FrameResponseTest))
	if err != nil {
		log.Fatalf("error al enviar la solicitud al módulo de memoria: %v", err)
	}
	defer resp.Body.Close()
	return nil
}

func sendPageTamToCPU(tamPage int) {
	CPUurl := fmt.Sprintf("http://%s:%d/recievePageTam", globals.ClientConfig.IpCPU, globals.ClientConfig.PuertoCPU)
	var body BodyPageTam
	body.PageTam = tamPage
	PageTamResponseTest, err := json.Marshal(body)
	if err != nil {
		log.Fatalf("Error al serializar el tamPage: %v", err)
	}

	resp, err := http.Post(CPUurl, "application/json", bytes.NewBuffer(PageTamResponseTest))
	if err != nil {
		log.Fatalf("error al enviar la solicitud al módulo de memoria: %v", err)
	}
	defer resp.Body.Close()
}
