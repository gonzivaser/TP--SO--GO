package utils

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/sisoputnfrba/tp-golang/kernel/globals"
)

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

/*-------------------------------------------------STRUCTS--------------------------------------------------------*/
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

type BodyResponsePCB struct {
	Pcb PCB `json:"pcb"`
}

type PCB struct {
	Pid     int
	Quantum int
	State   string
	CpuReg  RegisterCPU
}

type ExecutionContext struct {
	Pid    int
	State  string
	CpuReg RegisterCPU
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

// Estructura para la interfaz genérica
type Payload struct {
	Nombre string `json:"nombre"`
	IO     int    `json:"io"`
}

type Finalizado struct {
	Finalizado bool `json:"finalizado"`
}

type Proceso struct {
	Request BodyRequest
	PCB     *PCB
}

type Syscall struct {
	TIME int `json:"time"`
}

type KernelRequest struct {
	PcbUpdated     ExecutionContext `json:"pcbUpdated"`
	MotivoDesalojo string           `json:"motivoDesalojo"`
	TimeIO         int              `json:"timeIO"`
	Interface      string           `json:"interface"`
	IoType         string           `json:"ioType"`
	Recurso        string           `json:"recurso"`
}

type RequestInterrupt struct {
	Interrupt bool   `json:"interrupt"`
	PID       int    `json:"pid"`
	Motivo    string `json:"motivo"`
}

type BodyRequestPort struct {
	Nombre string `json:"nombre"`
	Port   int    `json:"port"`
	Type   string `json:"type"`
}
type interfaz struct {
	Name string
	Port int
	Type string
}

type BodyRegisters struct {
	DirFisica []int `json:"dirFisica"`
	LengthREG int   `json:"lengthREG"`
	IOpid     int   `json:"iopid"`
}

type Process struct {
	PID   int `json:"pid"`
	Pages int `json:"pages,omitempty"`
}

var interfaces []interfaz

/*---------------------------------------------------VAR GLOBALES------------------------------------------------*/

var (
	readyChannel chan PCB
	nextPid      = 1
	DirFisica    []int
	LengthREG    int
	IOpid        int
	done         chan struct{}
	pauseChan    chan struct{}
	resumeChan   chan struct{}
	kernelPaused bool
	pauseMutex   sync.RWMutex
	//CPURequest   KernelRequest

)

// ----------DECLARACION DE COLAS POR ESTADO----------------
var colaNew []PCB
var colaReady []PCB
var colaReadyVRR []PCB
var colaExecution []PCB
var colaBlocked = make(map[string][]PCB) // Tiene que ser un map string[]PCB[]
var colaExit []PCB

var multiProgramacion chan int

// --------------------------------------------------------
// ----------DECLARACION DE MUTEX POR COLAS DE ESTADO----------------
var mutexNew sync.Mutex
var mutexReady sync.Mutex
var mutexReadyVRR sync.Mutex
var mutexExecution sync.Mutex
var mutexBlocked sync.Mutex
var mutexExit sync.Mutex
var mutexQuantum sync.Mutex

// --------------------------------------------------------
// ----------DECLARACION MUTEX MÓDULO----------------
var mutexExecutionCPU sync.Mutex // este mutex es para que no se envie dos procesos al mismo tiempo a la cpu
var mutexExecutionMEMORIA sync.Mutex

var mutexes = make(map[string]*sync.Mutex)

// --------------------------------------------------------

// ----------DECLARACION DE PROCESO EN EJECUCION----------------
var procesoEXEC Proceso // este proceso es el que se esta ejecutando
//----------------------------------------------------------------------

/*-------------------------------------------------FUNCIONES CREADAS----------------------------------------------*/

func PausarKernel() {
	pauseMutex.Lock()
	defer pauseMutex.Unlock()
	if !kernelPaused {
		kernelPaused = true
		close(pauseChan)
		pauseChan = make(chan struct{})
	}
}

func ReanudarKernel() {
	pauseMutex.Lock()
	defer pauseMutex.Unlock()
	if kernelPaused {
		kernelPaused = false
		close(resumeChan)
		resumeChan = make(chan struct{})
	}
}

func waitIfPaused() {
	pauseMutex.RLock()
	if !kernelPaused {
		pauseMutex.RUnlock()
		return
	}
	pauseMutex.RUnlock()

	<-resumeChan
}

func ProcessSyscall(w http.ResponseWriter, r *http.Request) {

	if globals.ClientConfig.AlgoritmoPlanificacion != "FIFO" {
		//log.Println("Se cierra el canal DONE ", globals.ClientConfig.AlgoritmoPlanificacion)
		close(done)
	}
	var CPURequest KernelRequest

	err := json.NewDecoder(r.Body).Decode(&CPURequest)
	if err != nil {
		http.Error(w, "Error al decodificar los datos JSON", http.StatusInternalServerError)
		return
	}
	log.Printf("Recibido Motivo de desalojo: %+v", CPURequest.MotivoDesalojo)

	waitIfPaused()

	if len(colaExecution) > 0 { // aca lo saco de la cola exec
		mutexExecution.Lock()
		colaExecution = append(colaExecution[:0], colaExecution[1:]...)
		mutexExecution.Unlock()
	} else {
		return
	}

	procesoEXEC.PCB.CpuReg = CPURequest.PcbUpdated.CpuReg
	procesoEXEC.PCB.Pid = CPURequest.PcbUpdated.Pid
	switch CPURequest.MotivoDesalojo {
	case "FINALIZADO":
		log.Printf("Finaliza el proceso %v - Motivo: SUCCESS", CPURequest.PcbUpdated.Pid)
		enqueueExitProcess(*procesoEXEC.PCB)

	case "INTERRUPCION POR IO":
		procesoEXEC.PCB.State = "BLOCKED"
		go handleSyscallIO(*procesoEXEC.PCB, CPURequest.TimeIO, CPURequest.Interface, CPURequest.IoType)

	case "CLOCK":
		log.Printf("PID: %v desalojado por fin de Quantum", CPURequest.PcbUpdated.Pid)
		go enqueueReadyProcess(*procesoEXEC.PCB)
	case "WAIT":
		procesoEXEC.PCB.State = "BLOCKED"
		go waitHandler(*procesoEXEC.PCB, CPURequest.Recurso)

	case "INTERRUPTED_BY_USER":
		log.Printf("Finaliza el proceso %v - Motivo: INTERRUPTED_BY_USER", CPURequest.PcbUpdated.Pid)
		enqueueExitProcess(*procesoEXEC.PCB)

	case "INVALID_RESOURCE":
		log.Printf("Finaliza el proceso %v - Motivo: INVALID_RESOURCE", CPURequest.PcbUpdated.Pid)
		enqueueExitProcess(*procesoEXEC.PCB)

	default:
		log.Printf("PID: %v desalojado desconocido por %v", CPURequest.PcbUpdated.Pid, CPURequest.MotivoDesalojo)
	}

	mutexExecutionCPU.Unlock()

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf("%v", CPURequest.PcbUpdated)))

}

