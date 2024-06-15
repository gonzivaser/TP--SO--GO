package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/sisoputnfrba/tp-golang/cpu/globals"
)

type KernelRequest struct {
	PcbUpdated     ExecutionContext `json:"pcbUpdated"`
	MotivoDesalojo string           `json:"motivoDesalojo"`
	TimeIO         int              `json:"timeIO"`
	Interface      string           `json:"interface"`
	IoType         string           `json:"ioType"`
	Recurso        string           `json:"recurso"`
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

type ResponseInterrupt struct {
	Interrupt bool   `json:"interrupt"`
	Pid       int    `json:"pid"`
	Motivo    string `json:"motivo"`
}

type ResponseWait struct {
	Recurso string `json:"recurso"`
	Pid     int    `json:"pid"`
}

var interrupt bool = false
var requestCPU KernelRequest
var responseInterrupt ResponseInterrupt

func init() {
	globals.ClientConfig = IniciarConfiguracion("config.json") // tiene que prender la confi cuando arranca
}

func ConfigurarLogger() {

	logFile, err := os.OpenFile("cpu.log", os.O_CREATE|os.O_APPEND|os.O_RDWR, 0666)
	if err != nil {
		panic(err)
	}
	mw := io.MultiWriter(os.Stdout, logFile)
	log.SetOutput(mw)
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

var contextoDeEjecucion ExecutionContext //PCB recibido desde kernel

func ReceivePCB(w http.ResponseWriter, r *http.Request) {
	err := json.NewDecoder(r.Body).Decode(&contextoDeEjecucion)
	if err != nil {
		http.Error(w, "Error al decodificar los datos JSON", http.StatusInternalServerError)
		return
	}
	log.Printf("PCB recibido desde el kernel: %+v", contextoDeEjecucion)
	InstructionCycle(contextoDeEjecucion)
	w.WriteHeader(http.StatusOK)
}

func InstructionCycle(contextoDeEjecucion ExecutionContext) {
	requestCPU = KernelRequest{}

	for {
		log.Printf("PID: %d - FETCH - Program Counter: %d\n", contextoDeEjecucion.Pid, contextoDeEjecucion.CpuReg.PC)
		line, _ := Fetch(int(contextoDeEjecucion.CpuReg.PC), contextoDeEjecucion.Pid)

		contextoDeEjecucion.CpuReg.PC++

		instruction, _ := Decode(line)
		time.Sleep(1 * time.Second)
		log.Printf("PID: %d - Ejecutando: %s - %s”.", contextoDeEjecucion.Pid, instruction, line)
		Execute(instruction, line, &contextoDeEjecucion)
		// responseInterrupt.Interrupt ---> ese de clock y finalizacion
		// interrupt ---> ese de io y wait
		if responseInterrupt.Interrupt && responseInterrupt.Pid == contextoDeEjecucion.Pid || interrupt {
			responseInterrupt.Interrupt = false
			interrupt = false
			break
		}

	}
	log.Printf("PID: %d - Sale de CPU - PCB actualizado: %d\n", contextoDeEjecucion.Pid, contextoDeEjecucion.CpuReg) //LOG no official

	if requestCPU.MotivoDesalojo == "" {
		requestCPU.MotivoDesalojo = responseInterrupt.Motivo
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
	memoriaURL := fmt.Sprintf("http://localhost:%d/getInstructionFromPid?pid=%d&programCounter=%d", globals.ClientConfig.PortMemory, pid, pc)
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

func Execute(instruction string, line []string, contextoDeEjecucion *ExecutionContext) error {

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
	case "WAIT":
		err := CheckWait(nil, nil, contextoDeEjecucion, words[1])
		if err != nil {
			return fmt.Errorf("error en execute: %s", err)
		}
	case "SIGNAL":
		err := CheckSignal(nil, nil, contextoDeEjecucion.Pid, instruction, words[1])
		if err != nil {
			return fmt.Errorf("error en execute: %s", err)

		}
	case "EXIT":
		err := TerminarProceso(&contextoDeEjecucion.CpuReg, "FINALIZADO")
		if err != nil {
			return fmt.Errorf("error en execute: %s", err)
		}
	default:
		return nil
	}
	return nil
}

func TerminarProceso(registerCPU *RegisterCPU, motivo string) error {
	requestCPU = KernelRequest{
		MotivoDesalojo: motivo,
	}

	interrupt = true // Aquí va el valor booleano que quieres enviar al kernel
	registerCPU.PC--
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
			IoType:         "GENERICA",
			Interface:      words[1],
			TimeIO:         timeIO,
		}
	case "IO_STDIN_READ":
		fmt.Println("IO_STDOUT_READ")
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

func CheckSignal(w http.ResponseWriter, r *http.Request, pid int, motivo string, recurso string) error {
	log.Printf("Enviando solicitud de Signal al Kernel")

	waitRequest := ResponseWait{
		Recurso: recurso,
		Pid:     pid,
	}

	waitRequestJSON, err := json.Marshal(waitRequest)
	if err != nil {
		http.Error(w, "Error al codificar los datos JSON", http.StatusInternalServerError)
		return err
	}

	kernelURL := fmt.Sprintf("http://localhost:%d/signal", globals.ClientConfig.PortKernel)
	resp, err := http.Post(kernelURL, "application/json", bytes.NewBuffer(waitRequestJSON))
	if err != nil {
		http.Error(w, "Error al enviar la solicitud al kernel", http.StatusInternalServerError)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		http.Error(w, "Error en la respuesta del kernel", http.StatusInternalServerError)
		return err
	}

	var signalResponse struct {
		Success string `json:"success"`
	}

	err = json.NewDecoder(resp.Body).Decode(&signalResponse)
	if err != nil {
		http.Error(w, "Error al decodificar los datos JSON de la respuesta del kernel", http.StatusInternalServerError)
		return err
	}
	log.Printf("Respuesta del kernel: %v", signalResponse)
	if signalResponse.Success == "exit" {
		err := TerminarProceso(&contextoDeEjecucion.CpuReg, "INVALID_RESOURCE")
		if err != nil {
			return fmt.Errorf("error en execute: %s", err)
		}
	}
	return nil
}

func CheckWait(w http.ResponseWriter, r *http.Request, registerCPU *ExecutionContext, recurso string) error {
	log.Printf("Enviando solicitud de Wait al Kernel")

	waitRequest := ResponseWait{
		Recurso: recurso,
		Pid:     registerCPU.Pid,
	}

	waitRequestJSON, err := json.Marshal(waitRequest)
	if err != nil {
		http.Error(w, "Error al codificar los datos JSON", http.StatusInternalServerError)
		return err
	}

	kernelURL := fmt.Sprintf("http://localhost:%d/wait", globals.ClientConfig.PortKernel)
	resp, err := http.Post(kernelURL, "application/json", bytes.NewBuffer(waitRequestJSON))
	if err != nil {
		http.Error(w, "Error al enviar la solicitud al kernel", http.StatusInternalServerError)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		http.Error(w, "Error en la respuesta del kernel", http.StatusInternalServerError)
		return err
	}

	var waitResponse struct {
		Success string `json:"success"`
	}

	err = json.NewDecoder(resp.Body).Decode(&waitResponse)
	if err != nil {
		http.Error(w, "Error al decodificar los datos JSON de la respuesta del kernel", http.StatusInternalServerError)
		return err
	}
	log.Printf("Respuesta del kernel: %v", waitResponse)
	if waitResponse.Success == "false" {
		interrupt = true
		requestCPU = KernelRequest{
			MotivoDesalojo: "WAIT",
			Recurso:        recurso,
		}
	} else if waitResponse.Success == "exit" {
		err := TerminarProceso(&contextoDeEjecucion.CpuReg, "INVALID_RESOURCE")
		if err != nil {
			return fmt.Errorf("error en execute: %s", err)
		}
	}

	return nil
}

func Checkinterrupts(w http.ResponseWriter, r *http.Request) { // A chequear
	log.Printf("Recibiendo solicitud de Interrupcion del Kernel")

	err := json.NewDecoder(r.Body).Decode(&responseInterrupt)
	if err != nil {
		http.Error(w, "Error al decodificar los datos JSON", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(interrupt)
}
