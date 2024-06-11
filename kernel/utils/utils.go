package utils

// PONGO ACA ALGUNAS COSAS QUE NO SE DONDE PONERLAS
// fALTA APLICAR CANALES PARA ATENDER LAS io PODEMOS DEJARLO COMO ESTA, PERO BUENO
// FALTA PONER LAS COLAS DE BLOCKED, READY Y EXIT (CREO) ya ni me acuerdo cuales puse
import (
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
type InterfazIO struct {
	Name string // Nombre interfaz Int1
	Time int    // Configuración 10
}

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
	PcbUpdated     PCB    `json:"pcbUpdated"`
	MotivoDesalojo string `json:"motivoDesalojo"`
	TimeIO         int    `json:"timeIO"`
	Interface      string `json:"interface"`
	IoType         string `json:"ioType"`
}

type RequestInterrupt struct {
	Interrupt bool `json:"interrupt"`
	PID       int  `json:"pid"`
}

type BodyRequestPort struct {
	Nombre string `json:"nombre"`
	Port   int    `json:"port"`
}
type interfaz struct {
	Name string
	Port int
}

type BodyRegisters struct {
	DirFisica []int `json:"dirFisica"`
	LengthREG int   `json:"lengthREG"`
}

var interfaces []interfaz

/*---------------------------------------------------VAR GLOBALES------------------------------------------------*/

var (
	ioChannel    chan KernelRequest
	readyChannel chan PCB
	nextPid      = 1
	DirFisica    []int
	LengthREG    int
	//CPURequest   KernelRequest

)
var mutexes = make(map[string]*sync.Mutex)

// ----------DECLARACION DE COLAS POR ESTADO----------------
var colaNew []Proceso
var colaReady []Proceso
var colaExecution []Proceso
var colaBlocked []Proceso
var colaExit []Proceso

// --------------------------------------------------------
// ----------DECLARACION DE MUTEX POR COLAS DE ESTADO----------------
var mutexNew sync.Mutex
var mutexReady sync.Mutex
var mutexExecution sync.Mutex
var mutexBlocked sync.Mutex
var mutexExit sync.Mutex

// --------------------------------------------------------
// ----------DECLARACION MUTEX MÓDULO----------------
var mutexExecutionCPU sync.Mutex // este mutex es para que no se envie dos procesos al mismo tiempo a la cpu
var mutexExecutionMEMORIA sync.Mutex
var mutexExecutionIO sync.Mutex

// --------------------------------------------------------

// ----------DECLARACION DE PROCESO EN EJECUCION----------------
var procesoEXEC Proceso // este proceso es el que se esta ejecutando

/*-------------------------------------------------FUNCIONES CREADAS----------------------------------------------*/

func ProcessSyscall(w http.ResponseWriter, r *http.Request) {
	// CREO VARIABLE I/O
	var CPURequest KernelRequest

	err := json.NewDecoder(r.Body).Decode(&CPURequest)
	if err != nil {
		http.Error(w, "Error al decodificar los datos JSON", http.StatusInternalServerError)
		return
	}
	log.Printf("Recibido syscall: %+v", CPURequest)
	switch CPURequest.MotivoDesalojo {
	case "FINALIZADO":
		log.Printf("Finaliza el proceso %v - Motivo: <SUCCESS>", CPURequest.PcbUpdated.Pid)
		CPURequest.PcbUpdated.State = "EXIT"
		//meter en cola exit
	case "INTERRUPCION POR IO":
		// aca manejar el handelSyscallIo
		ioChannel <- CPURequest //meto erl proceso en IO para atender ESTO HAY QUE VERLO
		go handleSyscallIO(CPURequest)
		CPURequest.PcbUpdated.State = "BLOCKED"
	case "CLOCK":
		log.Printf("Proceso %v desalojado por fin de Quantum", CPURequest.PcbUpdated.Pid)
		go clockHandler(CPURequest)
		//actualizo el proceso
		//volver a meter proceso en ready

	default:
		log.Printf("Proceso %v desalojado desconocido por %v", CPURequest.PcbUpdated.Pid, CPURequest.MotivoDesalojo)
	}

	log.Printf("Recibido pcb: %v", CPURequest.PcbUpdated)

	if len(colaExecution) > 0 { // aca lo saco de la cola exec
		mutexExecution.Lock()
		colaExecution = append(colaExecution[:0], colaExecution[1:]...)
		mutexExecution.Unlock()
	}

	//actualizo despues de que vuelva de la cpu
	procesoEXEC.PCB.State = CPURequest.PcbUpdated.State
	procesoEXEC.PCB.CpuReg = CPURequest.PcbUpdated.CpuReg
	procesoEXEC.PCB.Pid = CPURequest.PcbUpdated.Pid

	mutexExecutionCPU.Unlock()

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf("%v", CPURequest.PcbUpdated)))

}

