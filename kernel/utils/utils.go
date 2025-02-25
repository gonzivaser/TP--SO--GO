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

type BodyResponseState struct {
	State string `json:"state"`
}

type BodyRequest struct {
	Path string `json:"path"`
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
	Pid    int    `json:"pid"`
}

type Proceso struct {
	Request BodyRequest
	PCB     PCB
}

type ProcessState struct {
	PID   int    `json:"pid"`
	State string `json:"state"`
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
	IOpid     int   `json:"iopid"`
	DirFisica []int `json:"dirFisica"`
	LengthREG int   `json:"lengthREG"`
}

type Process struct {
	PID   int `json:"pid"`
	Pages int `json:"pages,omitempty"`
}

type ProcessData struct {
	Pid             int
	LengthREG       int
	DireccionFisica []int
}

// -------------------------------VAR GLOBALES------------------------------ //

var (
	readyChannel            chan PCB
	newChannel              chan PCB
	quantumChannel          chan struct{}
	pauseChannel            chan struct{}
	resumeChannel           chan struct{}
	multiprogrammingChannel chan int

	nextPid = 1

	kernelPaused bool
	pauseMutex   sync.RWMutex

	global_interfaces []interfaz
	processDataMap    sync.Map
	DirFisica         []int
	LengthREG         int

	global_executionProcess Proceso // este proceso es el que se esta ejecutando

	global_quantumMap      = make(map[int]int)
	global_resourcesPIDMap = make(map[int][]string)
)

// ----------DECLARACION DE COLAS POR ESTADO---------------- //
var (
	queueNew       []PCB
	queueReady     []PCB
	queueReadyVRR  []PCB
	queueExecution []PCB
	queueExit      []PCB
	queueBlocked   = make(map[string][]PCB) // Tiene que ser un map una key (interfaz/Recurso) y un array de PCBs
)

// ----------DECLARACION DE MUTEX---------------- //

var (
	mutexNew       sync.Mutex
	mutexReady     sync.Mutex
	mutexReadyVRR  sync.Mutex
	mutexExecution sync.Mutex
	mutexBlocked   sync.Mutex
	mutexExit      sync.Mutex

	mutexExecutionCPU     sync.Mutex // este mutex es para que no se envie dos procesos al mismo tiempo a la cpu
	mutexExecutionMEMORIA sync.Mutex

	mutexInterfacceMap = make(map[string]*sync.Mutex)

	mutexQuantum sync.Mutex // este mutex es para que espere a que termine el quantum del proceso anterio
)

// ---------FileName global----------------------- //

var (
	fileName      string
	fsInstruction string
	fsRegTam      int
	fsRegDirec    []int
	fsRegPuntero  int
)

type FSstructure struct {
	FileName      string `json:"filename"`
	FSInstruction string `json:"fsinstruction"`
	FSRegTam      int    `json:"fsregtam"`
	FSRegDirec    []int  `json:"fsregdirec"`
	FSRegPuntero  int    `json:"fsregpuntero"`
}

/*------------------------------------FUNCIONES------------------------------------*/

func ConfigurarLogger() {
	logFile, err := os.OpenFile("kernel.log", os.O_CREATE|os.O_APPEND|os.O_RDWR, 0666)
	if err != nil {
		panic(err)
	}
	mw := io.MultiWriter(os.Stdout, logFile)
	log.SetOutput(mw)
}

func init() {
	globals.ClientConfig = IniciarConfiguracion(os.Args[1]) // tiene que prender la confi cuando arranca
	readyChannel = make(chan PCB, globals.ClientConfig.Multiprogramacion)
	newChannel = make(chan PCB, globals.ClientConfig.Multiprogramacion)
	multiprogrammingChannel = make(chan int, globals.ClientConfig.Multiprogramacion)
	pauseChannel = make(chan struct{})
	resumeChannel = make(chan struct{})

	go handelMultiProg()

	if globals.ClientConfig != nil {
		if globals.ClientConfig.AlgoritmoPlanificacion == "FIFO" {
			go planFIFO()
		} else if globals.ClientConfig.AlgoritmoPlanificacion == "RR" {
			go planRR(globals.ClientConfig.Quantum)
		} else if globals.ClientConfig.AlgoritmoPlanificacion == "VRR" {
			go planVRR()
		}
	} else {
		log.Fatal("ClientConfig is not initialized")
	}
}

