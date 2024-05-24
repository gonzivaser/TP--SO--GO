package utils

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

type BodyResponsePCB struct { //ESTO NO VA ACA
	Pcb PCB `json:"pcb"`
}

type PCB struct { //ESTO NO VA ACA
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

type RegisterCPU struct { //ESTO NO VA ACA
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
}

/*---------------------------------------------------VAR GLOBALES------------------------------------------------*/

var nextPid = 1
var timeIOGlobal int
var CPURequest KernelRequest
var (
	colaReady []Proceso
	mu        sync.Mutex
	muio      sync.Mutex
)
var syscallIO bool
var cond = sync.NewCond(&mu)
var quantum = globals.ClientConfig.Quantum

/*-------------------------------------------------FUNCIONES CREADAS----------------------------------------------*/

func ProcessSyscall(w http.ResponseWriter, r *http.Request) {
	log.Printf("Recibiendo solicitud de I/O desde el cpu")

	// CREO VARIABLE I/O

	err := json.NewDecoder(r.Body).Decode(&CPURequest)
	if err != nil {
		http.Error(w, "Error al decodificar los datos JSON", http.StatusInternalServerError)
		return
	}
	log.Printf("Recibido syscall: %+v", CPURequest)
	switch CPURequest.MotivoDesalojo {
	case "FINALIZADO":
		log.Printf("Proceso %v finalizado con éxito", CPURequest.PcbUpdated.Pid)
		CPURequest.PcbUpdated.State = "EXIT"
	case "INTERRUPCION POR IO":
		syscallIO = true
		CPURequest.PcbUpdated.State = "BLOCKED"
		timeIOGlobal = CPURequest.TimeIO
	default:
		log.Printf("Proceso %v desalojado desconocido por %v", CPURequest.PcbUpdated.Pid, CPURequest.MotivoDesalojo)
	}

	// enviar I/O a entradasalida
	// HAGO UN LOG SI PASO ERRORES PARA RECEPCION DEL I/O

	log.Printf("Recibido pcb: %v", CPURequest.PcbUpdated)

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
}

func IniciarPlanificacionDeProcesos(request BodyRequest, pcb PCB) {
	// Create a new process and add it to the queue
	proceso := Proceso{
		Request: request,
		PCB:     &pcb,
	}

	mu.Lock()
	colaReady = append(colaReady, proceso)
	if err := SendPathToMemory(proceso.Request, proceso.PCB.Pid); err != nil {
		log.Printf("Error sending path to memory: %v", err)

		return
	}
	mu.Unlock()

	if globals.ClientConfig.AlgoritmoPlanificacion == "FIFO" {
		go executeProcessFIFO()
	} else if globals.ClientConfig.AlgoritmoPlanificacion == "RR" {
		go executeProcessRR()
	}
}

func executeProcessRR() {
	for {
		mu.Lock()
		for len(colaReady) == 0 {
			cond.Wait()
		}

		// Dequeue a process from colaReady
		log.Printf("hola a amover la cola: %v", colaReady[0].PCB.Pid)
		proceso := colaReady[0]
		colaReady = colaReady[1:]
		mu.Unlock()

		go func(proceso Proceso) {
			// Execute the process
			mu.Lock()
			if err := SendContextToCPU(*proceso.PCB); err != nil {
				log.Printf("Error sending context to CPU: %v", err)
				return
			}
			mu.Unlock()

			proceso.PCB.CpuReg = newPCB.PcbUpdated.CpuReg
			proceso.PCB.State = newPCB.PcbUpdated.State
			proceso.PCB.Pid = newPCB.PcbUpdated.Pid

			executionTime := time.NewTimer(time.Duration(quantum) * time.Second)
			defer executionTime.Stop()

			select {
			// SI EL TEMPORIZADOR TERMINA ANTES DE QUE EL PROCESO TERMINE SE MANDA INTERRUPCION DE CLOCK
			case <-executionTime.C:
				log.Printf("Quantum expirado para el proceso %v", proceso.PCB.Pid)
				SendInterruptForClock()

				// SI EL PROCESO TERMINA ANTES DE QUE EL TEMPORIZADOR TERMINE DIRECAMENTE SE TERMINA EL PROCESO
			default:
				if proceso.PCB.State == "EXIT" {
					if !executionTime.Stop() {
						<-executionTime.C
					}
					log.Printf("Proceso %v terminó antes de que expire el quantum", proceso.PCB.Pid)
				}
			}

			if syscallIO {
				muio.Lock()
				if err := SendIOToEntradaSalida(timeIOGlobal); err != nil {
					log.Printf("Error sending IO to EntradaSalida: %v", err)
				}
				muio.Unlock()
				syscallIO = false
				if proceso.PCB.State != "EXIT" {
					proceso.PCB.State = "READY"
				}
			}

			if proceso.PCB.State == "READY" {
				colaReady = append(colaReady, proceso)
				cond.Signal() // Notify that colaReady is not empty
			}
		}(proceso)
	}
}

func executeProcessFIFO() {
	for {
		mu.Lock()
		for len(colaReady) == 0 {
			cond.Wait()
		}

		// Dequeue a process from colaReady
		log.Printf("hola a amover la cola: %v", colaReady[0].PCB.Pid)
		proceso := colaReady[0]
		colaReady = colaReady[1:]
		mu.Unlock()

		go func(proceso Proceso) {
			// Execute the process
			mu.Lock()
			if err := SendContextToCPU(*proceso.PCB); err != nil {
				log.Printf("Error sending context to CPU: %v", err)
				return
			}
			mu.Unlock()

			log.Printf("PID: %d - Estado Anterior: %s - Estado Actual: %s", proceso.PCB.Pid, proceso.PCB.State, CPURequest.PcbUpdated.State)

			proceso.PCB.CpuReg = CPURequest.PcbUpdated.CpuReg
			proceso.PCB.State = CPURequest.PcbUpdated.State
			proceso.PCB.Pid = CPURequest.PcbUpdated.Pid // Dudoso

			if syscallIO {
				muio.Lock()
				if err := SendIOToEntradaSalida(timeIOGlobal); err != nil {
					log.Printf("Error sending IO to EntradaSalida: %v", err)
				}
				muio.Unlock()
				syscallIO = false
				if proceso.PCB.State != "EXIT" {
					proceso.PCB.State = "READY"
				}
			}

			if proceso.PCB.State == "READY" {
				colaReady = append(colaReady, proceso)
				cond.Signal() // Notify that colaReady is not empty
			}
		}(proceso)
	}
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

	// CREO EL CONTEXTO DE EJECUCION -> OSEA LOS DATOS DEL PCB QUE VA A NECESITAR LA CPU PARA EL MOMENTO DE EJECUCION
	context := ExecutionContext{
		Pid:    pcb.Pid,
		State:  pcb.State,
		CpuReg: pcb.CpuReg,
	}
	pcbResponseTest, err := json.Marshal(context)

	// CHEQUEO ERRORES
	if err != nil {
		return fmt.Errorf("error al serializar el PCB: %v", err)
	}

	// CONFIRMACION DE QUE PASO ERRORES Y SE MANDA SOLICITUD
	log.Println("Enviando solicitud con contenido:", string(pcbResponseTest))

	// CREO VARIABLE resp y err CON EL
	resp, err := http.Post(cpuURL, "application/json", bytes.NewBuffer(pcbResponseTest))
	if err != nil {
		return fmt.Errorf("error al enviar la solicitud al módulo de cpu: %v", err)
	}
	defer resp.Body.Close()

	// CHEQUEO STATUS CODE CON MI VARIABLE resp
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("error en la respuesta del módulo de cpu: %v", resp.StatusCode)
	}

	// SE CHEQUEA CON UN PRINT QUE LA CPU RECIBIO CORRECTAMENTE EL PCB
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

func SendInterruptForClock() error {
	cpuURL := "http://localhost:8075/receiveInterrupt"

	resp, err := http.Post(cpuURL, "application/json", nil)
	if err != nil {
		log.Printf("Error al enviar la solicitud al módulo de cpu: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Error en la respuesta del módulo de cpu: %v", resp.StatusCode)
	}

	log.Println("Respuesta del módulo de cpu recibida correctamente.")
	return nil
}

/*---------------------------------------------LOGS OBLIGATORIOS--------------------------------------------------*/

func FinalizarProceso(w http.ResponseWriter, r *http.Request) {

	pid := r.PathValue("pid")

	log.Printf("Finaliza el proceso %s - Motivo: <SUCCESS / INVALID_RESOURCE / INVALID_WRITE>", pid)

	respuestaOK := fmt.Sprintf("Proceso finalizado:%s", pid)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(respuestaOK))
}

func EstadoProceso(w http.ResponseWriter, r *http.Request) {
	pidStr := r.PathValue("pid")
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		// handle error
		log.Printf("Error converting pid to integer: %v", err)
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

	//log.Printf("PID: %s - Estado Anterior: <ESTADO_ANTERIOR> - Estado Actual: %v", pid, BodyResponse.State) // A checkear

	w.WriteHeader(http.StatusOK)
	w.Write(stateResponse)
}

func IniciarPlanificacion(w http.ResponseWriter, r *http.Request) {
	//log.Printf("PID: <PID> - Bloqueado por: <INTERFAZ / NOMBRE_RECURSO>") //ESTO NO VA ACA
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Planificación iniciada"))
}

func DetenerPlanificacion(w http.ResponseWriter, r *http.Request) {
	log.Printf("PID: <PID> - Desalojado por fin de Quantum") //ESTO NO VA ACA
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Planificación detenida"))
}

func ListarProcesos(w http.ResponseWriter, r *http.Request) {
	// Convert colaReady array to JSON
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

	// Write the JSON response
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
