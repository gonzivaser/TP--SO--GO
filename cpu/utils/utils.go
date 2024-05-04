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

	"github.com/sisoputnfrba/tp-golang/cpu/globals"
)

type PruebaMensaje struct {
	Mensaje string `json:"Prueba"`
}

type BodyResponsePCB struct { //ESTO NO VA ACA
	Pcb PCB `json:"pcb"`
}

type BodyResponseInstruction struct {
	Instruction string `json:"instruction"`
}

type PCB struct { //ESTO NO VA ACA
	Pid, ProgramCounte, Quantum int
	CpuReg                      RegisterCPU
}

type ExecutionContext struct {
	Pid, ProgramCounter int
	CpuReg              RegisterCPU
}

type RegisterCPU struct { //ESTO NO VA ACA
	PC, EAX, EBX, ECX, EDX, SI, DI uint32
	AX, BX, CX, DX                 uint8
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

func Prueba(w http.ResponseWriter, r *http.Request) {

	Prueba := PruebaMensaje{
		Mensaje: "Todo OK CPU",
	}

	pruebaResponse, _ := json.Marshal(Prueba)

	w.WriteHeader(http.StatusOK)
	w.Write(pruebaResponse)
}

var programCounter int
var sendPCB ExecutionContext

func ProcessSavedPCBFromKernel(w http.ResponseWriter, r *http.Request) {
	// HAGO UN LOG PARA CHEQUEAR RECEPCION
	log.Printf("Recibiendo solicitud de contexto de ejecucuion desde el kernel")

	// GUARDO PCB RECIBIDO EN sendPCB

	err := json.NewDecoder(r.Body).Decode(&sendPCB)
	if err != nil {
		http.Error(w, "Error al decodificar los datos JSON", http.StatusInternalServerError)
		return
	}

	// HAGO UN LOG PARA CHEQUEAR QUE PASO ERRORES
	log.Printf("PCB recibido desde el kernel: %+v", sendPCB)

	// MANDO EL PC DIRECTAMENTE A MEMORIA
	programCounter = int(sendPCB.CpuReg.PC)
	// for i := 1; i < 5; i++ {
	// 	if err := Fetch(i); err != nil {
	// 		http.Error(w, err.Error(), http.StatusInternalServerError)
	// 		return
	// 	}
	// }

	if err := Fetch(programCounter); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return

	}
	// Convertir el pid PCB recibido de nuevo a JSON para enviarlo en la respuesta
	responseJSON, err := json.Marshal(sendPCB.Pid)
	if err != nil {
		http.Error(w, "Error al serializar el PCB", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(responseJSON)
}

func Fetch(pc int) error {
	memoriaURL := "http://localhost:8085/savePC"

	// CREO VARIABLE DONDE GUARDO EL PROGRAM COUNTER
	pcProcess, err := json.Marshal(pc)

	// CHEQUEO ERRORES
	if err != nil {
		return fmt.Errorf("error al serializar el PCB: %v", err)
	}

	// CONFIRMACION DE QUE PASO ERRORES Y SE MANDA SOLICITUD
	log.Println("Enviando Program Counter con valor:", string(pcProcess))

	// CREO VARIABLE resp y err CON EL
	resp, err := http.Post(memoriaURL, "application/json", bytes.NewBuffer(pcProcess))
	if err != nil {
		return fmt.Errorf("error al enviar la solicitud al módulo de memoria: %v", err)
	}

	// CHEQUEO STATUS CODE CON MI VARIABLE resp
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("error en la respuesta del módulo de memoria: %v", resp.StatusCode)
	}
	var response BodyResponseInstruction
	err = json.NewDecoder(resp.Body).Decode(&response)

	defer resp.Body.Close()
	if err != nil {
		return fmt.Errorf("error al decodificar la respuesta del módulo de memoria: %v", err)
	}
	instuction := strings.Fields(response.Instruction)

	Decode(instuction)

	log.Printf("PCB modificado: %+v", sendPCB)
	//llamada a interpretacion de instruccion
	programCounter += 1

	// SE CHEQUEA CON UN PRINT QUE LA LA MEMORIA RECIBIO CORRECTAMENTE EL pc
	log.Println("Respuesta del módulo de memoria recibida correctamente.")
	return nil
}

func Decode(instuction []string) error {
	switch instuction[0] {
	case "SET": // Change the type of the switch case expression from byte to string
		err := SetCampo(&sendPCB.CpuReg, instuction[1], instuction[2])
		if err != nil {
			return fmt.Errorf("error en la respuesta del módulo de memoria: %s", err)
		}
	case "SUM":
		err := Suma(&sendPCB.CpuReg, instuction[1], instuction[2])
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
