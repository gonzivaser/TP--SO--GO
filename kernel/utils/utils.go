package utils

// PONGO ACA ALGUNAS COSAS QUE FALTAN HACER
// fALTA APLICAR CANALES PARA ATENDER LAS io
// fALTA APLICAR EL ALGORITMO DE VRR
// MANEJO DE RECURSOS:
/* A la hora de recibir de la CPU un Contexto de Ejecución desalojado por WAIT, el Kernel deberá verificar primero que exista el recurso
solicitado ("resources") y en caso de que exista restarle 1 a la cantidad de instancias del mismo (""resource_instances").
En caso de que el número sea estrictamente menor a 0, el proceso que realizó WAIT se bloqueará en la cola de
bloqueados correspondiente al recurso.
A la hora de recibir de la CPU un Contexto de Ejecución desalojado por SIGNAL,
el Kernel deberá verificar primero que exista el recurso solicitado, luego sumarle 1 a la cantidad de instancias del mismo.
En caso de que corresponda, desbloquea al primer proceso de la cola de bloqueados de ese recurso. Una vez hecho esto, se devuelve la
ejecución al proceso que peticiona el SIGNAL.
Para las operaciones de WAIT y SIGNAL donde no se cumpla que el recurso exista, se deberá enviar el proceso a EXIT.*/
//MANEJO DE I/O:
/**/
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

type Finalizado struct {
	Finalizado bool `json:"finalizado"`
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
type InterfazIO struct {
	Name string // Nombre interfaz Int1
	Time int    // Configuración 10
}

type Payload struct {
	Nombre string `json:"nombre"`
	IO     int    `json:"io"`
}

type BodyRequestPort struct {
	Nombre string `json:"nombre"`
	Port   int    `json:"port"`
}

type interfaz struct {
	Name string
	Port int
}

var interfaces []interfaz

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
	Interrupt bool `json:"interrupt"`
	PID       int  `json:"pid"`
}

/*---------------------------------------------------VAR GLOBALES------------------------------------------------*/

var (
	ioChannel    chan KernelRequest
	readyChannel chan PCB
	nextPid      = 1
	done         chan struct{}
	//CPURequest   KernelRequest

)

// ----------DECLARACION DE COLAS POR ESTADO----------------
var colaNew []PCB
var colaReady []PCB
var colaReadyVRR []PCB
var colaExecution []PCB
var colaBlocked = make(map[string][]PCB) // Tiene que ser un map string[]PCB[]
var colaExit []PCB

// --------------------------------------------------------
// ----------DECLARACION DE MUTEX POR COLAS DE ESTADO----------------
var mutexNew sync.Mutex
var mutexReady sync.Mutex
var mutexReadyVRR sync.Mutex
var mutexExecution sync.Mutex
var mutexBlocked sync.Mutex
var mutexExit sync.Mutex

// --------------------------------------------------------
// ----------DECLARACION MUTEX MÓDULO----------------
var mutexExecutionCPU sync.Mutex // este mutex es para que no se envie dos procesos al mismo tiempo a la cpu
var mutexExecutionMEMORIA sync.Mutex
var mutexExecutionIO sync.Mutex

var mutexes = make(map[string]*sync.Mutex)

// --------------------------------------------------------

// ----------DECLARACION DE PROCESO EN EJECUCION----------------
var procesoEXEC Proceso // este proceso es el que se esta ejecutando

/*-------------------------------------------------FUNCIONES CREADAS----------------------------------------------*/

func ProcessSyscall(w http.ResponseWriter, r *http.Request) {

	if globals.ClientConfig.AlgoritmoPlanificacion != "FIFO" {
		close(done)
	}
	var CPURequest KernelRequest

	err := json.NewDecoder(r.Body).Decode(&CPURequest)
	if err != nil {
		http.Error(w, "Error al decodificar los datos JSON", http.StatusInternalServerError)
		return
	}
	//log.Printf("Recibido syscall de la CPU: %+v", CPURequest.)

	if len(colaExecution) > 0 { // aca lo saco de la cola exec
		mutexExecution.Lock()
		colaExecution = append(colaExecution[:0], colaExecution[1:]...)
		mutexExecution.Unlock()
	}

	procesoEXEC.PCB.CpuReg = CPURequest.PcbUpdated.CpuReg
	procesoEXEC.PCB.Pid = CPURequest.PcbUpdated.Pid
	switch CPURequest.MotivoDesalojo {
	case "FINALIZADO":
		log.Printf("Finaliza el proceso %v - Motivo: SUCCESS", CPURequest.PcbUpdated.Pid)

		CPURequest.PcbUpdated.State = "EXIT"
		//meter en cola exit
		mutexExit.Lock()
		colaExit = append(colaExit, *procesoEXEC.PCB)
		mutexExit.Unlock()

	case "INTERRUPCION POR IO":
		// aca manejar el handelSyscallIo
		//ioChannel <- CPURequest //meto erl proceso en IO para atender ESTO HAY QUE VERLO
		CPURequest.PcbUpdated.State = "BLOCKED"
		go handleSyscallIO(*procesoEXEC.PCB, CPURequest.TimeIO, CPURequest.Interface)

	case "CLOCK":
		log.Printf("PID: %v desalojado por fin de Quantum", CPURequest.PcbUpdated.Pid)
		go enqueueProcess(*procesoEXEC.PCB)
		CPURequest.PcbUpdated.State = "BLOCKED"
		//actualizo el proceso
		//volver a meter proceso en ready
	case "WAIT":
		go waitHandler(*procesoEXEC.PCB, CPURequest.Recurso)

	default:
		log.Printf("PID: %v desalojado desconocido por %v", CPURequest.PcbUpdated.Pid, CPURequest.MotivoDesalojo)
	}

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

	//log.Printf("Received data: %+v", request)

	// Create PCB
	pcb := createPCB()
	log.Printf("Se crea el proceso %v en NEW", pcb.Pid) // log obligatorio
	response := BodyResponsePid{
		Pid: pcb.Pid,
	}

	IniciarPlanificacionDeProcesos(request, pcb)

	// Response with the PID
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	w.Write([]byte(fmt.Sprintf(`{"pid":%d}`, response.Pid)))
}

