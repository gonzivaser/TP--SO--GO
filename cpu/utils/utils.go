package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/sisoputnfrba/tp-golang/cpu/globals"
)

/*---------------------------------------------- STRUCTS --------------------------------------------------------*/
type KernelRequest struct {
	PcbUpdated     PCB    `json:"pcbUpdated"`
	MotivoDesalojo string `json:"motivoDesalojo"`
	TimeIO         int    `json:"timeIO"`
	Interface      string `json:"interface"`
	IoType         string `json:"ioType"`
}

type PCB struct { //ESTO NO VA ACA
	Pid, Quantum int
	State        string
	CpuReg       RegisterCPU
}

type ExecutionContext struct {
	Pid    int
	State  string
	CpuReg RegisterCPU
}

type RegisterCPU struct {
	PC, EAX, EBX, ECX, EDX, SI, DI uint32
	AX, BX, CX, DX                 uint8
}

type BodyResponseInstruction struct {
	Instruction string `json:"instruction"`
}

type ResponseQuantum struct {
	Interrupt bool `json:"interrupt"`
	Pid       int  `json:"pid"`
}

type TranslationRequest struct {
	DireccionLogica int `json:"logical_address"`
	TamPag          int `json:"page_size"`
	TamData         int `json:"data_size"`
	PID             int `json:"pid"`
}

type TranslationResponse struct {
	DireccionesFisicas []int `json:"physical_addresses"`
}

type TLBEntry struct {
	PID          int
	Pagina       int
	Frame        int
	UltimoAcceso time.Time // Para LRU
	PosicionFila int       // Para FIFO
}

type bodyProcess struct {
	Pid   int `json:"pid"`
	Pages int `json:"pages,omitempty"`
}

type bodyPageTable struct {
	Pid  int `json:"pid"`
	Page int `json:"page"`
}

type bodyRegisters struct {
	DirFisica []int `json:"dirFisica"`
	LengthREG int   `json:"lengthREG"`
}

/*------------------------------------------------- VAR GLOBALES --------------------------------------------------------*/

var tlb []TLBEntry
var tlbSize int
var replacementAlgorithm string
var posicionFila int
var interrupt bool = false
var requestCPU KernelRequest
var responseQuantum ResponseQuantum
var contextoDeEjecucion PCB //PCB recibido desde kernel
var MemoryFrame int

func ConfigurarLogger() {

	logFile, err := os.OpenFile("cpu.log", os.O_CREATE|os.O_APPEND|os.O_RDWR, 0666)
	if err != nil {
		panic(err)
	}
	mw := io.MultiWriter(os.Stdout, logFile)
	log.SetOutput(mw)
}