func createStructuresMemory(pid int, pages int) error {
	memoriaURL := fmt.Sprintf("http://localhost:%d/createProcess", globals.ClientConfig.PuertoMemoria)
	var process Process
	process.PID = pid
	process.Pages = pages

	processBytes, err := json.Marshal(process)
	if err != nil {
		return fmt.Errorf("error al serializar los datos JSON: %v", err)
	}

	//log.Println("Enviando solicitud con contenido:", string(processBytes))

	resp, err := http.Post(memoriaURL, "application/json", bytes.NewBuffer(processBytes))
	if err != nil {
		return fmt.Errorf("error al enviar la solicitud al módulo de entradasalida: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("error en la respuesta del módulo de memoria: %v", resp.StatusCode)
	}
	log.Println("Respuesta del módulo de entradasalida recibida correctamente.")
	return nil
}

func IniciarProceso(w http.ResponseWriter, r *http.Request) {
	var request BodyRequest
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		http.Error(w, "Error decoding JSON data", http.StatusInternalServerError)
		return
	}

	waitIfPaused()
	// Create PCB
	pcb := createPCB()
	log.Printf("Se crea el proceso %d en NEW", pcb.Pid) // log obligatorio
	createStructuresMemory(pcb.Pid, 0)
	IniciarPlanificacionDeProcesos(request, pcb)

	// Response with the PID
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf(`{"pid":%d}`, pcb.Pid)))
}

