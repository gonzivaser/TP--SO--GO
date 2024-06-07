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
	IO int `json:"io"`
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
	Interrupt bool `json:"interrupt"`
	PID       int  `json:"pid"`
}

/*---------------------------------------------------VAR GLOBALES------------------------------------------------*/

var (
	// ioChannel       chan KernelRequest
	readyChannel    chan PCB
	readyChannelVRR chan PCB
	nextPid         = 1
	done            chan struct{}
	//CPURequest   KernelRequest
)

// ----------DECLARACION DE COLAS POR ESTADO----------------
var colaNew []PCB
var colaReady []PCB
var colaReadyVRR []PCB
var colaExecution []PCB
var colaBlocked map[string][]PCB // Corregir la declaración del mapa
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
	log.Printf("Recibido syscall de la CPU: %+v", CPURequest)

	procesoEXEC.PCB.CpuReg = CPURequest.PcbUpdated.CpuReg
	procesoEXEC.PCB.Pid = CPURequest.PcbUpdated.Pid
	switch CPURequest.MotivoDesalojo {
	case "FINALIZADO":
		log.Printf("PID: %v finalizado con éxito", CPURequest.PcbUpdated.Pid)
		CPURequest.PcbUpdated.State = "EXIT"
		//meter en cola exit
		mutexExit.Lock()
		colaExit = append(colaExit, *procesoEXEC.PCB)
		mutexExit.Unlock()

	case "INTERRUPCION POR IO":
		// aca manejar el handelSyscallIo
		log.Printf("PID: %v desalojado por IO", CPURequest.PcbUpdated.Pid)
		//ioChannel <- CPURequest //meto erl proceso en IO para atender ESTO HAY QUE VERLO
		go handleSyscallIO(*procesoEXEC.PCB, CPURequest.TimeIO)
		CPURequest.PcbUpdated.State = "BLOCKED"
	case "CLOCK":
		log.Printf("PID: %v desalojado por fin de Quantum", CPURequest.PcbUpdated.Pid)
		go clockHandler(*procesoEXEC.PCB)
		CPURequest.PcbUpdated.State = "BLOCKED"
		//actualizo el proceso
		//volver a meter proceso en ready
	case "WAIT":
		log.Printf("PID: %v desalojado por WAIT", CPURequest.PcbUpdated.Pid)
		go waitHandler(*procesoEXEC.PCB, CPURequest.Recurso)

		CPURequest.PcbUpdated.State = "BLOCKED"
	case "SIGNAL":
		log.Printf("PID: %v desalojado por SIGNAL", CPURequest.PcbUpdated.Pid)
		go handleSignal(*procesoEXEC.PCB, CPURequest.Recurso)
		// aca manejar el handelSyscaSignal

	default:
		log.Printf("PID: %v desalojado desconocido por %v", CPURequest.PcbUpdated.Pid, CPURequest.MotivoDesalojo)
	}

	if len(colaExecution) > 0 { // aca lo saco de la cola exec
		mutexExecution.Lock()
		colaExecution = append(colaExecution[:0], colaExecution[1:]...)
		mutexExecution.Unlock()
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
	readyChannel = make(chan PCB)
	readyChannelVRR = make(chan PCB, 10) // Ajusta el tamaño del buffer según sea necesario, lista por estado
	// ioChannel = make(chan KernelRequest)

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
	mutexReady.Lock()
	colaReady = append(colaReady, *proceso.PCB)
	mutexReady.Unlock()

	readyChannel <- *proceso.PCB
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

func waitHandler(pcb PCB, recurso string) {
	// Verificar si el recurso existe
	recursoExistente := false
	for i, r := range globals.ClientConfig.Recursos {
		if r == recurso {
			recursoExistente = true
			// Restar 1 a las instancias del recurso
			globals.ClientConfig.InstanciasRecursos[i] -= 1
			// Verificar si el número de instancias es menor a 0
			if globals.ClientConfig.InstanciasRecursos[i] < 0 {
				// Bloquear el proceso en la cola de bloqueados correspondiente al recurso
				mutexBlocked.Lock()
				if _, ok := colaBlocked[recurso]; !ok {
					colaBlocked[recurso] = []PCB{}
				}
				colaBlocked[recurso] = append(colaBlocked[recurso], pcb)
				mutexBlocked.Unlock()
				log.Printf("Proceso %+v bloqueado por recurso %s", pcb, recurso)
				return
			}
			break
		}
	}
	// Si el recurso no existe, enviar el proceso a EXIT
	if !recursoExistente {
		mutexExit.Lock()
		colaExit = append(colaExit, pcb)
		mutexExit.Unlock()
		log.Printf("Proceso %+v enviado a EXIT por recurso inexistente: %s", pcb, recurso)
		return
	} else {
		mutexExit.Lock()
		colaReady = append(colaReady, pcb)
		mutexExit.Unlock()
		readyChannel <- pcb
	}
	fmt.Println("Instancias recursos: ", globals.ClientConfig.InstanciasRecursos)
}
func handleSignal(pcb PCB, recurso string) {
	// Verificar si el recurso existe
	recursoExistente := false
	for i, r := range globals.ClientConfig.Recursos {
		if r == recurso {
			recursoExistente = true
			// Sumarle 1 a la cantidad de instancias del recurso
			globals.ClientConfig.InstanciasRecursos[i]++
			break
		}
		//Si el recurso existe, desbloquear al primer proceso de la cola de bloqueados
		if recursoExistente {
			for i, p := range colaBlocked[recurso] {
				if p == pcb {
					// Desbloquear al proceso
					colaBlocked[recurso] = append(colaBlocked[recurso][:i], colaBlocked[recurso][i+1:]...)
					// Devolver la ejecución al proceso que peticiona el SIGNAL
					readyChannel <- pcb
					log.Printf("Proceso %+v desbloqueado por SIGNAL", pcb)
					break
				}
			}
		}
		if !recursoExistente {
			mutexExit.Lock()
			colaExit = append(colaExit, pcb)
			mutexExit.Unlock()
			log.Printf("Proceso %+v enviado a EXIT por recurso inexistente: %s", pcb, recurso)
			return
		} else {
			mutexExit.Lock()
			colaReady = append(colaReady, pcb)
			mutexExit.Unlock()
			readyChannel <- pcb
		}
	}
}

func handleSyscallIO(pcb PCB, timeIo int) {

	//proceso := <-ioChannel MIRAR ESTO
	// meter en bloqueado
	mutexBlocked.Lock()
	colaBlocked["IO"] = append(colaBlocked["IO"], pcb)
	mutexBlocked.Unlock()
	mutexExecutionIO.Lock()
	SendIOToEntradaSalida(timeIo)
	mutexExecutionIO.Unlock()

	if len(colaBlocked) > 0 { // aca lo saco de la cola blocked y lo mando a ready
		mutexBlocked.Lock()
		colaBlocked["IO"] = append(colaBlocked["IO"][:0], colaBlocked["IO"][1:]...)
		mutexBlocked.Unlock()
	}
	log.Printf("Proceso %+v desalojado por IO. Quantum: %d", pcb, pcb.Quantum)
	if pcb.Quantum > 0 && globals.ClientConfig.AlgoritmoPlanificacion == "VRR" {
		mutexReadyVRR.Lock()
		colaReadyVRR = append(colaReadyVRR, pcb)
		mutexReadyVRR.Unlock()
		readyChannelVRR <- pcb
	} else {
		mutexReady.Lock()
		pcb.Quantum = 0
		colaReady = append(colaReady, pcb)
		mutexReady.Unlock()
		readyChannel <- pcb
	}
	//requeueProcess(proceso.PcbUpdated)

}

func clockHandler(pcb PCB) {
	//mutexExecutionCPU.Lock()
	mutexReady.Lock()
	colaReady = append(colaReady, pcb)
	mutexReady.Unlock()
	readyChannel <- pcb
	//mutexExecutionCPU.Unlock()

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
		proceso = colaReady[0]
		log.Printf("Proceso recibido de readyChannel: %d", proceso.Pid)
		startQuantum(quantum, &proceso)
		executeTask(proceso)
		//mutexExecutionCPU.Unlock()

	}

}