func init() {
	globals.ClientConfig = IniciarConfiguracion("config.json") // tiene que prender la confi cuando arranca
	readyChannel = make(chan PCB, 1)
	ioChannel = make(chan KernelRequest)

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

	if len(colaNew) > 0 { // aca lo saco de la cola new y lo mando a ready
		mutexNew.Lock()
		colaNew = append(colaNew[:0], colaNew[1:]...)
		mutexNew.Unlock()
	}

	//meter en ready
	enqueueProcess(*proceso.PCB)
}

func executeTask(proceso PCB) {
	procesoEXEC.PCB = &proceso
	procesoEXEC.PCB.State = "EXEC"
	//sacar de Ready y lo mando a execution
	if len(colaReady) > 0 && procesoEXEC.PCB.Quantum == 0 { // aca lo saco de la cola ready y lo mando a execution
		mutexReady.Lock()
		colaReady = append(colaReady[:0], colaReady[1:]...)
		log.Printf("Cola R desalojada  %+v", colaReady)
		mutexReady.Unlock()
	} else if len(colaReadyVRR) > 0 && procesoEXEC.PCB.Quantum > 0 {
		mutexReadyVRR.Lock()
		colaReadyVRR = append(colaReadyVRR[:0], colaReadyVRR[1:]...)
		log.Printf("Cola VRR desalojada  %+v", colaReadyVRR)
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

	// Verificar si el recurso existe
	recursoExistente := false
	for i, r := range globals.ClientConfig.Recursos {
		if r == request.Recurso {
			recursoExistente = true
			// Restar 1 a las instancias del recurso
			globals.ClientConfig.InstanciasRecursos[i] -= 1
			fmt.Println("Instancias recursos: ", globals.ClientConfig.InstanciasRecursos, request.Recurso)
			// Verificar si el número de instancias es menor a 0
			if globals.ClientConfig.InstanciasRecursos[i] < 0 {
				// Bloquear el proceso en la cola de bloqueados correspondiente al recurso
				// if _, ok := colaBlocked[request.Recurso]; !ok {
				// 	colaBlocked[request.Recurso] = []PCB{}
				// }
				// mutexBlocked.Lock()
				// colaBlocked[request.Recurso] = append(colaBlocked[request.Recurso], getPCBByID(request.Pid))
				// mutexBlocked.Unlock()
				// log.Printf("Proceso %+v bloqueado por recurso %s", getPCBByID(request.Pid), request.Recurso)
				w.Write([]byte(`{"success": "false"}`))
				return
			}
			break
		}
	}
	// Si el recurso no existe, enviar el proceso a EXIT
	if !recursoExistente {
		w.Write([]byte(`{"success": "exit"}`))
	} else {
		// Devolver la ejecución al proceso que peticiona el WAIT
		//log.Printf("Proceso devuelto con true")
		w.Write([]byte(`{"success": "true"}`))
	}
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
	//log.Printf("Recibido signal de la CPU: %+v", request)
	// Verificar si el recurso existe
	recursoExistente := false
	for i, r := range globals.ClientConfig.Recursos {
		if r == recurso {
			recursoExistente = true
			// Sumarle 1 a la cantidad de instancias del recurso
			globals.ClientConfig.InstanciasRecursos[i]++
			if len(colaBlocked[recurso]) > 0 {
				// Desbloquear al primer proceso de la cola de bloqueados
				proceso := colaBlocked[recurso][0]
				mutexBlocked.Lock()
				colaBlocked[recurso] = colaBlocked[recurso][1:]
				mutexBlocked.Unlock()
				enqueueProcess(proceso)
			}
			break
		}
	}
	if !recursoExistente {
		w.Write([]byte(`{"success": "exit"}`))
	} else {
		// Devolver la ejecución al proceso que peticiona el WAIT
		//log.Printf("Proceso devuelto con true")
		w.Write([]byte(`{"success": "true"}`))
	}
}

func handleSyscallIO(pcb PCB, timeIo int, ioInterface string) {

	//proceso := <-ioChannel MIRAR ESTO
	// meter en bloqueado
	log.Printf("PID: %d - Bloqueado por: %s", pcb.Pid, ioInterface)
	mutexBlocked.Lock()
	colaBlocked[ioInterface] = append(colaBlocked[ioInterface], pcb)
	mutexBlocked.Unlock()
	//log.Printf("Proceso %+v desalojado por IO p1. Quantum: %d", pcb, pcb.Quantum)
	mutex, ok := mutexes[ioInterface]
	if !ok {
		mutex = &sync.Mutex{}
		mutexes[ioInterface] = mutex
	}

	mutex.Lock()                               // el 2
	SendIOToEntradaSalida(ioInterface, timeIo) //el 1
	mutex.Unlock()
	//log.Printf("Proceso %+v desalojado por IO p2. Quantum: %d", pcb, pcb.Quantum)

	if len(colaBlocked) > 0 { // aca lo saco de la cola blocked y lo mando a ready
		mutexBlocked.Lock()
		colaBlocked[ioInterface] = append(colaBlocked[ioInterface][:0], colaBlocked[ioInterface][1:]...)
		mutexBlocked.Unlock()

	}
	log.Printf("Proceso %+v volvió de con. Quantum: %d", pcb.Pid, pcb.Quantum)

	enqueueProcess(pcb)
}

func enqueueProcess(pcb PCB) {
	pcb.State = "READY"
	if pcb.Quantum > 0 && globals.ClientConfig.AlgoritmoPlanificacion == "VRR" {
		log.Printf("Proceso %+v con Quantum: %d", pcb.Pid, pcb.Quantum)
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

// func clockHandler(pcb PCB) {
// 	//mutexExecutionCPU.Lock()
// 	mutexReady.Lock()
// 	colaReady = append(colaReady, pcb)
// 	mutexReady.Unlock()
// 	readyChannel <- pcb
// 	//mutexExecutionCPU.Unlock()

// 	//requeueProcess(proceso.PcbUpdated)
// }

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
		proceso = colaReady[0]
		//log.Printf("Proceso recibido de readyChannel: %d", proceso.Pid)
		startQuantum(quantum, &proceso)
		executeTask(proceso)
		//mutexExecutionCPU.Unlock()

	}

}

//var readyQueue = make(chan PCB)

func executeProcessVRR() {
	for {
		mutexExecutionCPU.Lock()
		var proceso PCB
		var quantum int

		proceso = <-readyChannel
		fmt.Printf("Process %d arrived in the ready queue\n", proceso.Pid)
		if len(colaReadyVRR) > 0 {
			proceso = colaReadyVRR[0]
			//log.Printf("Proceso recibido de readyChannelVRR: %d", proceso.Pid)
		} else {
			proceso = colaReady[0]
		}

		if proceso.Quantum > 0 {
			quantum = proceso.Quantum
		} else {
			quantum = globals.ClientConfig.Quantum
		}

		startQuantum(quantum, &proceso)
		executeTask(proceso)
	}
}
func startQuantum(quantum int, proceso *PCB) {
	log.Printf("PID %d - Quantum iniciado %d", proceso.Pid, quantum)

	go func() {
		done = make(chan struct{})
		ticker := time.NewTicker(time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				//log.Printf("PID %d - Quantum restante: %d", proceso.Pid, quantum)
				quantum -= 10
				//log.Printf("PID %d - Quantum restante: %d", proceso.Pid, quantum)
				if quantum == 0 {
					if err := SendInterruptForClock(proceso.Pid); err != nil {
						log.Printf("Error sending interrupt to CPU: %v", err)
					}
					procesoEXEC.PCB.Quantum = quantum
					return
				}
			case <-done:
				procesoEXEC.PCB.Quantum = quantum
				log.Printf("PID %d - Proceso finalizado antes de que el quantum termine. Quantum restante %d", proceso.Pid, quantum)
				return
			}
		}
	}()
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

	//log.Println("Enviando solicitud con contenido:", string(pcbResponseTest))

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

func RecievePort(w http.ResponseWriter, r *http.Request) {
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
	log.Printf("Puerto de Io recibido: %+v", requestPort)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf("Port received: %d", requestPort.Port)))
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

	log.Println("Solicitado la interrupción del módulo CPU.")
	return nil
}

func IOFinished(w http.ResponseWriter, r *http.Request) {
	var finished Finalizado
	err := json.NewDecoder(r.Body).Decode(&finished)

	if err != nil {
		http.Error(w, "Error decoding JSON data", http.StatusInternalServerError)
		return
	}

	log.Printf("Termino IO: %+v", finished)

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

	processState := findPID(pid)

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
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Planificación detenida"))
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