func IniciarProceso(w http.ResponseWriter, r *http.Request) {
	var request BodyRequest
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		http.Error(w, "Error decoding JSON data", http.StatusInternalServerError)
		return
	}

	log.Printf("Received data: %+v", request)

	// Create PCB
	pcb := createPCB()
	log.Printf("Se crea el proceso %v en NEW", pcb.Pid) // log obligatorio

	IniciarPlanificacionDeProcesos(request, pcb)

	// Response with the PID
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf(`{"pid":%d}`, pcb.Pid)))
}

func init() {
	globals.ClientConfig = IniciarConfiguracion("config.json") // tiene que prender la confi cuando arranca
	readyChannel = make(chan PCB)                              // Ajusta el tamaño del buffer según sea necesario, lista por estado
	ioChannel = make(chan KernelRequest)

	if globals.ClientConfig != nil {
		if globals.ClientConfig.AlgoritmoPlanificacion == "FIFO" {
			go executeProcessFIFO()
		} else if globals.ClientConfig.AlgoritmoPlanificacion == "RR" {
			go executeProcessRR(globals.ClientConfig.Quantum)
		}
	} else {
		log.Fatal("ClientConfig is not initialized")
	}
	//go executeProcessFIFO()
}

func IniciarPlanificacionDeProcesos(request BodyRequest, pcb PCB) {
	proceso := Proceso{
		Request: request,
		PCB:     &pcb,
	}
	mutexNew.Lock()
	colaNew = append(colaNew, proceso)
	mutexNew.Unlock()

	mutexExecutionMEMORIA.Lock()
	if err := SendPathToMemory(proceso.Request, proceso.PCB.Pid); err != nil {
		log.Printf("Error sending path to memory: %v", err)
		return
	}
	mutexExecutionMEMORIA.Unlock()

	if len(colaNew) > 0 { // aca lo saco de la cola new y lo mando a ready
		mutexNew.Lock()
		colaNew = append(colaNew[:0], colaNew[1:]...)
		mutexNew.Unlock()
	}

	//meter en ready
	mutexReady.Lock()
	colaReady = append(colaReady, proceso)
	mutexReady.Unlock()

	readyChannel <- *proceso.PCB
}

func executeTask(proceso PCB) {
	procesoEXEC.PCB = &proceso
	//sacar de Ready y lo mando a execution
	if len(colaReady) > 0 { // aca lo saco de la cola new y lo mando a ready
		mutexReady.Lock()
		colaReady = append(colaReady[:0], colaReady[1:]...)
		mutexReady.Unlock()
	}
	//meter en execution
	mutexExecution.Lock()
	colaExecution = append(colaExecution, procesoEXEC)
	mutexExecution.Unlock()

	if err := SendContextToCPU(*procesoEXEC.PCB); err != nil {
		log.Printf("Error sending context to CPU: %v", err)
		return
	}
}

/*func requeueProcess(proceso PCB) {
	mu.Lock()
	defer mu.Unlock()
	readyChannel <- proceso
}*/