func InitializeProcess(w http.ResponseWriter, r *http.Request) {
	var request BodyRequest
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		http.Error(w, "Error decoding JSON data", http.StatusInternalServerError)
		return
	}

	waitIfPaused()
	// Create PCB
	pcb := createPCB()
	createStructuresMemory(pcb.Pid, 0)
	planProcessNew(request, pcb)
	global_quantumMap[pcb.Pid] = 0
	newChannel <- pcb
	// Response with the PID
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf(`{"pid":%d}`, pcb.Pid)))
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

func createStructuresMemory(pid int, pages int) error {
	memoriaURL := fmt.Sprintf("http://%s:%d/createProcess", globals.ClientConfig.IpMemoria, globals.ClientConfig.PuertoMemoria)
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
	//log.Println("Respuesta del módulo de entradasalida recibida correctamente.")
	return nil
}

func planProcessNew(request BodyRequest, pcb PCB) {
	mutexNew.Lock()
	queueNew = append(queueNew, pcb)
	mutexNew.Unlock()

	mutexExecutionMEMORIA.Lock()
	if err := sendPathToMemory(request, pcb.Pid); err != nil {
		log.Printf("Error sending path to memory: %v", err)
		return
	}
	mutexExecutionMEMORIA.Unlock()
}

func sendPathToMemory(request BodyRequest, pid int) error {
	memoriaURL := fmt.Sprintf("http://%s:8085/setInstructionFromFileToMap?pid=%d&path=%s", globals.ClientConfig.IpMemoria, pid, request.Path)
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

func handelMultiProg() {
	for {
		proceso := <-newChannel
		if len(queueNew) > 0 {
			multiprogrammingChannel <- 0
			mutexNew.Lock()
			proceso = queueNew[0]
			queueNew = append(queueNew[:0], queueNew[1:]...)
			mutexNew.Unlock()
			log.Printf("Se crea el proceso %d en NEW", proceso.Pid)
			enqueueReadyProcess(proceso)

		}
	}
}

func planFIFO() {
	var proceso PCB
	for {
		proceso = <-readyChannel
		if len(queueReady) > 0 {
			mutexExecutionCPU.Lock()
			proceso = queueReady[0]
			executeTask(proceso)
		}
	}

}

func planRR(quantum int) {
	var proceso PCB
	for {
		proceso = <-readyChannel
		if len(queueReady) > 0 {
			mutexExecutionCPU.Lock()
			proceso = queueReady[0]
			go startQuantum(quantum, proceso.Pid)
			executeTask(proceso)
		}
	}

}

func planVRR() {
	var proceso PCB
	var quantum int
	for {
		proceso = <-readyChannel
		if len(queueReadyVRR) > 0 {
			mutexExecutionCPU.Lock()
			proceso = queueReadyVRR[0]
			//log.Printf("PID %d (VRR)- Quantum iniciado %d", proceso.Pid, global_quantumMap[proceso.Pid])
			go startQuantum(global_quantumMap[proceso.Pid], proceso.Pid)
			executeTask(proceso)

		} else if len(queueReady) > 0 {
			mutexExecutionCPU.Lock()
			proceso = queueReady[0]
			quantum = globals.ClientConfig.Quantum
			//log.Printf("PID %d (RR)- Quantum iniciado %d", proceso.Pid, globals.ClientConfig.Quantum)
			go startQuantum(quantum, proceso.Pid)
			executeTask(proceso)
		}

	}
}

func executeTask(pcb PCB) {
	//sacar de Ready y lo mando a execution
	if len(queueReady) > 0 && global_quantumMap[pcb.Pid] == 0 { // aca lo saco de la cola ready y lo mando a execution
		mutexReady.Lock()
		queueReady = append(queueReady[:0], queueReady[1:]...)
		//log.Printf("Cola R desalojada  %+v", queueReady)
		mutexReady.Unlock()
	} else if len(queueReadyVRR) > 0 && global_quantumMap[pcb.Pid] > 0 {
		mutexReadyVRR.Lock()
		queueReadyVRR = append(queueReadyVRR[:0], queueReadyVRR[1:]...)
		//log.Printf("Cola VRR desalojada  %+v", queueReadyVRR)
		mutexReadyVRR.Unlock()
	}
	log.Printf("PID: %d - Estado Anterior: %s - Estado Actual: EXEC", pcb.Pid, pcb.State)
	pcb.State = "EXEC"
	//meter en execution
	mutexExecution.Lock()
	queueExecution = append(queueExecution, pcb)
	mutexExecution.Unlock()

	if err := sendContextToCPU(pcb); err != nil {
		log.Printf("Error sending context to CPU: %v", err)
		return
	}
}

func startQuantum(quantum int, pid int) {
	//log.Printf("PID %d - Quantum iniciado %d", pid, quantum)
	mutexQuantum.Lock()
	global_quantumMap[pid] = 0
	defer mutexQuantum.Unlock()
	quantumChannel = make(chan struct{})
	timer := time.NewTimer(time.Duration(quantum) * time.Millisecond)
	start := time.Now()

	for {
		select {
		case <-timer.C:
			//log.Printf("PID %d - Quantum terminado. Tiempo real transcurrido: %v", pid, elapsed)
			if err := interruptToCPU(pid, "CLOCK"); err != nil {
				log.Printf("Error sending interrupt to CPU: %v", err)
			}
			return
		case <-quantumChannel:
			if !timer.Stop() {
				<-timer.C
			}
			elapsed := time.Since(start)
			remainingQuantum := quantum - int(elapsed.Milliseconds())
			if remainingQuantum < 0 {
				remainingQuantum = 0
			}
			//log.Printf("PID %d - Proceso desalojado antes de que el quantum termine. Quantum restante %d", pid, remainingQuantum)
			if globals.ClientConfig.AlgoritmoPlanificacion == "VRR" {
				global_quantumMap[pid] = remainingQuantum
			}
			return
		default:
			// Evitar bloqueo del select
			time.Sleep(time.Millisecond)
		}
	}
}

func sendContextToCPU(pcb PCB) error {
	cpuURL := fmt.Sprintf("http://%s:%d/receivePCB", globals.ClientConfig.IpCPU, globals.ClientConfig.PuertoCPU)

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

func interruptToCPU(pid int, motivo string) error {
	cpuURL := fmt.Sprintf("http://%s:%d/interrupt", globals.ClientConfig.IpCPU, globals.ClientConfig.PuertoCPU)

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
	//log.Printf("Mandando interrupción a la CPU PID: %d", pid)
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

func ProcessSyscallFromCPU(w http.ResponseWriter, r *http.Request) {

	if globals.ClientConfig.AlgoritmoPlanificacion != "FIFO" {
		//log.Println("Se cierra el canal quantumChannel ", globals.ClientConfig.AlgoritmoPlanificacion)
		close(quantumChannel)
	}
	var CPURequest KernelRequest

	err := json.NewDecoder(r.Body).Decode(&CPURequest)
	if err != nil {
		http.Error(w, "Error al decodificar los datos JSON", http.StatusInternalServerError)
		return
	}
	//log.Printf("Recibido Motivo de desalojo: %+v", CPURequest.MotivoDesalojo)

	waitIfPaused()

	if len(queueExecution) > 0 { // aca lo saco de la cola exec
		mutexExecution.Lock()
		queueExecution = append(queueExecution[:0], queueExecution[1:]...)
		mutexExecution.Unlock()
	} else {
		return
	}

	global_executionProcess.PCB.CpuReg = CPURequest.PcbUpdated.CpuReg
	global_executionProcess.PCB.Pid = CPURequest.PcbUpdated.Pid
	global_executionProcess.PCB.State = CPURequest.PcbUpdated.State

	switch CPURequest.MotivoDesalojo {
	case "FINALIZADO":
		log.Printf("Finaliza el proceso %v - Motivo: SUCCESS", CPURequest.PcbUpdated.Pid)
		enqueueExitProcess(global_executionProcess.PCB)

	case "INTERRUPCION POR IO":
		go handleSyscallIO(global_executionProcess.PCB, CPURequest.TimeIO, CPURequest.Interface, CPURequest.IoType)

	case "CLOCK":
		log.Printf("PID: %v desalojado por fin de Quantum", CPURequest.PcbUpdated.Pid)
		go enqueueReadyProcess(global_executionProcess.PCB)
	case "WAIT":
		go waitHandler(global_executionProcess.PCB, CPURequest.Recurso)

	case "INTERRUPTED_BY_USER":
		//log.Printf("Finaliza el proceso %v - Motivo: INTERRUPTED_BY_USER", CPURequest.PcbUpdated.Pid)
		enqueueExitProcess(global_executionProcess.PCB)

	case "INVALID_RESOURCE":
		log.Printf("Finaliza el proceso %v - Motivo: INVALID_RESOURCE", CPURequest.PcbUpdated.Pid)
		enqueueExitProcess(global_executionProcess.PCB)

	default:
		log.Printf("PID: %v desalojado desconocido por %v", CPURequest.PcbUpdated.Pid, CPURequest.MotivoDesalojo)
	}

	mutexExecutionCPU.Unlock()

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf("%v", CPURequest.PcbUpdated)))

}

/*------------------------------Entrada salida---------------------------------*/

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
	// log.Printf("Port received: %d, Name: %s, type: %s", requestPort.Port, requestPort.Nombre, requestPort.Type)

	global_interfaces = append(global_interfaces, interfaz)
	sendPortOfInterfaceToMemory(interfaz.Name, interfaz.Port)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf("Port received: %d", requestPort.Port)))
}

