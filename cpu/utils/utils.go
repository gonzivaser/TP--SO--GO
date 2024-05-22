package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"reflect"
	"strconv"
	"strings"

	"github.com/sisoputnfrba/tp-golang/cpu/globals"
)

type PruebaMensaje struct {
	Mensaje string `json:"Prueba"`
}

type BodyResponse struct {
	ExecutionContext ExecutionContext `json:"ExecutionContext"`
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

type RegisterCPU struct {
	PC, EAX, EBX, ECX, EDX, SI, DI uint32
	AX, BX, CX, DX                 uint8
}

type BodyResponseInstruction struct {
	Instruction string `json:"instruction"`
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

func SendResponse(w http.ResponseWriter) {
	BodyResponse := BodyResponse{
		ExecutionContext: receivedPCB,
	}
	response, _ := json.Marshal(BodyResponse)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(response)
}

var receivedPCB ExecutionContext //PCB recibido desde kernel

func ReceivePCB(w http.ResponseWriter, r *http.Request) {

	// HAGO UN LOG PARA CHEQUEAR RECEPCION
	log.Printf("Recibiendo solicitud de contexto de ejecucuion desde el kernel")

	// GUARDO PCB RECIBIDO EN sendPCB

	err := json.NewDecoder(r.Body).Decode(&receivedPCB)
	if err != nil {
		http.Error(w, "Error al decodificar los datos JSON", http.StatusInternalServerError)
		return
	}
	log.Printf("PCB recibido desde el kernel: %+v", receivedPCB)
	InstructionCycle(receivedPCB)
	// HAGO UN LOG PARA CHEQUEAR QUE PASO ERRORES

	w.WriteHeader(http.StatusOK)
}

func InstructionCycle(receivedPCB ExecutionContext) {
	for {
		line, _ := Fetch(int(receivedPCB.CpuReg.PC), receivedPCB.Pid)
		instruction, _ := Decode(line)
		Execute(instruction, line, &receivedPCB) // Pass the address of receivedPCB
		fmt.Println("hola", instruction)

		receivedPCB.CpuReg.PC++
		words := strings.Fields(line[0])
		if words[0] == "EXIT" {
			break
		}
	}
	fmt.Println("el PCB ya actualizado;", receivedPCB)
	fmt.Println("el ax es", receivedPCB.CpuReg.AX)
	fmt.Println("el ax es", receivedPCB.CpuReg.BX)

	fmt.Println("teeermino")

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
	log.Println("Instrucción recibida:", instructions)
	return instructions, nil
}

// 	// // CREO VARIABLE DONDE GUARDO EL PROGRAM COUNTER
// 	// pcProcess, err := json.Marshal(pc)

// 	// // CHEQUEO ERRORES
// 	// if err != nil {
// 	// 	return nil, fmt.Errorf("error al serializar el PCB: %v", err)
// 	// }

// 	// // CONFIRMACION DE QUE PASO ERRORES Y SE MANDA SOLICITUD
// 	// log.Println("Enviando Program Counter con valor:", string(pcProcess))

// 	// // CREO VARIABLE resp y err CON EL
// 	// resp, err := http.Post(memoriaURL, "application/json", bytes.NewBuffer(pcProcess))
// 	// if err != nil {
// 	// 	return nil, fmt.Errorf("error al enviar la solicitud al módulo de memoria: %v", err)
// 	// }

// 	// defer resp.Body.Close()
// 	// if err != nil {
// 	// 	return nil, fmt.Errorf("error al decodificar la respuesta del módulo de memoria: %v", err)
// 	// }
// 	// instruction := strings.Fields(response.Instruction)

// 	// // SE CHEQUEA CON UN PRINT QUE LA LA MEMORIA RECIBIO CORRECTAMENTE EL pc
// 	// log.Println("Respuesta del módulo de memoria recibida correctamente.")
// 	// return instruction, nil

func Decode(instruction []string) (string, error) {
	// Esta función se va a conoplicar con la traducción de las direcciones fisicas y logicas
	if len(instruction) == 0 {
		return "nil", fmt.Errorf("instrucción vacía")
	}
	return instruction[0], nil
}

func Execute(instruction string, line []string, receivedPCB *ExecutionContext) error {

	words := strings.Fields(line[0])

	switch words[0] {
	case "SET": // Change the type of the switch case expression from byte to string
		err := SetCampo(&receivedPCB.CpuReg, words[1], words[2])
		if err != nil {
			return fmt.Errorf("error en la respuesta del módulo de memoria: %s", err)
		}
	case "SUM":
		err := Suma(&receivedPCB.CpuReg, words[1], words[2])
		if err != nil {
			return fmt.Errorf("error en la respuesta del módulo de memoria: %s", err)
		}
	case "SUB":
		err := Resta(&receivedPCB.CpuReg, words[1], words[2])
		if err != nil {
			return fmt.Errorf("error en la respuesta del módulo de memoria: %s", err)
		}

	default:
		fmt.Println("Unknown instruction")
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

func Checkinterrupts(w http.ResponseWriter, r *http.Request) {
	// HAGO UN LOG PARA CHEQUEAR RECEPCION
	log.Printf("Recibiendo solicitud de contexto de ejecucuion desde el kernel")

	err := json.NewDecoder(r.Body).Decode(&receivedPCB)
	if err != nil {
		http.Error(w, "Error al decodificar los datos JSON", http.StatusInternalServerError)
		return
	}
}