func init() {
	globals.ClientConfig = IniciarConfiguracion("config.json") // tiene que prender la confi cuando arranca
	readyChannel = make(chan PCB, 1)
	multiProgramacion = make(chan int, globals.ClientConfig.Multiprogramacion)
	pauseChan = make(chan struct{})
	resumeChan = make(chan struct{})

	go handelMultiProg()

	if globals.ClientConfig != nil {
		if globals.ClientConfig.AlgoritmoPlanificacion == "FIFO" {
			go executeProcessFIFO()
		} else if globals.ClientConfig.AlgoritmoPlanificacion == "RR" {
			go executeProcessRR(globals.ClientConfig.Quantum)
		} else if globals.ClientConfig.AlgoritmoPlanificacion == "VRR" {
			go executeProcessVRR()
		}
	} else {
		log.Fatal("ClientConfig is not initialized")
	}
}

func handelMultiProg() {
	for {
		if len(colaNew) > 0 {
			multiProgramacion <- 0
			mutexNew.Lock()
			proceso := colaNew[0]
			colaNew = append(colaNew[:0], colaNew[1:]...)
			mutexNew.Unlock()
			enqueueReadyProcess(proceso)

		}
	}
}

func IniciarPlanificacionDeProcesos(request BodyRequest, pcb PCB) {
	proceso := Proceso{
		Request: request,
		PCB:     &pcb,
	}
	mutexNew.Lock()
	colaNew = append(colaNew, *proceso.PCB)
	mutexNew.Unlock()

	mutexExecutionMEMORIA.Lock()
	if err := SendPathToMemory(proceso.Request, proceso.PCB.Pid); err != nil {
		log.Printf("Error sending path to memory: %v", err)
		return
	}
	mutexExecutionMEMORIA.Unlock()

	// if len(colaNew) > 0 { // aca lo saco de la cola new y lo mando a ready
	// 	mutexNew.Lock()
	// 	colaNew = append(colaNew[:0], colaNew[1:]...)
	// 	mutexNew.Unlock()
	// }

	// //meter en ready
	// enqueueReadyProcess(*proceso.PCB)
}

func executeTask(proceso PCB) {
	procesoEXEC.PCB = &proceso
	procesoEXEC.PCB.State = "EXEC"
	//sacar de Ready y lo mando a execution
	if len(colaReady) > 0 && procesoEXEC.PCB.Quantum == 0 { // aca lo saco de la cola ready y lo mando a execution
		mutexReady.Lock()
		colaReady = append(colaReady[:0], colaReady[1:]...)
		//log.Printf("Cola R desalojada  %+v", colaReady)
		mutexReady.Unlock()
	} else if len(colaReadyVRR) > 0 && procesoEXEC.PCB.Quantum > 0 {
		mutexReadyVRR.Lock()
		colaReadyVRR = append(colaReadyVRR[:0], colaReadyVRR[1:]...)
		//log.Printf("Cola VRR desalojada  %+v", colaReadyVRR)
		mutexReadyVRR.Unlock()
	}
	//meter en execution
	mutexExecution.Lock()
	colaExecution = append(colaExecution, *procesoEXEC.PCB)
	mutexExecution.Unlock()

	if err := SendContextToCPU(*procesoEXEC.PCB); err != nil {
		log.Printf("Error sending context to CPU: %v", err)
		return
	}
}

// Function to check if a resource exists
func resourceExists(recurso string) (bool, int) {
	for i, r := range globals.ClientConfig.Recursos {
		if r == recurso {
			return true, i
		}
	}
	return false, -1
}