/*func handleIOQueue() {
	for proceso := range ioChannel {
		go handleSyscallIO(proceso)

	}
}*/

func handleSyscallIO(proceso KernelRequest) {

	//proceso := <-ioChannel MIRAR ESTO
	// meter en bloqueado
	mutex, ok := mutexes[proceso.Interface]
	if !ok {
		mutex = &sync.Mutex{}
		mutexes[proceso.Interface] = mutex
	}

	mutex.Lock()
	SendIOToEntradaSalida(proceso.Interface, proceso.TimeIO)
	mutex.Unlock()
	readyChannel <- proceso.PcbUpdated
	//requeueProcess(proceso.PcbUpdated)
}

func clockHandler(proceso KernelRequest) {
	mutexExecutionCPU.Lock()
	readyChannel <- proceso.PcbUpdated
	//requeueProcess(proceso.PcbUpdated)
}

func executeProcessFIFO() {
	// infinitamente estar sacando el primero de taskque ---> readyqueue
	for {
		//mutex para no enviar dos procesos al mismo timepo a cpu
		mutexExecutionCPU.Lock()
		proceso := <-readyChannel
		executeTask(proceso)

	}

}

func executeProcessRR(quantum int) {

	for {
		mutexExecutionCPU.Lock()
		proceso := <-readyChannel
		startQuantum(quantum, proceso)
		executeTask(proceso)

	}

}

func startQuantum(quantum int, proceso PCB) {
	log.Printf("PID %d - Quantum iniciado", proceso.Pid)
	go func() {
		done := make(chan struct{})
		select {
		case <-time.After(time.Duration(quantum) * time.Millisecond):
			if err := SendInterruptForClock(proceso.Pid); err != nil {
				log.Printf("Error sending interrupt to CPU: %v", err)
			}

		case <-done:
			log.Printf("PID %d - Proceso finalizado antes de que el quantum termine", proceso.Pid)
		}
	}()

}

func createPCB() PCB {
	nextPid++

	return PCB{
		Pid: nextPid - 1, // ASIGNO EL VALOR ANTERIOR AL pid

		Quantum: 0,
		State:   "READY",

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

	log.Println("Enviando solicitud con contenido:", string(savedPathJSON))

	resp, err := http.Post(memoriaURL, "application/json", bytes.NewBuffer(savedPathJSON))
	if err != nil {
		return fmt.Errorf("error al enviar la solicitud al módulo de memoria: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("error en la respuesta del módulo de memoria: %v", resp.StatusCode)
	}

	log.Println("Respuesta del módulo de memoria recibida correctamente.")
	return nil
}

func SendContextToCPU(pcb PCB) error {
	cpuURL := "http://localhost:8075/receivePCB"

	context := pcb
	pcbResponseTest, err := json.Marshal(context)
	if err != nil {
		return fmt.Errorf("error al serializar el PCB: %v", err)
	}

	log.Println("Enviando solicitud con contenido:", string(pcbResponseTest))

	resp, err := http.Post(cpuURL, "application/json", bytes.NewBuffer(pcbResponseTest))
	if err != nil {
		return fmt.Errorf("error al enviar la solicitud al módulo de cpu: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("error en la respuesta del módulo de cpu: %v", resp.StatusCode)
	}

	log.Println("Respuesta del módulo de cpu recibida correctamente.")
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

	interfaces = append(interfaces, interfaz)
	log.Printf("Received data: %+v", requestPort)

	SendPortOfInterfaceToMemory(interfaz.Name, interfaz.Port)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf("Port received: %d", requestPort.Port)))
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
		return fmt.Errorf("error en la respuesta del módulo de memoria: %v", resp.StatusCode)
	}

	log.Println("Respuesta del módulo de memoria recibida correctamente.")
	return nil
}

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
	ioURL := fmt.Sprintf("http://localhost:%d/ReceiveREGFromCPU", port)
	body := BodyRegisters{
		DirFisica: REGdireccion,
		LengthREG: lengthREG,
	}
	savedRegJSON, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("error al serializar los datos JSON: %v", err)
	}

	log.Println("Enviando solicitud con contenido:", string(savedRegJSON))

	resp, err := http.Post(ioURL, "application/json", bytes.NewBuffer(savedRegJSON))
	if err != nil {
		return fmt.Errorf("error al enviar la solicitud al módulo de entradasalida: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("error en la respuesta del módulo de entradasalida: %v", resp.StatusCode)
	}

	log.Println("Respuesta del módulo de entradasalida recibida correctamente.")
	return nil
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
	if interfazEncontrada != (interfaz{}) {
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

		log.Println("Respuesta del módulo de IO recibida correctamente.")
		return nil
	}
	return nil
}