func handleSyscallIO(pcb PCB, timeIo int, ioInterface string, ioType string) {
	if !interfaceExists(ioInterface, ioType) {
		log.Printf("Finaliza el proceso %v - Motivo: INVALID_INTERFACE", pcb.Pid)
		//Llamar a funcion que finalioza el proceso aca se esta terminando y no se porque
		enqueueExitProcess(pcb)
		return
	}

	// meter en bloqueado
	enqueueBlockedProcess(pcb, ioInterface)

	mutex, ok := mutexInterfacceMap[ioInterface]
	if !ok {
		mutex = &sync.Mutex{}
		mutexInterfacceMap[ioInterface] = mutex
	}

	mutex.Lock()
	sendProcessToIO(ioInterface, timeIo, pcb.Pid)
	mutex.Unlock()

	waitIfPaused()

	if len(queueBlocked[ioInterface]) > 0 { // aca lo saco de la cola blocked
		mutexBlocked.Lock()
		queueBlocked[ioInterface] = append(queueBlocked[ioInterface][:0], queueBlocked[ioInterface][1:]...)
		mutexBlocked.Unlock()

		enqueueReadyProcess(pcb)
	}
	//log.Printf("Proceso %+v volvió de con. Quantum: %d", pcb.Pid, pcb.Quantum)
}