func RecieveWait(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Pid     int    `json:"pid"`
		Recurso string `json:"recurso"`
	}

	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Check if the resource exists
	recursoExistente, index := resourceExists(request.Recurso)
	if recursoExistente {
		// resto 1 si existe
		globals.ClientConfig.InstanciasRecursos[index] -= 1
		fmt.Println("Instancias recursos: ", globals.ClientConfig.InstanciasRecursos, request.Recurso)
		// Check if the number of instances is less than 0
		if globals.ClientConfig.InstanciasRecursos[index] < 0 {
			w.Write([]byte(`{"success": "false"}`))
			return
		}
	} else {
		// If the resource does not exist, send the process to EXIT
		w.Write([]byte(`{"success": "exit"}`))
		return
	}

	// Return execution to the process that requests the WAIT
	w.Write([]byte(`{"success": "true"}`))
}

func waitHandler(pcb PCB, recurso string) {
	log.Printf("PID: %v Bloqueado por: %s", pcb.Pid, recurso)
	mutexBlocked.Lock()
	colaBlocked[recurso] = append(colaBlocked[recurso], pcb)
	mutexBlocked.Unlock()
}

func HandleSignal(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Pid     int    `json:"pid"`
		Recurso string `json:"recurso"`
	}

	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var recurso = request.Recurso
	// Check if the resource exists
	recursoExistente, index := resourceExists(recurso)
	if recursoExistente {
		// Add 1 to the number of resource instances
		globals.ClientConfig.InstanciasRecursos[index]++
		if len(colaBlocked[recurso]) > 0 {
			// Unblock the first process in the blocked queue
			waitIfPaused()
			proceso := colaBlocked[recurso][0]
			mutexBlocked.Lock()
			colaBlocked[recurso] = colaBlocked[recurso][1:]
			mutexBlocked.Unlock()

			enqueueReadyProcess(proceso)
		}
	} else {
		// If the resource does not exist, send the process to EXIT
		w.Write([]byte(`{"success": "exit"}`))
		return
	}

	// Return execution to the process that requests the WAIT
	w.Write([]byte(`{"success": "true"}`))
}

func handleSyscallIO(pcb PCB, timeIo int, ioInterface string, ioType string) {
	if !InterfazExiste(ioInterface, ioType) {
		log.Printf("Finaliza el proceso %v - Motivo: INVALID_INTERFACE", pcb.Pid)
		//Llamar a funcion que finalioza el proceso aca se esta terminando y no se porque
		enqueueExitProcess(pcb)
		return
	}

	// meter en bloqueado
	log.Printf("PID: %d - Bloqueado por: %s", pcb.Pid, ioInterface)
	mutexBlocked.Lock()
	colaBlocked[ioInterface] = append(colaBlocked[ioInterface], pcb)
	mutexBlocked.Unlock()

	mutex, ok := mutexes[ioInterface]
	if !ok {
		mutex = &sync.Mutex{}
		mutexes[ioInterface] = mutex
	}

	mutex.Lock()
	SendIOToEntradaSalida(ioInterface, timeIo)
	mutex.Unlock()

	waitIfPaused()

	if len(colaBlocked[ioInterface]) > 0 { // aca lo saco de la cola blocked
		mutexBlocked.Lock()
		colaBlocked[ioInterface] = append(colaBlocked[ioInterface][:0], colaBlocked[ioInterface][1:]...)
		mutexBlocked.Unlock()

		enqueueReadyProcess(pcb)
	}
	log.Printf("Proceso %+v volvió de con. Quantum: %d", pcb.Pid, pcb.Quantum)

}

func InterfazExiste(nombre string, ioType string) bool {
	for _, interfaz := range interfaces {
		if interfaz.Name == nombre && interfaz.Type == ioType {
			return true
		}
	}
	return false
}