func init() {
	globals.ClientConfig = IniciarConfiguracion("config.json") // tiene que prender la confi cuando arranca

	if globals.ClientConfig != nil {
		tlbSize = globals.ClientConfig.NumberFellingTLB
		replacementAlgorithm = globals.ClientConfig.AlgorithmTLB
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

func ReceivePCB(w http.ResponseWriter, r *http.Request) {

	// HAGO UN LOG PARA CHEQUEAR RECEPCION
	log.Printf("Recibiendo solicitud de contexto de ejecucuion desde el kernel")

	// GUARDO PCB RECIBIDO EN sendPCB

	err := json.NewDecoder(r.Body).Decode(&contextoDeEjecucion)
	if err != nil {
		http.Error(w, "Error al decodificar los datos JSON", http.StatusInternalServerError)
		return
	}
	log.Printf("PCB recibido desde el kernel: %+v", contextoDeEjecucion)
	InstructionCycle(contextoDeEjecucion)
	w.WriteHeader(http.StatusOK)
}

func InstructionCycle(contextoDeEjecucion PCB) {
	requestCPU = KernelRequest{}

	for {
		log.Printf("PID: %d - FETCH - Program Counter: %d\n", contextoDeEjecucion.Pid, contextoDeEjecucion.CpuReg.PC)
		line, _ := Fetch(int(contextoDeEjecucion.CpuReg.PC), contextoDeEjecucion.Pid)

		contextoDeEjecucion.CpuReg.PC++

		instruction, _ := Decode(line)
		Execute(instruction, line, &contextoDeEjecucion)
		log.Printf("PID: %d - Ejecutando: %s - %s”.", contextoDeEjecucion.Pid, instruction, line)

		if responseQuantum.Interrupt && responseQuantum.Pid == contextoDeEjecucion.Pid || interrupt {
			responseQuantum.Interrupt = false
			interrupt = false
			break
		}
		requestCPU.PcbUpdated = contextoDeEjecucion
	}
	log.Printf("PID: %d - Sale de CPU - PCB actualizado: %d\n", contextoDeEjecucion.Pid, contextoDeEjecucion.CpuReg) //LOG no officia
	if requestCPU.MotivoDesalojo != "FINALIZADO" && requestCPU.MotivoDesalojo != "INTERRUPCION POR IO" {
		requestCPU.MotivoDesalojo = "CLOCK"
	}
	requestCPU.PcbUpdated = contextoDeEjecucion
	responsePCBtoKernel()

}

func responsePCBtoKernel() {
	kernelURL := fmt.Sprintf("http://localhost:%d/syscall", globals.ClientConfig.PortKernel)

	requestJSON, err := json.Marshal(requestCPU)
	if err != nil {
		return
	}
	resp, err := http.Post(kernelURL, "application/json", bytes.NewBuffer(requestJSON))
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return
	}
}

func Fetch(pc int, pid int) ([]string, error) {
	memoriaURL := fmt.Sprintf("http://localhost:8085/getInstructionFromPid?pid=%d&programCounter=%d", pid, pc)
	resp, err := http.Get(memoriaURL)
	if err != nil {
		log.Fatalf("error al enviar la solicitud al módulo de memoria: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("error en la respuesta del módulo de memoria: %v", resp.StatusCode)
		log.Println(err)
		return nil, err
	}

	var response BodyResponseInstruction
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		log.Println("error al decodificar la respuesta del módulo de memoria:", err)
		return nil, err
	}

	instructions := strings.Split(response.Instruction, ",") // split the string into a slice
	return instructions, nil
}

func Decode(instruction []string) (string, error) {
	// Esta función se va a complejizar con la traducción de las direcciones fisicas y logicas
	words := strings.Fields(instruction[0])
	if len(instruction) == 0 {
		return "nil", fmt.Errorf("instrucción vacía")
	}
	return words[0], nil
}

func Execute(instruction string, line []string, contextoDeEjecucion *PCB) error {

	words := strings.Fields(line[0])

	switch instruction {
	case "SET": // Change the type of the switch case expression from byte to string
		err := SetCampo(&contextoDeEjecucion.CpuReg, words[1], words[2])
		if err != nil {
			return fmt.Errorf("error en execute: %s", err)
		}
	case "SUM":
		err := Suma(&contextoDeEjecucion.CpuReg, words[1], words[2])
		if err != nil {
			return fmt.Errorf("error en execute: %s", err)
		}
	case "SUB":
		err := Resta(&contextoDeEjecucion.CpuReg, words[1], words[2])
		if err != nil {
			return fmt.Errorf("error en execute: %s", err)
		}
	case "JNZ":
		err := JNZ(&contextoDeEjecucion.CpuReg, words[1], words[2])
		if err != nil {
			return fmt.Errorf("error en execute: %s", err)
		}
	case "IO_GEN_SLEEP":
		err := IO(instruction, words)
		if err != nil {
			return fmt.Errorf("error en execute: %s", err)
		}
	case "IO_STDIN_READ":
		err := IO(instruction, words)
		if err != nil {
			return fmt.Errorf("error en execute: %s", err)
		}
	case "IO_STDOUT_WRITE":
		err := IO(instruction, words)
		if err != nil {
			return fmt.Errorf("error en execute: %s", err)
		}
	case "RESIZE":
		tam, err := strconv.Atoi(words[1])
		if err != nil {
			return fmt.Errorf("error en execute: %s", err)
		}
		sendResizeMemory(tam)
	case "EXIT":
		requestCPU = KernelRequest{
			MotivoDesalojo: "FINALIZADO",
		}
		interrupt = true // Aquí va el valor booleano que quieres enviar
	default:
		fmt.Println("Instruction no implementada")
	}
	return nil
}

func SetCampo(r *RegisterCPU, campo string, valor interface{}) error {
	// Obtener el valor reflect.Value de la estructura Persona
	valorRef := reflect.ValueOf(r)

	// Obtener el campo especificado por el parámetro campo
	campoRef := valorRef.Elem().FieldByName(campo)

	// Verificar si el campo existe
	if !campoRef.IsValid() {
		return fmt.Errorf("campo '%s' no encontrado en la estructura", campo)
	}

	// Obtener el tipo de dato del campo
	tipoCampo := campoRef.Type()

	// Convertir el valor proporcionado al tipo de dato del campo
	switch tipoCampo.Kind() {
	case reflect.String:
		campoRef.SetString(fmt.Sprintf("%v", valor))
	case reflect.Uint8:
		valorUint, err := strconv.ParseUint(fmt.Sprintf("%v", valor), 10, 8)
		if err != nil {
			return err
		}
		campoRef.SetUint(valorUint)
	case reflect.Uint32:
		valorUint, err := strconv.ParseUint(fmt.Sprintf("%v", valor), 10, 32)
		if err != nil {
			return err
		}
		campoRef.SetUint(valorUint)
	case reflect.Int:
		valorInt, err := strconv.ParseInt(fmt.Sprintf("%v", valor), 10, 64)
		if err != nil {
			return err
		}
		campoRef.SetInt(valorInt)
	// Agrega más casos según sea necesario para otros tipos de datos
	default:
		return fmt.Errorf("tipo de dato del campo '%s' no soportado", tipoCampo)
	}

	return nil
}

func Suma(registerCPU *RegisterCPU, s1, s2 string) error {
	// Suma al Registro Destino el Registro Origen y deja el resultado en el Registro Destino.
	// Los registros pueden ser AX, BX, CX, DX.
	// Los registros son de 8 bits, por lo que el resultado de la suma debe ser truncado a 8 bits.
	// Si el resultado de la suma es mayor a 255, el registro destino debe quedar en 255.
	// Si el resultado de la suma es menor a 0, el registro destino debe quedar en 0.

	// Obtener el valor reflect.Value de la estructura Persona
	valorRef := reflect.ValueOf(registerCPU)

	// Obtener el valor reflect.Value del campo destino
	campoDestinoRef := valorRef.Elem().FieldByName(s1)

	// Verificar si el campo destino existe
	if !campoDestinoRef.IsValid() {
		return fmt.Errorf("campo destino '%s' no encontrado en la estructura", s1)
	}

	// Obtener el tipo de dato del campo destino
	tipoCampoDestino := campoDestinoRef.Type()

	// Obtener el valor reflect.Value del campo origen
	campoOrigenRef := valorRef.Elem().FieldByName(s2)

	// Verificar si el campo origen existe
	if !campoOrigenRef.IsValid() {
		return fmt.Errorf("campo origen '%s' no encontrado en la estructura", s2)
	}

	// Obtener el tipo de dato del campo origen
	tipoCampoOrigen := campoOrigenRef.Type()

	// Verificar que ambos campos sean del mismo tipo
	if tipoCampoDestino != tipoCampoOrigen {
		return fmt.Errorf("los campos '%s' y '%s' no son del mismo tipo", s1, s2)
	}

	// Realizar la suma entre los valores de los campos
	switch tipoCampoDestino.Kind() {
	case reflect.Uint8:
		valorDestino := campoDestinoRef.Uint()
		valorOrigen := campoOrigenRef.Uint()
		suma := valorDestino + valorOrigen

		// Truncar el resultado a 8 bits
		if suma > 255 {
			suma = 255
		}

		// Asignar el resultado de la suma al campo destino
		campoDestinoRef.SetUint(suma)
	}
	return nil
}

func Resta(registerCPU *RegisterCPU, s1, s2 string) error {
	// Suma al Registro Destino el Registro Origen y deja el resultado en el Registro Destino.
	// Los registros pueden ser AX, BX, CX, DX.
	// Los registros son de 8 bits, por lo que el resultado de la suma debe ser truncado a 8 bits.
	// Si el resultado de la suma es mayor a 255, el registro destino debe quedar en 255.
	// Si el resultado de la suma es menor a 0, el registro destino debe quedar en 0.

	// Obtener el valor reflect.Value de la estructura Persona
	valorRef := reflect.ValueOf(registerCPU)

	// Obtener el valor reflect.Value del campo destino
	campoDestinoRef := valorRef.Elem().FieldByName(s1)

	// Verificar si el campo destino existe
	if !campoDestinoRef.IsValid() {
		return fmt.Errorf("campo destino '%s' no encontrado en la estructura", s1)
	}

	// Obtener el tipo de dato del campo destino
	tipoCampoDestino := campoDestinoRef.Type()

	// Obtener el valor reflect.Value del campo origen
	campoOrigenRef := valorRef.Elem().FieldByName(s2)

	// Verificar si el campo origen existe
	if !campoOrigenRef.IsValid() {
		return fmt.Errorf("campo origen '%s' no encontrado en la estructura", s2)
	}

	// Obtener el tipo de dato del campo origen
	tipoCampoOrigen := campoOrigenRef.Type()

	// Verificar que ambos campos sean del mismo tipo
	if tipoCampoDestino != tipoCampoOrigen {
		return fmt.Errorf("los campos '%s' y '%s' no son del mismo tipo", s1, s2)
	}

	// Realizar la suma entre los valores de los campos
	switch tipoCampoDestino.Kind() {
	case reflect.Uint8:
		valorDestino := campoDestinoRef.Uint()
		valorOrigen := campoOrigenRef.Uint()
		resta := valorDestino - valorOrigen

		// Truncar el resultado a 8 bits
		if resta <= 0 {
			resta = 0
		}

		// Asignar el resultado de la resta al campo destino
		campoDestinoRef.SetUint(resta)
	}
	return nil
}

func JNZ(registerCPU *RegisterCPU, reg, valor string) error {
	// Obtener el valor reflect.Value de la estructura RegisterCPU
	valorRef := reflect.ValueOf(registerCPU)

	// Obtener el valor reflect.Value del campo destino
	campoDestinoRef := valorRef.Elem().FieldByName(reg)

	if !campoDestinoRef.IsValid() {
		return fmt.Errorf("campo destino '%s' no encontrado en la estructura", reg)
	}

	// Obtener el valor del campo destino
	campoDestinoValor := campoDestinoRef.Uint()

	if campoDestinoValor != 0 {
		valorUint32, err := strconv.ParseUint(valor, 10, 32)
		if err != nil {
			return err
		}
		registerCPU.PC = uint32(valorUint32)
	}

	return nil
}

func IO(kind string, words []string) error {
	interrupt = true

	switch kind {
	case "IO_GEN_SLEEP":
		timeIO, err := strconv.Atoi(words[2])
		if err != nil {
			return err
		}
		log.Printf("PID IO: %d - %v", contextoDeEjecucion.Pid, contextoDeEjecucion)
		requestCPU = KernelRequest{
			MotivoDesalojo: "INTERRUPCION POR IO",
			IoType:         "IO_GEN_SLEEP",
			Interface:      words[1],
			TimeIO:         timeIO,
		}
	case "IO_STDIN_READ":
		adressREG := words[2]
		valueAdress := verificarRegistro(adressREG)

		lengthREG := words[3]
		valueLength := verificarRegistro(lengthREG)

		direcciones := TranslateAddress(contextoDeEjecucion.Pid, valueAdress, 16, valueLength)
		sendREGtoKernel(direcciones, valueLength)
		requestCPU = KernelRequest{
			MotivoDesalojo: "INTERRUPCION POR IO",
			IoType:         "IO_STDIN_READ",
			Interface:      words[1],
			TimeIO:         0,
		}
	case "IO_STDOUT_WRITE":
		addresREG := words[2]
		valueAdress := verificarRegistro(addresREG)

		lengthREG := words[3]
		valueLength := verificarRegistro(lengthREG) //hay que ver si convertirlo a bytes

		direcciones := TranslateAddress(contextoDeEjecucion.Pid, valueAdress, 16, valueLength) //el 16 está en el config de memoria, hay que ver eso
		sendREGtoKernel(direcciones, valueLength)
		requestCPU = KernelRequest{
			MotivoDesalojo: "INTERRUPCION POR IO",
			IoType:         "IO_STDOUT_WRITE",
			Interface:      words[1],
			TimeIO:         0,
		}
	case "IO_FS_CREATE":
		fmt.Println("IO_FS_CREATE")
	case "IO_FS_DELETE":
		fmt.Println("IO_FS_DELETE")
	case "IO_FS_SEEK":
		fmt.Println("IO_FS_SEEK")
	case "IO_FS_TRUNCATE":
		fmt.Println("IO_FS_TRUNCATE")
	case "IO_FS_WRITE":
		fmt.Println("IO_FS_WRITE")
	case "IO_FS_READ":
		fmt.Println("IO_FS_READ")
	default:
		return fmt.Errorf("tipo de instrucción no soportado")
	}
	return nil
}

func verificarRegistro(registerName string) int {
	fmt.Println(&requestCPU)
	var registerValue int
	switch registerName {
	case "AX":
		registerValue = int(requestCPU.PcbUpdated.CpuReg.PC)
	case "BX":
		registerValue = int(requestCPU.PcbUpdated.CpuReg.BX)
	case "CX":
		registerValue = int(requestCPU.PcbUpdated.CpuReg.CX)
	case "DX":
		registerValue = int(requestCPU.PcbUpdated.CpuReg.DX)
	case "SI":
		registerValue = int(requestCPU.PcbUpdated.CpuReg.SI)
	case "DI":
		registerValue = int(requestCPU.PcbUpdated.CpuReg.DI)
	case "EAX":
		registerValue = int(requestCPU.PcbUpdated.CpuReg.EAX)
	case "EBX":
		registerValue = int(requestCPU.PcbUpdated.CpuReg.EBX)
	case "ECX":
		registerValue = int(requestCPU.PcbUpdated.CpuReg.ECX)
	case "EDX":
		registerValue = int(requestCPU.PcbUpdated.CpuReg.EDX)
	default:
		log.Fatalf("Register %s not found", registerName)
	}
	return registerValue
}

func Checkinterrupts(w http.ResponseWriter, r *http.Request) { // A chequear
	log.Printf("Recibiendo solicitud de Interrupcionde quantum")

	err := json.NewDecoder(r.Body).Decode(&responseQuantum)
	if err != nil {
		http.Error(w, "Error al decodificar los datos JSON", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(interrupt)
}

func CheckTLB(pid, page int) (int, bool) { //Verifica si la etrada ya estaba en la TLB. Si se usa LRU, actualiza el tiempo de acceso
	for i, entry := range tlb {
		if entry.PID == pid && entry.Pagina == page {
			if replacementAlgorithm == "LRU" {
				tlb[i].UltimoAcceso = time.Now()
			}
			return entry.Frame, true
		}
	}
	return -1, false
}

func ReplaceTLBEntry(pid, page, frame int) { //Reemplazo una entrada de TLB según el algoritmo de reemplazo
	newEntry := TLBEntry{
		PID:          pid,
		Pagina:       page,
		Frame:        frame,
		UltimoAcceso: time.Now(),
		PosicionFila: posicionFila,
	}

	if len(tlb) < tlbSize {
		tlb = append(tlb, newEntry) //Si la TLB no está llena, agrego la entrada
	} else {
		if replacementAlgorithm == "FIFO" {
			oldestPos := 0
			for i, entry := range tlb {
				if entry.PosicionFila < tlb[oldestPos].PosicionFila {
					oldestPos = i
				}
			}
			tlb[oldestPos] = newEntry
		} else if replacementAlgorithm == "LRU" {
			oldestPos := 0
			for i, entry := range tlb {
				if entry.UltimoAcceso.Before(tlb[oldestPos].UltimoAcceso) {
					oldestPos = i
				}
			}
			tlb[oldestPos] = newEntry
		}
	}
	posicionFila++
}

func TranslateHandler(w http.ResponseWriter, r *http.Request) {
	var req TranslationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Realizar la traducción
	addresses := TranslateAddress(req.PID, req.DireccionLogica, req.TamPag, req.TamData)

	// Responder con las direcciones físicas
	res := TranslationResponse{DireccionesFisicas: addresses}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
}

// Función de traducción de direcciones
func TranslateAddress(pid, DireccionLogica, TamPag, TamData int) []int {
	var DireccionesFisicas []int

	for offset := 0; offset < TamData; offset += TamPag {
		pageNumber := int(math.Floor(float64(DireccionLogica) / float64(TamPag)))
		pageOffset := DireccionLogica - (pageNumber * TamPag)

		frame, found := CheckTLB(pid, pageNumber)
		if !found {
			fmt.Println("TLB Miss")
			err := FetchFrameFromMemory(pid, pageNumber)
			if err != nil {
				fmt.Println("Error al obtener el marco desde la memoria")
			}
			ReplaceTLBEntry(pid, pageNumber, MemoryFrame) //frame encontrado en memoria con la funcion FetchFrameFromMemory
		} else {
			fmt.Println("TLB Hit")
		}

		physicalAddress := frame*TamPag + pageOffset
		DireccionesFisicas = append(DireccionesFisicas, physicalAddress)
	}
	return DireccionesFisicas
}

// simulacion de la obtención de un marco desde la memoria
func FetchFrameFromMemory(pid, pageNumber int) error {
	memoryURL := fmt.Sprintf("http://localhost:%d/getFramefromCPU", globals.ClientConfig.PortMemory)
	var pageTable bodyPageTable
	pageTable.Pid = pid
	pageTable.Page = pageNumber

	pageTableJSON, err := json.Marshal(pageTable)
	if err != nil {
		log.Fatalf("Error al serializar el Input: %v", err)
	}

	log.Println("Enviando solicitud con contenido:", pageTableJSON)

	resp, err := http.Post(memoryURL, "application/json", bytes.NewBuffer(pageTableJSON))
	if err != nil {
		log.Fatalf("error al enviar la solicitud al módulo de memoria: %v", err)
	}
	defer resp.Body.Close()
	return nil
}

func RecieveFramefromMemory(w http.ResponseWriter, r *http.Request) {
	log.Printf("Recibiendo solicitud de marco desde la memoria")

	var pageTable bodyPageTable
	err := json.NewDecoder(r.Body).Decode(&pageTable)
	if err != nil {
		http.Error(w, "Error al decodificar los datos JSON", http.StatusInternalServerError)
		return
	}
	log.Printf("Marco recibido desde la memoria: %+v", pageTable)

	MemoryFrame = pageTable.Page

	w.WriteHeader(http.StatusOK)
}

func sendResizeMemory(tam int) {
	memoriaURL := fmt.Sprintf("http://localhost:%d//resizeProcess", globals.ClientConfig.PortMemory)
	var process bodyProcess
	process.Pid = contextoDeEjecucion.Pid
	process.Pages = tam

	bodyResizeJSON, err := json.Marshal(process)
	if err != nil {
		log.Fatalf("Error al serializar el Input: %v", err)
	}

	log.Println("Enviando solicitud con contenido:", bodyResizeJSON)
	resp, err := http.Post(memoriaURL, "application/json", bytes.NewBuffer(bodyResizeJSON))
	if err != nil {
		log.Fatalf("error al enviar la solicitud al módulo de memoria: %v", err)
	}
	defer resp.Body.Close()

}

func sendREGtoKernel(addres []int, length int) {
	kernelURL := fmt.Sprintf("http://localhost:%d/recieveREG", globals.ClientConfig.PortKernel)
	var BodyRegisters bodyRegisters
	BodyRegisters.DirFisica = addres
	BodyRegisters.LengthREG = length

	BodyRegistersJSON, err := json.Marshal(BodyRegisters)
	if err != nil {
		log.Fatalf("Error al serializar el Input: %v", err)
	}

	resp, err := http.Post(kernelURL, "application/json", bytes.NewBuffer(BodyRegistersJSON))
	if err != nil {
		log.Fatalf("error al enviar la solicitud al módulo de memoria: %v", err)
	}
	defer resp.Body.Close()
}