func SendInterruptForClock(pid int) error {
	cpuURL := "http://localhost:8075/interrupt"

	RequestInterrupt := RequestInterrupt{
		Interrupt: true,
		PID:       pid,
	}

	hayQuantumBytes, err := json.Marshal(RequestInterrupt)
	if err != nil {
		log.Printf("Error al serializar el valor de hayQuantum: %v", err)
		return err
	}

	resp, err := http.Post(cpuURL, "application/json", bytes.NewBuffer(hayQuantumBytes))
	if err != nil {
		log.Printf("Error al enviar la solicitud al módulo de cpu: %v", err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Error en la respuesta del módulo de cpu: %v", resp.StatusCode)
	}

	log.Println("Respuesta del módulo de cpu recibida correctamente.")
	return nil
}

func IOFinished(w http.ResponseWriter, r *http.Request) {
	var finished Finalizado
	err := json.NewDecoder(r.Body).Decode(&finished)

	if err != nil {
		http.Error(w, "Error decoding JSON data", http.StatusInternalServerError)
		return
	}

	log.Printf("Termino: %+v", finished)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf("Termino: %+v", finished)))
}

/*---------------------------------------------FUNCIONES OBLIGATORIAS--------------------------------------------------*/

func FinalizarProceso(w http.ResponseWriter, r *http.Request) {
	pid := r.URL.Query().Get("pid")
	if pid == "" {
		http.Error(w, "PID no especificado", http.StatusBadRequest)
		return
	}

	log.Printf("Finaliza el proceso %s - Motivo: <SUCCESS / INVALID_RESOURCE / INVALID_WRITE>", pid)

	respuestaOK := fmt.Sprintf("Proceso finalizado: %s", pid)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(respuestaOK))
}

func EstadoProceso(w http.ResponseWriter, r *http.Request) {
	pidStr := r.URL.Query().Get("pid")
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

	var processState string
	for _, process := range colaReady {
		if process.PCB.Pid == pid {
			processState = process.PCB.State
			break
		}
	}

	BodyResponse := BodyResponseState{
		State: processState,
	}

	stateResponse, _ := json.Marshal(BodyResponse)

	w.WriteHeader(http.StatusOK)
	w.Write(stateResponse)
}

func IniciarPlanificacion(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Planificación iniciada"))
}

func DetenerPlanificacion(w http.ResponseWriter, r *http.Request) {
	log.Printf("PID: <PID> - Desalojado por fin de Quantum")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Planificación detenida"))
}

func ListarProcesos(w http.ResponseWriter, r *http.Request) {
	var pids []int
	for _, process := range colaReady {
		pids = append(pids, process.PCB.Pid)
	}

	pidsJSON, err := json.Marshal(pids)
	if err != nil {
		http.Error(w, "Error al convertir colaReady a JSON", http.StatusInternalServerError)
		return
	}

	log.Printf("Cola Ready COLA: %v", pids)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(pidsJSON)
}

func ConfigurarLogger() {
	logFile, err := os.OpenFile("kernel.log", os.O_CREATE|os.O_APPEND|os.O_RDWR, 0666)
	if err != nil {
		panic(err)
	}
	mw := io.MultiWriter(os.Stdout, logFile)
	log.SetOutput(mw)
}