func enqueueReadyProcess(pcb PCB) {
	pcb.State = "READY"
	log.Printf("Proceso %+v con Quantum: %d", pcb.Pid, pcb.Quantum)
	if pcb.Quantum > 0 && globals.ClientConfig.AlgoritmoPlanificacion == "VRR" {
		mutexReadyVRR.Lock()
		colaReadyVRR = append(colaReadyVRR, pcb)
		mutexReadyVRR.Unlock()
		readyChannel <- pcb
		pidSlice := make([]int, 0, len(colaReadyVRR))
		for _, pcb := range colaReadyVRR {
			pidSlice = append(pidSlice, pcb.Pid)
		}
		log.Printf("Cola Ready VRR: %+v", pidSlice)
	} else {
		mutexReady.Lock()
		pcb.Quantum = 0
		colaReady = append(colaReady, pcb)
		mutexReady.Unlock()
		readyChannel <- pcb
		pidSlice := make([]int, 0, len(colaReady))
		for _, pcb := range colaReady {
			pidSlice = append(pidSlice, pcb.Pid)
		}
		log.Printf("Cola Ready: %+v", pidSlice)
	}
}

func enqueueExitProcess(pcb PCB) {
	pcb.State = "EXIT"
	mutexExit.Lock()
	<-multiProgramacion
	colaExit = append(colaExit, pcb)
	mutexExit.Unlock()
}

func executeProcessFIFO() {
	var proceso PCB
	for {
		proceso = <-readyChannel
		if len(colaReady) > 0 {
			mutexExecutionCPU.Lock()
			proceso = colaReady[0]
			executeTask(proceso)
		}
	}

}

func executeProcessRR(quantum int) {
	var proceso PCB
	for {
		proceso = <-readyChannel
		if len(colaReady) > 0 {
			mutexExecutionCPU.Lock()
			proceso = colaReady[0]
			go startQuantum(quantum, proceso)
			executeTask(proceso)
		}
	}

}

func executeProcessVRR() {
	var proceso PCB
	var quantum int
	for {
		proceso = <-readyChannel
		if len(colaReadyVRR) > 0 {
			mutexExecutionCPU.Lock()
			proceso = colaReadyVRR[0]
			quantum = proceso.Quantum
			go startQuantum(quantum, proceso)
			executeTask(proceso)

		} else if len(colaReady) > 0 {
			mutexExecutionCPU.Lock()
			proceso = colaReady[0]
			quantum = globals.ClientConfig.Quantum
			go startQuantum(quantum, proceso)
			executeTask(proceso)
		}

	}
}
func startQuantum(quantum int, proceso PCB) {
	log.Printf("PID %d - Quantum iniciado %d", proceso.Pid, quantum)
	mutexQuantum.Lock()
	defer mutexQuantum.Unlock()
	done = make(chan struct{})
	timer := time.NewTimer(time.Duration(quantum) * time.Millisecond)
	start := time.Now()

	for {
		select {
		case <-timer.C:
			elapsed := time.Since(start)
			log.Printf("PID %d - Quantum terminado. Tiempo real transcurrido: %v", proceso.Pid, elapsed)
			if err := SendInterrupt(proceso.Pid, "CLOCK"); err != nil {
				log.Printf("Error sending interrupt to CPU: %v", err)
			}
			return
		case <-done:
			if !timer.Stop() {
				<-timer.C
			}
			elapsed := time.Since(start)
			remainingQuantum := quantum - int(elapsed.Milliseconds())
			if remainingQuantum < 0 {
				remainingQuantum = 0
			}
			log.Printf("PID %d - Proceso desalojado antes de que el quantum termine. Quantum restante %d", proceso.Pid, remainingQuantum)
			procesoEXEC.PCB.Quantum = remainingQuantum
			return
		default:
			// Evitar bloqueo del select
			time.Sleep(time.Millisecond)
		}
	}
}

