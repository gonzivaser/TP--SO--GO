package utils

//vana  a ir llegando prcesos segun la interfaz. primero chequear que intefaz es de esta froma
// If Int1-->Io_GEN_SLEEP
// If Int2-->Io_STDIN_READ Y ASI....

//YA ESTOY A DENTRO DE LA INTERFAZ
// aca voy a poner un canal a forma de buffer para que vayan llegando los procesos
// voy a ir sacando de a uno y haciendo lo que tenga que hacer la entradasalida
// cuando termino de hacer lo que tenga que hacer la entradasalida, le aviso al kernel que termine
// y saco el proximo proceso del canal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/sisoputnfrba/tp-golang/entradasalida/globals"
)

func ConfigurarLogger() {
	logFile, err := os.OpenFile("entradasalida.log", os.O_CREATE|os.O_APPEND|os.O_RDWR, 0666)
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

/*---------------------------------------------------- STRUCTS ------------------------------------------------------*/
type BodyRequestPort struct {
	Nombre string `json:"nombre"`
	Port   int    `json:"port"`
}

type BodyRequestInput struct {
	Input string `json:"input"`
}

type Adress struct {
	Adress int `json:"adress"`
}

type Finalizado struct {
	Finalizado bool `json:"finalizado"`
}

type BodyRequest struct {
	Instruction string `json:"instruction"`
}

// Estructura para la interfaz genérica
type InterfazIO struct {
	Nombre string         // Nombre único
	Config globals.Config // Configuración
}

type Payload struct {
	IO int
}

/*--------------------------------------------------- VAR GLOBALES ------------------------------------------------------*/

var Puerto int

/*---------------------------------------------------- FUNCIONES ------------------------------------------------------*/

func LoadConfig(filename string) (*globals.Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var config globals.Config
	err = json.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}

func Iniciar(w http.ResponseWriter, r *http.Request) {
	log.Printf("Recibiendo solicitud de I/O desde el kernel")

	var payload Payload
	err := json.NewDecoder(r.Body).Decode(&payload)
	if err != nil {
		http.Error(w, "Error al decodificar los datos JSON", http.StatusInternalServerError)
		return
	}

	N := payload.IO

	interfaceName := os.Args[1]
	log.Printf("Nombre de la interfaz: %s", interfaceName)

	pathToConfig := os.Args[2]
	log.Printf("Path al archivo de configuración: %s", pathToConfig)

	config, err := LoadConfig(pathToConfig)
	if err != nil {
		log.Fatalf("Error al cargar la configuración desde '%s': %v", pathToConfig, err)
	}

	Interfaz := &InterfazIO{
		Nombre: interfaceName,
		Config: *config,
	}

	switch Interfaz.Config.Tipo {
	case "STDIN":
		Interfaz.IO_STDIN_READ()
		log.Printf("Termino de leer desde la interfaz '%s'\n", Interfaz.Nombre)

	case "GENERICA":
		duracion := Interfaz.IO_GEN_SLEEP(N)
		log.Printf("La espera por %d unidades para la interfaz '%s' es de %v\n", N, Interfaz.Nombre, duracion)
		time.Sleep(duracion)
		log.Printf("Termino de esperar por la interfaz genérica '%s' es de %v\n", Interfaz.Nombre, duracion)

	case "STDOUT":
		//Interfaz.IO_STDOUT_WRITE(N)
		log.Printf("Termino de escribir en la interfaz '%s'\n", Interfaz.Nombre)

	default:
		log.Fatalf("Tipo de interfaz desconocido: %s", Interfaz.Config.Tipo)
	}
}

func SendPort(nombreInterfaz string, pathToConfig string) error {
	config, err := LoadConfig(pathToConfig)
	if err != nil {
		log.Fatalf("Error al cargar la configuración desde '%s': %v", pathToConfig, err)
	}
	Puerto = config.Puerto
	PuertoKernel := config.PuertoKernel
	kernelURL := fmt.Sprintf("http://localhost:%d/recievePort", PuertoKernel)

	port := BodyRequestPort{
		Nombre: nombreInterfaz,
		Port:   Puerto,
	}
	portJSON, err := json.Marshal(port)
	if err != nil {
		log.Fatalf("Error al codificar el puerto a JSON: %v", err)
	}

	resp, err := http.Post(kernelURL, "application/json", bytes.NewBuffer(portJSON))
	if err != nil {
		return fmt.Errorf("error al enviar la solicitud al módulo kernel: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("error en la respuesta del módulo kernel: %v", resp.StatusCode)
	}

	log.Println("Respuesta del módulo de kernel recibida correctamente.")
	return nil
}

func IOFinished(w http.ResponseWriter, r *http.Request) {
	kernelURL := "http://localhost:8080/IOFinished"

	finished := Finalizado{
		Finalizado: true,
	}

	finishedResponseTest, err := json.Marshal(finished)
	if err != nil {
		log.Fatalf("Error al serializar el PCB: %v", err)
	}

	log.Println("Enviando solicitud con contenido:", string(finishedResponseTest))

	resp, err := http.Post(kernelURL, "application/json", bytes.NewBuffer(finishedResponseTest))
	if err != nil {
		log.Fatalf("Error al enviar la solicitud al módulo de kernel: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Fatalf("Error en la respuesta del módulo de kernel: %v", resp.StatusCode)
	}
}

func SendAdressSTDOUTToMemory(adress Adress) error {
	memoriaURL := "http://localhost:8085/SendAdressSTDOUTToMemory"

	adressResponseTest, err := json.Marshal(adress)
	if err != nil {
		log.Fatalf("Error al serializar el adress: %v", err)
	}

	log.Println("Enviando solicitud con contenido:", adressResponseTest)

	resp, err := http.Post(memoriaURL, "application/json", bytes.NewBuffer(adressResponseTest))
	if err != nil {
		log.Fatalf("Error al enviar la solicitud al módulo de memoria: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Fatalf("Error en la respuesta del módulo de memoria: %v", resp.StatusCode)
	}

	return nil
}

func SendInputSTDINToMemory(input BodyRequestInput) error {
	memoriaURL := fmt.Sprintf("http://localhost:%d/SendInputSTDINToMemory", globals.ClientConfig.PuertoMemoria)

	inputResponseTest, err := json.Marshal(input)
	if err != nil {
		log.Fatalf("Error al serializar el Input: %v", err)
	}

	log.Println("Enviando solicitud con contenido:", inputResponseTest)

	resp, err := http.Post(memoriaURL, "application/json", bytes.NewBuffer(inputResponseTest))
	if err != nil {
		log.Fatalf("Error al enviar la solicitud al módulo de memoria: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Fatalf("Error en la respuesta del módulo de memoria: %v", resp.StatusCode)
	}

	return nil
}

/*---------------------------------------------- INTERFACES -------------------------------------------------------*/
// INTERFAZ STDOUT (IO_STDOUT_WRITE)
/*func (Interfaz *InterfazIO) IO_STDOUT_WRITE() {
	// IO_STDOUT_WRITE Int3 BX EAX
	// BX: REGISTRO QUE CONTIENE DIRECCION FISICA EN MEMORIA DONDE SE LEERA EL VALOR
	// EAX: REGISTRO QUE VA A CONTENER EL VALOR QUE SE LEA
	var adress Adress

	adress, err := SendAdressSTDOUTToMemory(adress)
	if err != nil {
		log.Fatalf("Error al leer desde la memoria: %v", err)
	}

	// Imprimir el texto en la consola
	fmt.Println(adress)
}*/

// INTERFAZ STDIN (IO_STDIN_READ)
func (Interfaz *InterfazIO) IO_STDIN_READ() {
	/*
		EAX: Registro que contiene la dirección física en memoria donde se almacenará el texto ingresado.
		AX: Registro donde se almacena el valor resultante (puede usarse para alguna otra operación posterior).
	*/
	var BodyInput BodyRequestInput
	var input string

	fmt.Print("Ingrese por teclado: ")
	_, err := fmt.Scanln(&input)
	if err != nil {
		log.Fatalf("Error al leer desde stdin: %v", err)
	}

	BodyInput.Input = input
	// Guardar el texto en la memoria en la dirección especificada
	err = SendInputSTDINToMemory(BodyInput)
	if err != nil {
		log.Fatalf("Error al escribir en la memoria: %v", err)
	}
}

func IO_STDIN_READ() {
	var input BodyRequestInput

	fmt.Print("Ingrese por teclado: ")
	_, err := fmt.Scanln(&input)
	if err != nil {
		log.Fatalf("Error al leer desde stdin: %v", err)
	}

	// Guardar el texto en la memoria en la dirección especificada
	err = SendInputSTDINToMemory(input)
	if err != nil {
		log.Fatalf("Error al escribir en la memoria: %v", err)
	}
}

// INTERFAZ GENERICA (IO_GEN_SLEEP)
func (interfaz *InterfazIO) IO_GEN_SLEEP(n int) time.Duration {
	return time.Duration(n*interfaz.Config.UnidadDeTiempo) * time.Millisecond
}