func interfaceExists(nombre string, ioType string) bool {
	for _, interfaz := range global_interfaces {
		if interfaz.Name == nombre && interfaz.Type == ioType {
			return true
		}
	}
	return false
}

func sendProcessToIO(nombre string, io int, pid int) error {
	payload := Payload{
		Nombre: nombre,
		IO:     io,
		Pid:    pid,
	}
	var interfazEncontrada interfaz // Asume que Interfaz es el tipo de tus interfaces

	for _, interfaz := range global_interfaces {
		if interfaz.Name == payload.Nombre {
			interfazEncontrada = interfaz
			break
		}
	}
	if interfazEncontrada != (interfaz{}) && interfazEncontrada.Type == "STDOUT" || interfazEncontrada.Type == "STDIN" {

		processData, ok := getProcessData(pid)
		if !ok {
			// log.Printf("No se encontraron datos para el PID: %d", payload.Pid)
			// Manejar el caso donde no hay datos para el PID
		}

		sendREGtoIO(processData.DireccionFisica, processData.LengthREG, interfazEncontrada.Port, pid) //envia los registros a IO

		//envia el payload a IO
		entradasalidaURL := fmt.Sprintf("http://%s:%d/interfaz", globals.ClientConfig.IpEntradaSalida, interfazEncontrada.Port)

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
		entradasalidaURL := fmt.Sprintf("http://%s:%d/interfaz", globals.ClientConfig.IpEntradaSalida, interfazEncontrada.Port)

		ioResponseTest, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("error al serializar el payload: %v", err)
		}
		// log.Printf("LA URL ES: %s", entradasalidaURL)

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
	} else if interfazEncontrada != (interfaz{}) && interfazEncontrada.Type == "DialFS" {
		sendFSDataToIO(fileName, fsInstruction, interfazEncontrada.Port, fsRegTam, fsRegDirec, fsRegPuntero) //envia los registros a IO
		entradasalidaURL := fmt.Sprintf("http://%s:%d/interfaz", globals.ClientConfig.IpEntradaSalida, interfazEncontrada.Port)

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

func sendPortOfInterfaceToMemory(nombreInterfaz string, puerto int) error {
	memoriaURL := fmt.Sprintf("http://%s:%d/SendPortOfInterfaceToMemory", globals.ClientConfig.IpMemoria, globals.ClientConfig.PuertoMemoria)
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

func getProcessData(pid int) (ProcessData, bool) {
	data, ok := processDataMap.Load(pid)
	if !ok {
		return ProcessData{}, false
	}
	return data.(ProcessData), true
}

/*------------------------------Manejo de colas---------------------------------*/

func enqueueReadyProcess(pcb PCB) {
	log.Printf("PID: %d - Estado Anterior: %s - Estado Actual: READY", pcb.Pid, pcb.State)
	pcb.State = "READY"
	if global_quantumMap[pcb.Pid] > 0 && globals.ClientConfig.AlgoritmoPlanificacion == "VRR" {
		mutexReadyVRR.Lock()
		queueReadyVRR = append(queueReadyVRR, pcb)
		log.Printf("Cola Ready VRR: %+v", listIds(queueReadyVRR))
		mutexReadyVRR.Unlock()
		readyChannel <- pcb
	} else {
		pcb.Quantum = 0
		mutexReady.Lock()
		queueReady = append(queueReady, pcb)
		log.Printf("Cola Ready: %+v", listIds(queueReady))
		mutexReady.Unlock()
		readyChannel <- pcb
	}
}

func enqueueBlockedProcess(pcb PCB, key string) {
	log.Printf("PID: %d - Estado Anterior: %s - Estado Actual: BLOCKED", pcb.Pid, pcb.State)
	pcb.State = "BLOCKED"
	mutexBlocked.Lock()
	queueBlocked[key] = append(queueBlocked[key], pcb)
	mutexBlocked.Unlock()
	log.Printf("PID: %d - Bloqueado por: %s", pcb.Pid, key)
}

func listIds(cola []PCB) []int {
	pidSlice := make([]int, 0, len(cola))
	for _, pcb := range cola {
		pidSlice = append(pidSlice, pcb.Pid)
	}
	return pidSlice
}

func enqueueExitProcess(pcb PCB) {
	log.Printf("PID: %d - Estado Anterior: %s - Estado Actual: EXIT", pcb.Pid, pcb.State)
	releaseResources(pcb.Pid)
	deletePagesmemory(pcb.Pid)
	pcb.State = "EXIT"
	mutexExit.Lock()
	<-multiprogrammingChannel
	queueExit = append(queueExit, pcb)
	mutexExit.Unlock()
}

/*------------------------------Manejo de recursos---------------------------------*/

func RecieveWaitFromCPU(w http.ResponseWriter, r *http.Request) {
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
		// le asigna el recurso al pid
		if recursos, exists := global_resourcesPIDMap[request.Pid]; exists {
			// Añadir el nuevo valor al arreglo existente
			global_resourcesPIDMap[request.Pid] = append(recursos, request.Recurso)
		} else {
			// Crear un nuevo arreglo con el valor y asignarlo a la clave
			global_resourcesPIDMap[request.Pid] = []string{request.Recurso}
		}

		// resto 1 si existe
		globals.ClientConfig.InstanciasRecursos[index] -= 1
		//fmt.Println("Instancias recursos: ", globals.ClientConfig.InstanciasRecursos, request.Recurso)

		if globals.ClientConfig.InstanciasRecursos[index] < 0 {
			w.Write([]byte(`{"success": "false"}`))
			return
		}
	} else {
		w.Write([]byte(`{"success": "exit"}`))
		return
	}

	w.Write([]byte(`{"success": "true"}`))
}

func resourceExists(recurso string) (bool, int) {
	for i, r := range globals.ClientConfig.Recursos {
		if r == recurso {
			return true, i
		}
	}
	return false, -1
}

func waitHandler(pcb PCB, recurso string) {
	enqueueBlockedProcess(pcb, recurso)
}

func releaseResource(pid int, recursoAbuscar string) {
	recursos, exists := global_resourcesPIDMap[pid]
	if exists {
		// Buscar el recurso y eliminarlo si existe
		index := -1
		for i, recurso := range recursos {
			if recursoAbuscar == recurso {
				index = i
				break
			}
		}

		if index != -1 {
			// Recurso encontrado, eliminarlo
			recursos = append(recursos[:index], recursos[index+1:]...)
			global_resourcesPIDMap[pid] = recursos
		}
	}
}

func releaseResources(pidFinalizado int) {
	recursos, exists := global_resourcesPIDMap[pidFinalizado]
	if exists {
		for _, recurso := range recursos {
			_, index := resourceExists(recurso)
			globals.ClientConfig.InstanciasRecursos[index]++

			releaseResource(pidFinalizado, recurso)

			if len(queueBlocked[recurso]) > 0 {
				proceso := queueBlocked[recurso][0]
				mutexBlocked.Lock()
				queueBlocked[recurso] = queueBlocked[recurso][1:]
				mutexBlocked.Unlock()
				enqueueReadyProcess(proceso)
			}
		}
	}
}

func RecieveSignalFromCPU(w http.ResponseWriter, r *http.Request) {
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

	recursoExistente, index := resourceExists(recurso)
	if recursoExistente {
		releaseResource(request.Pid, recurso)

		globals.ClientConfig.InstanciasRecursos[index]++
		if len(queueBlocked[recurso]) > 0 {

			waitIfPaused()
			proceso := queueBlocked[recurso][0]
			mutexBlocked.Lock()
			queueBlocked[recurso] = queueBlocked[recurso][1:]
			mutexBlocked.Unlock()

			enqueueReadyProcess(proceso)
		}
	} else {

		w.Write([]byte(`{"success": "exit"}`))
		return
	}

	w.Write([]byte(`{"success": "true"}`))
}

/*------------------------------STDIN & STDOUT & FS---------------------------------*/

func RecieveREGFromCPU(w http.ResponseWriter, r *http.Request) {
	var bodyRegisters BodyRegisters
	err := json.NewDecoder(r.Body).Decode(&bodyRegisters)
	if err != nil {
		http.Error(w, "Error decoding JSON data", http.StatusInternalServerError)
		return
	}
	processData := ProcessData{
		Pid:             bodyRegisters.IOpid,
		LengthREG:       bodyRegisters.LengthREG,
		DireccionFisica: bodyRegisters.DirFisica,
	}
	processDataMap.Store(processData.Pid, processData)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf("Registers received: %v", bodyRegisters)))
}