func createPCB() PCB {
	nextPid++

	return PCB{
		Pid: nextPid - 1, // ASIGNO EL VALOR ANTERIOR AL pid

		Quantum: 0,
		State:   "NEW",

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
}

func SendPathToMemory(request BodyRequest, pid int) error {
	memoriaURL := fmt.Sprintf("http://localhost:8085/setInstructionFromFileToMap?pid=%d&path=%s", pid, request.Path)
	savedPathJSON, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("error al serializar los datos JSON: %v", err)
	}

	//log.Println("Enviando solicitud con contenido:", string(savedPathJSON))

	resp, err := http.Post(memoriaURL, "application/json", bytes.NewBuffer(savedPathJSON))
	if err != nil {
		return fmt.Errorf("error al enviar la solicitud al módulo de memoria: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("error en la respuesta del módulo de memoria: %v", resp.StatusCode)
	}

	//log.Println("Respuesta del módulo de memoria recibida correctamente.")
	return nil
}

func SendContextToCPU(pcb PCB) error {
	cpuURL := "http://localhost:8075/receivePCB"

	context := pcb
	pcbResponseTest, err := json.Marshal(context)
	if err != nil {
		return fmt.Errorf("error al serializar el PCB: %v", err)
	}

	//log.Println("Enviando solicitud con contenido:", string(pcbResponseTest))

	resp, err := http.Post(cpuURL, "application/json", bytes.NewBuffer(pcbResponseTest))
	if err != nil {
		return fmt.Errorf("error al enviar la solicitud al módulo de cpu: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("error en la respuesta del módulo de cpu: %v", resp.StatusCode)
	}

	//log.Println("Respuesta del módulo de cpu recibida correctamente.")
	return nil
}

func RecievePortOfInterfaceFromIO(w http.ResponseWriter, r *http.Request) {
	var requestPort BodyRequestPort
	var interfaz interfaz
	err := json.NewDecoder(r.Body).Decode(&requestPort)
	if err != nil {
		http.Error(w, "Error decoding JSON data", http.StatusInternalServerError)
		return
	}
	interfaz.Name = requestPort.Nombre
	interfaz.Port = requestPort.Port
	interfaz.Type = requestPort.Type

	interfaces = append(interfaces, interfaz)
	SendPortOfInterfaceToMemory(interfaz.Name, interfaz.Port)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf("Port received: %d", requestPort.Port)))
}

func SendIOToEntradaSalida(nombre string, io int) error {
	payload := Payload{
		Nombre: nombre,
		IO:     io,
	}
	var interfazEncontrada interfaz // Asume que Interfaz es el tipo de tus interfaces

	for _, interfaz := range interfaces {
		if interfaz.Name == payload.Nombre {
			interfazEncontrada = interfaz
			break
		}
	}
	if interfazEncontrada != (interfaz{}) && interfazEncontrada.Type == "STDOUT" || interfazEncontrada.Type == "STDIN" {
		SendREGtoIO(DirFisica, LengthREG, interfazEncontrada.Port) //envia los registros a IO
		//envia el payload a IO
		entradasalidaURL := fmt.Sprintf("http://localhost:%d/interfaz", interfazEncontrada.Port)

		ioResponseTest, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("error al serializar el payload: %v", err)
		}

		resp, err := http.Post(entradasalidaURL, "application/json", bytes.NewBuffer(ioResponseTest))
		if err != nil {
			return fmt.Errorf("error al enviar la solicitud al módulo de cpu: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("error en la respuesta del módulo de cpu: %v", resp.StatusCode)
		}

		//log.Println("Respuesta del módulo de IO recibida correctamente.")
		return nil
	} else if interfazEncontrada != (interfaz{}) && interfazEncontrada.Type == "GENERICA" {
		entradasalidaURL := fmt.Sprintf("http://localhost:%d/interfaz", interfazEncontrada.Port)

		ioResponseTest, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("error al serializar el payload: %v", err)
		}

		resp, err := http.Post(entradasalidaURL, "application/json", bytes.NewBuffer(ioResponseTest))
		if err != nil {
			return fmt.Errorf("error al enviar la solicitud al módulo de cpu: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("error en la respuesta del módulo de cpu: %v", resp.StatusCode)
		}

		//log.Println("Respuesta del módulo de IO recibida correctamente.")
		return nil
	}
	return nil
}

// esto es solamente para stdin y stdout

func RecieveREGFromCPU(w http.ResponseWriter, r *http.Request) {
	var bodyRegisters BodyRegisters
	err := json.NewDecoder(r.Body).Decode(&bodyRegisters)
	if err != nil {
		http.Error(w, "Error decoding JSON data", http.StatusInternalServerError)
		return
	}
	DirFisica = bodyRegisters.DirFisica
	LengthREG = bodyRegisters.LengthREG

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf("Registers received: %v", bodyRegisters)))
}