func executeProcessVRR() {
	for {
		mutexExecutionCPU.Lock()
		var proceso PCB
		var quantum int
		select {
		case <-readyChannelVRR: // Intenta recibir de readyChannelVRR primero
			proceso = colaReadyVRR[0] // Tomar el primer proceso de readyVRR
		// No es necesario hacer nada aquí, proceso ya tiene el valor
		case <-readyChannel: // Si readyChannelVRR no está listo, intenta recibir de readyChannel
			// Verifica si readyChannelVRR recibió algo mientras tanto
			select {
			case <-readyChannelVRR:
				// Si readyChannelVRR tiene un proceso, lo usa y devuelve el otro proceso a readyChannel
				proceso = colaReadyVRR[0] // Tomar el primer proceso de readyVRR
			default:
				proceso = colaReady[0] // Tomar el primer proceso de ready
			}
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
				log.Printf("PID %d - Proceso finalizado antes de que el quantum termine. Quantum restante %d", proceso.Pid, quantum)
				procesoEXEC.PCB.Quantum = quantum
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

func SendIOToEntradaSalida(io int) error {
	entradasalidaURL := "http://localhost:8090/interfaz"

	payload := Payload{
		IO: io,
	}

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
	log.Printf("PID: <PID> - Desalojado por fin de Quantum")
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
		queues["Blocked"+state] = queue
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
		"Execution": colaExecution,
		"Exit":      colaExit,
	}

	for state, queue := range colaBlocked {
		queues["Blocked"+state] = queue
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