func RecieveFileNameFromCPU(w http.ResponseWriter, r *http.Request) {
	var fsStructure FSstructure
	err := json.NewDecoder(r.Body).Decode(&fsStructure)
	if err != nil {
		http.Error(w, "Error decoding JSON data", http.StatusInternalServerError)
		return
	}
	fileName = fsStructure.FileName
	fsInstruction = fsStructure.FSInstruction
	fsRegTam = fsStructure.FSRegTam
	fsRegDirec = fsStructure.FSRegDirec
	fsRegPuntero = fsStructure.FSRegPuntero
	//log.Printf("Received filename: %+v", fileName)
	//log.Printf("Received FS instruction: %+v", fsInstruction)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf("Registers received: %v", fileName)))
}

func sendREGtoIO(REGdireccion []int, lengthREG int, port int, pid int) error {
	ioURL := fmt.Sprintf("http://%s:%d/recieveREG", globals.ClientConfig.IpEntradaSalida, port)
	var BodyRegister BodyRegisters
	BodyRegister.DirFisica = REGdireccion
	BodyRegister.LengthREG = lengthREG
	BodyRegister.IOpid = pid

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

func sendFSDataToIO(filename string, instruction string, port int, regTam int, regDirec []int, regPuntero int) error {
	ioURL := fmt.Sprintf("http://%s:%d/recieveFSDATA", globals.ClientConfig.IpEntradaSalida, port)
	fsStructure := FSstructure{
		FileName:      fileName,
		FSInstruction: instruction,
		FSRegTam:      regTam,
		FSRegDirec:    regDirec,
		FSRegPuntero:  regPuntero,
	}

	fsStructureJSON, err := json.Marshal(fsStructure)
	if err != nil {
		return fmt.Errorf("error al serializar los datos JSON: %v", err)
	}

	//log.Println("Enviando solicitud con contenido:", string(fsStructureJSON))

	resp, err := http.Post(ioURL, "application/json", bytes.NewBuffer(fsStructureJSON))
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

/*------------------------------Endpoints Obligatorios---------------------------------*/

func FinishProcess(w http.ResponseWriter, r *http.Request) {
	if r.Method != "DELETE" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

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
	motivo := r.URL.Query().Get("motivo")

	pausePlani()
	// Use pidExists to check if the PID exists in any of the queues
	pcb, err := findPCB(pid)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		resumePlani()
		return
	}
	//log.Printf("Finalizing process %v - Reason: <SUCCESS / INVALID_RESOURCE / INVALID_WRITE> con estado %v", pcb.Pid, pcb.State)

	//Llamar a funcion que finalioza el proceso
	if motivo == "OUT_OF_MEMORY" {
		log.Printf("Finaliza el proceso %v - Motivo: OUT_OF_MEMORY", pcb.Pid)
	} else {
		log.Printf("Finaliza el proceso %v - Motivo: INTERRUPTED_BY_USER", pcb.Pid)
	}
	if pcb.State == "EXEC" {
		interruptToCPU(pcb.Pid, "INTERRUPTED_BY_USER")
	} else {
		unqueuProcess(pcb.Pid)
		enqueueExitProcess(pcb)

	}
	resumePlani()
	w.WriteHeader(http.StatusOK)
}

func findPCB(pid int) (PCB, error) {
	queues := map[string][]PCB{
		"New":       queueNew,
		"Ready":     queueReady,
		"Execution": queueExecution,
		"Exit":      queueExit,
	}

	for state, queue := range queueBlocked {
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

func deletePagesmemory(pid int) {
	memoriaURL := fmt.Sprintf("http://%s:%d/terminateProcess?pid=%d", globals.ClientConfig.IpMemoria, globals.ClientConfig.PuertoMemoria, pid)
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

func GetProcessState(w http.ResponseWriter, r *http.Request) {
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

func RestopPlanification(w http.ResponseWriter, r *http.Request) {
	resumePlani()
	w.WriteHeader(http.StatusOK)
}

func StopPlanification(w http.ResponseWriter, r *http.Request) {
	pausePlani()
	w.WriteHeader(http.StatusOK)
}

func pausePlani() {
	pauseMutex.Lock()
	defer pauseMutex.Unlock()
	if !kernelPaused {
		kernelPaused = true
		close(pauseChannel)
		pauseChannel = make(chan struct{})
	}
}

func resumePlani() {
	pauseMutex.Lock()
	defer pauseMutex.Unlock()
	if kernelPaused {
		kernelPaused = false
		close(resumeChannel)
		resumeChannel = make(chan struct{})
	}
}

func waitIfPaused() {
	pauseMutex.RLock()
	if !kernelPaused {
		pauseMutex.RUnlock()
		return
	}
	pauseMutex.RUnlock()

	<-resumeChannel
}

func findPID(pid int) string {
	queues := map[string][]PCB{
		"New":       queueNew,
		"Ready":     queueReady,
		"Execution": queueExecution,
		"Exit":      queueExit,
	}

	for state, queue := range queueBlocked {
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

func unqueuProcess(pid int) error {
	var findIt = false
	colas := []*[]PCB{&queueNew, &queueReady, &queueReadyVRR}
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
		for key, cola := range queueBlocked {
			for i, proceso := range cola {
				if proceso.Pid == pid {
					// Eliminar el proceso de la cola
					mutexBlocked.Lock()
					queueBlocked[key] = append(cola[:i], cola[i+1:]...)
					mutexBlocked.Unlock()
					//log.Printf("Proceso %v eliminado de la cola: %v", pid, key)
					return nil
				}
			}
		}
	}
	return errors.New("Proceso no encontrado")
}

func ListProcesses(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	queues := map[string][]PCB{
		"New":       queueNew,
		"Ready":     queueReady,
		"Ready+":    queueReadyVRR,
		"Execution": queueExecution,
		"Exit":      queueExit,
	}

	for state, queue := range queueBlocked {
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