func SendREGtoIO(REGdireccion []int, lengthREG int, port int) error {
	ioURL := fmt.Sprintf("http://localhost:%d/recieveREG", port)
	var BodyRegister BodyRegisters
	BodyRegister.DirFisica = REGdireccion
	BodyRegister.LengthREG = lengthREG
	BodyRegister.IOpid = IOpid

	savedRegJSON, err := json.Marshal(BodyRegister)
	if err != nil {
		return fmt.Errorf("error al serializar los datos JSON: %v", err)
	}

	resp, err := http.Post(ioURL, "application/json", bytes.NewBuffer(savedRegJSON))
	if err != nil {
		return fmt.Errorf("error al enviar la solicitud al módulo de entradasalida: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("error en la respuesta del módulo de entradasalida: %v", resp.StatusCode)
	}

	//log.Println("Respuesta del módulo de entradasalida recibida correctamente.")
	return nil
}

func SendPortOfInterfaceToMemory(nombreInterfaz string, puerto int) error {
	memoriaURL := fmt.Sprintf("http://localhost:%d/SendPortOfInterfaceToMemory", globals.ClientConfig.PuertoMemoria)
	body := BodyRequestPort{
		Nombre: nombreInterfaz,
		Port:   puerto,
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("error al serializar el body: %v", err)
	}

	resp, err := http.Post(memoriaURL, "application/json", bytes.NewBuffer(bodyBytes))
	if err != nil {
		return fmt.Errorf("error al enviar la solicitud al módulo de memoria: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("error en la respuesta del módulo de memoria en port of interface: %v, %v", resp.StatusCode, resp.Request.Response)
	}

	///log.Println("Respuesta del módulo de memoria recibida correctamente.")
	return nil
}

func RecieveFileNameFromCPU(w http.ResponseWriter, r *http.Request) {
	var fileNmae string
	err := json.NewDecoder(r.Body).Decode(&fileNmae)
	if err != nil {
		http.Error(w, "Error decoding JSON data", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf("Registers received: %v", fileNmae)))
}

func SendInterrupt(pid int, motivo string) error {
	cpuURL := "http://localhost:8075/interrupt"

	RequestInterrupt := RequestInterrupt{
		Interrupt: true,
		PID:       pid,
		Motivo:    motivo,
	}

	hayQuantumBytes, err := json.Marshal(RequestInterrupt)
	if err != nil {
		log.Printf("Error al serializar el valor de hayQuantum: %v", err)
		return err
	}
	log.Printf("Mandando interrupción a la CPU PID: %d", pid)
	resp, err := http.Post(cpuURL, "application/json", bytes.NewBuffer(hayQuantumBytes))
	if err != nil {
		log.Printf("Error al enviar la solicitud al módulo de cpu: %v", err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Error en la respuesta del módulo de cpu: %v", resp.StatusCode)
	}

	//log.Println("Solicitado la interrupción del módulo CPU.")
	return nil
}

/*---------------------------------------------FUNCIONES OBLIGATORIAS--------------------------------------------------*/

// New function to check if a PID exists
func findPCB(pid int) (PCB, error) {
	queues := map[string][]PCB{
		"New":       colaNew,
		"Ready":     colaReady,
		"Execution": colaExecution,
		"Exit":      colaExit,
	}

	for state, queue := range colaBlocked {
		queues["Blocked "+state] = queue
	}

	for _, queue := range queues {
		for _, pcb := range queue {
			if pcb.Pid == pid {
				return pcb, nil
			}
		}
	}

	return PCB{}, fmt.Errorf("PID not found")
}

func FinalizarProceso(w http.ResponseWriter, r *http.Request) {
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
	PausarKernel()
	// Use pidExists to check if the PID exists in any of the queues
	pcb, err := findPCB(pid)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	//log.Printf("Finalizing process %v - Reason: <SUCCESS / INVALID_RESOURCE / INVALID_WRITE> con estado %v", pcb.Pid, pcb.State)

	//Llamar a funcion que finalioza el proceso

	if pcb.State == "EXEC" {
		SendInterrupt(pcb.Pid, "INTERRUPTED_BY_USER")
		deletePagesmemory(pcb.Pid)
	} else {
		eliminarProceso(pcb.Pid)
		deletePagesmemory(pcb.Pid)
		enqueueExitProcess(pcb)
		log.Printf("Finaliza el proceso %v - Motivo: INTERRUPTED_BY_USER", pcb.Pid)

	}
	ReanudarKernel()
	w.WriteHeader(http.StatusOK)
}

func deletePagesmemory(pid int) {
	memoriaURL := fmt.Sprintf("http://localhost:8085/terminateProcess?pid=%d", pid)
	resp, err := http.Post(memoriaURL, "application/json", nil)
	if err != nil {
		log.Printf("Error al enviar la solicitud al módulo de memoria: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Error en la respuesta del módulo de memoria: %v, %v", resp.StatusCode, resp)
	}
}

func EstadoProceso(w http.ResponseWriter, r *http.Request) {
	pidStr := r.PathValue("pid")
	if pidStr == "" {
		http.Error(w, "PID no especificado", http.StatusBadRequest)
		return
	}
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		log.Printf("Error converting pid to integer: %v", err)
		http.Error(w, "PID inválido", http.StatusBadRequest)
		return
	}

	processState := findPID(pid)

	BodyResponse := BodyResponseState{
		State: processState,
	}

	stateResponse, _ := json.Marshal(BodyResponse)

	w.WriteHeader(http.StatusOK)
	w.Write(stateResponse)
}

func IniciarPlanificacion(w http.ResponseWriter, r *http.Request) {
	ReanudarKernel()
	w.WriteHeader(http.StatusOK)
}

func DetenerPlanificacion(w http.ResponseWriter, r *http.Request) {
	PausarKernel()
	w.WriteHeader(http.StatusOK)
}

func findPID(pid int) string {
	queues := map[string][]PCB{
		"New":       colaNew,
		"Ready":     colaReady,
		"Execution": colaExecution,
		"Exit":      colaExit,
	}

	for state, queue := range colaBlocked {
		queues["Blocked "+state] = queue
	}

	for state, queue := range queues {
		for _, pcb := range queue {
			if pcb.Pid == pid {
				return state
			}
		}
	}

	return "PID not found"
}
func eliminarProceso(pid int) error {
	var findIt = false
	colas := []*[]PCB{&colaNew, &colaReady, &colaReadyVRR}
	for _, cola := range colas {
		for i, proceso := range *cola {
			if proceso.Pid == pid {
				// Eliminar el proceso de la cola
				findIt = true
				*cola = append((*cola)[:i], (*cola)[i+1:]...)
				//log.Printf("Proceso %v eliminado de la cola: %v", pid, cola)
				return nil
			}
		}
	}
	if !findIt {
		for key, cola := range colaBlocked {
			for i, proceso := range cola {
				if proceso.Pid == pid {
					// Eliminar el proceso de la cola
					colaBlocked[key] = append(cola[:i], cola[i+1:]...)
					//log.Printf("Proceso %v eliminado de la cola: %v", pid, key)
					return nil
				}
			}
		}
	}
	return errors.New("Proceso no encontrado")
}

type ProcessState struct {
	PID   int    `json:"pid"`
	State string `json:"state"`
}

func ListarProcesos(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	queues := map[string][]PCB{
		"New":       colaNew,
		"Ready":     colaReady,
		"Ready+":    colaReadyVRR,
		"Execution": colaExecution,
		"Exit":      colaExit,
	}

	for state, queue := range colaBlocked {
		queues["Blocked "+state] = queue
	}

	var processStates []ProcessState
	for state, queue := range queues {
		for _, pcb := range queue {
			processStates = append(processStates, ProcessState{PID: pcb.Pid, State: state})
		}
	}

	json, err := json.Marshal(processStates)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(json)
}

func ConfigurarLogger() {
	logFile, err := os.OpenFile("kernel.log", os.O_CREATE|os.O_APPEND|os.O_RDWR, 0666)
	if err != nil {
		panic(err)
	}
	mw := io.MultiWriter(os.Stdout, logFile)
	log.SetOutput(mw)
}
