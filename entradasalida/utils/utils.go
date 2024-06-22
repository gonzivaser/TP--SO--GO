package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
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
	Type   string `json:"type"`
}

type BodyRequestRegister struct {
	Length  int   `json:"lengthREG"`
	Address []int `json:"dirFisica"`
	Pid     int   `json:"iopid"`
}

type BodyRequestInput struct {
	Pid     int    `json:"pid"`
	Input   string `json:"input"`
	Address []int  `json:"address"` //Esto viene desde kernel
}

type BodyAdress struct {
	Address []int  `json:"address"`
	Length  int    `json:"length"`
	Name    string `json:"name"`
}

type Finalizado struct {
	Finalizado bool `json:"finalizado"`
}

type BodyRequest struct {
	Instruction string `json:"instruction"`
}

type BodyContent struct {
	Content string `json:"content"`
}

// Estructura para la interfaz genérica
type InterfazIO struct {
	Nombre string         // Nombre único
	Config globals.Config // Configuración
}

type Payload struct {
	IO  int
	Pid int
}

type BlockFile struct {
	Content string
}

/*--------------------------------------------------- VAR GLOBALES ------------------------------------------------------*/

var GLOBALlengthREG int
var GLOBALmemoryContent string
var GLOBALdireccionFisica []int
var GLOBALpid int
var config *globals.Config

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
	pidExecutionProcess := payload.Pid

	interfaceName := os.Args[1]
	log.Printf("Nombre de la interfaz: %s", interfaceName)

	pathToConfig := os.Args[2]
	log.Printf("Path al archivo de configuración: %s", pathToConfig)

	config, err = LoadConfig(pathToConfig)
	if err != nil {
		log.Fatalf("Error al cargar la configuración desde '%s': %v", pathToConfig, err)
	}

	Interfaz := &InterfazIO{
		Nombre: interfaceName,
		Config: *config,
	}

	switch Interfaz.Config.Tipo {
	case "GENERICA":
		log.Printf("PID: %d - Operacion: IO_GEN_SLEEP", pidExecutionProcess)
		duracion := Interfaz.IO_GEN_SLEEP(N)
		log.Printf("La espera por %d unidades para la interfaz '%s' es de %v\n", N, Interfaz.Nombre, duracion)
		time.Sleep(duracion)
		log.Printf("Termino de esperar por la interfaz genérica '%s' es de %v\n", Interfaz.Nombre, duracion)

	case "STDIN":
		log.Printf("PID: %d - Operacion: IO_STDIN_READ", pidExecutionProcess)
		Interfaz.IO_STDIN_READ(GLOBALlengthREG)
		log.Printf("Termino de leer desde la interfaz '%s'\n", Interfaz.Nombre)

	case "STDOUT":
		log.Printf("PID: %d - Operacion: IO_STDOUT_WRITE", pidExecutionProcess)
		Interfaz.IO_STDOUT_WRITE(GLOBALdireccionFisica, GLOBALlengthREG)
		log.Printf("Termino de escribir en la interfaz '%s'\n", Interfaz.Nombre)

	case "DialFS":
		Interfaz.FILE_SYSTEM(N)

	default:
		log.Fatalf("Tipo de interfaz desconocido: %s", Interfaz.Config.Tipo)
	}
}

func SendPortOfInterfaceToKernel(nombreInterfaz string, config *globals.Config) error {
	kernelURL := fmt.Sprintf("http://localhost:%d/SendPortOfInterfaceToKernel", config.PuertoKernel)

	port := BodyRequestPort{
		Nombre: nombreInterfaz,
		Port:   config.Puerto,
		Type:   config.Tipo,
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

func SendAdressSTDOUTToMemory(address BodyAdress) error {
	memoriaURL := "http://localhost:8085/SendAdressSTDOUTToMemory"

	adressResponseTest, err := json.Marshal(address)
	if err != nil {
		log.Fatalf("Error al serializar el address: %v", err)
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

func SendInputSTDINToMemory(input *BodyRequestInput) error {
	memoriaURL := "http://localhost:8085/SendInputSTDINToMemory"

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

func RecieveREG(w http.ResponseWriter, r *http.Request) {
	var requestRegister BodyRequestRegister

	err := json.NewDecoder(r.Body).Decode(&requestRegister)
	if err != nil {
		http.Error(w, "Error decoding JSON data", http.StatusInternalServerError)
		return
	}

	GLOBALlengthREG = requestRegister.Length
	GLOBALdireccionFisica = requestRegister.Address
	GLOBALpid = requestRegister.Pid

	log.Printf("Recieved Register:%v", GLOBALdireccionFisica)
	log.Printf("Received data: %d", GLOBALlengthREG)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf("length received: %d", requestRegister.Length)))
}

func RecieveFileName(w http.ResponseWriter, r *http.Request) {
	var requestFileName string

	err := json.NewDecoder(r.Body).Decode(&requestFileName)
	if err != nil {
		http.Error(w, "Error decoding JSON data", http.StatusInternalServerError)
		return
	}

	log.Printf("Received data: %s", requestFileName)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Content received correctly"))
}

func ReceiveContentFromMemory(w http.ResponseWriter, r *http.Request) {
	var content BodyContent
	err := json.NewDecoder(r.Body).Decode(&content)

	if err != nil {
		http.Error(w, "Error decoding JSON data", http.StatusInternalServerError)
		return
	}

	GLOBALmemoryContent = content.Content
	log.Printf("Received data: %s", GLOBALmemoryContent)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Content received correctly"))
}

/*---------------------------------------------- INTERFACES -------------------------------------------------------*/
// INTERFAZ STDOUT (IO_STDOUT_WRITE)
func (Interfaz *InterfazIO) IO_STDOUT_WRITE(address []int, length int) {
	pathToConfig := os.Args[2]
	log.Printf("Path al archivo de configuración: %s", pathToConfig)

	config, err := LoadConfig(pathToConfig)
	if err != nil {
		log.Fatalf("Error al cargar la configuración desde '%s': %v", pathToConfig, err)
	}

	var Bodyadress BodyAdress

	Bodyadress.Address = address
	Bodyadress.Length = length
	Bodyadress.Name = os.Args[1]

	err1 := SendAdressSTDOUTToMemory(Bodyadress)
	if err1 != nil {
		log.Fatalf("Error al leer desde la memoria: %v", err)
	}

	time.Sleep(time.Duration(config.UnidadDeTiempo) * time.Millisecond)
	// Imprimir el texto en la consola
	fmt.Println(GLOBALmemoryContent)
}

// INTERFAZ STDIN (IO_STDIN_READ)
func (Interfaz *InterfazIO) IO_STDIN_READ(lengthREG int) {
	var BodyInput BodyRequestInput
	var input string

	fmt.Print("Ingrese por teclado: ")
	_, err := fmt.Scanln(&input)
	if err != nil {
		log.Fatalf("Error al leer desde stdin: %v", err)
	}

	if len(input) > lengthREG {
		input = input[:lengthREG]
		log.Println("El texto ingresado es mayor al tamaño del registro, se truncará a: ", input)
	}

	BodyInput.Input = input
	BodyInput.Address = GLOBALdireccionFisica
	BodyInput.Pid = GLOBALpid

	// Guardar el texto en la memoria en la dirección especificada
	err1 := SendInputSTDINToMemory(&BodyInput)
	if err1 != nil {
		log.Fatalf("Error al escribir en la memoria: %v", err)
	}
}

// INTERFAZ GENERICA (IO_GEN_SLEEP)
func (interfaz *InterfazIO) IO_GEN_SLEEP(n int) time.Duration {
	return time.Duration(n*interfaz.Config.UnidadDeTiempo) * time.Millisecond
}

// INTERFAZ FILE SYSTEM
func (interfaz *InterfazIO) FILE_SYSTEM(n int) {
	log.Printf("La interfaz '%s' es de tipo FILE SYSTEM", interfaz.Nombre)

	pathDialFS := interfaz.Config.PathDialFS
	blocksSize := interfaz.Config.TamanioBloqueDialFS
	blocksCount := interfaz.Config.CantidadBloquesDialFS
	sizeFile := blocksSize * blocksCount
	bitmapSize := (blocksCount + 7) / 8

	// CHEQUEO EXISTENCIA DE ARCHIVOS BLOQUES.DAT Y BITMAP.DAT, DE NO SER ASI, LOS CREO
	EnsureIfFileExists(pathDialFS, blocksSize, blocksCount, sizeFile, bitmapSize)

	// COMO YA TENGO MI ARCHIVO DE BLOQUES Y BITMAP, PUEDO PROCEDER A REALIZAR LAS OPERACIONES DE FILE SYSTEM
	/*switch InstruccionFS {
	case "IO_FS_CREATE":
	case "IO_FS_DELETE":
	case "IO_FS_TRUNCATE":
	case "IO_FS_READ":
	}*/

	log.Printf("La duración de la operación de FILE SYSTEM es de %d unidades de tiempo", n)
	time.Sleep(time.Duration(n*interfaz.Config.UnidadDeTiempo) * time.Millisecond)
}

func EnsureIfFileExists(pathDialFS string, blocksSize int, blocksCount int, sizeFile int, bitmapSize int) {
	// Ruta completa para bloques.dat
	blockFilePath := pathDialFS + "/bloques.dat"
	if _, err := os.Stat(blockFilePath); os.IsNotExist(err) {
		log.Printf("El archivo de bloques no existe, creando: %s", blockFilePath)
		CreateBlockFile(pathDialFS, blocksSize, blocksCount, sizeFile)
	} else {
		log.Printf("El archivo de bloques ya existe: %s", blockFilePath)
	}

	// Ruta completa para bitmap.dat
	bitmapFilePath := pathDialFS + "/bitmap.dat"
	if _, err := os.Stat(bitmapFilePath); os.IsNotExist(err) {
		log.Printf("El archivo bitmap no existe, creando: %s", bitmapFilePath)
		CreateBitmapFile(pathDialFS, blocksCount, bitmapSize)
	} else {
		log.Printf("El archivo bitmap ya existe: %s", bitmapFilePath)
	}
}

func CreateBlockFile(path string, blocksSize int, blocksCount int, sizeFile int) {

	filePath := path + "bloques.dat"

	file, err := os.Create(filePath)
	if err != nil {
		log.Fatalf("Error al crear el archivo '%s': %v", path, err)
	}
	defer file.Close()

	// ASIGNO EL TAMAÑO DEL ARCHIVO AL QUE DICE EL CONFIG
	err = file.Truncate(int64(sizeFile))
	if err != nil {
		log.Fatalf("Error al truncar el archivo '%s': %v", path, err)
	}
	_, err = file.WriteString("hola2")
	if err != nil {
		log.Fatalf("Error al escribir en el archivo '%s': %v", filePath, err)
	}
	ShowFileContent(filePath)
}

func ShowFileContent(path string) {
	content, err := ioutil.ReadFile(path)
	if err != nil {
		log.Fatalf("Error al leer el archivo '%s': %v", path, err)
	}
	fmt.Println("Contenido del archivo:")
	fmt.Println(string(content))
}

func CreateBitmapFile(path string, blocksCount int, bitmapSize int) {
	// CADA BIT EN EL BITMAP REPRESENTA UN BLOQUE
	// ENTONCES BLOCKCOUNT == BLOCKCOUNT bits
	// ENTONCES EL TAMAÑO DEL ARCHIVO VA A SER LA CANTIDAD DE BLOQUES (+7 por si division no es exacta) / 8 bytes

	filePath := path + "bitmap.dat"

	// CREO EL ARCHIVO DE BITMAP
	bitmapFile, err := os.Create(filePath)
	if err != nil {
		log.Fatalf("Error al crear el archivo de bitmap '%s.bitmap': %v", path, err)
	}
	defer bitmapFile.Close()

	// INICIALIZO TODOS LOS BITS EN CERO
	bitmap := make([]byte, bitmapSize)
	_, err = bitmapFile.Write(bitmap)
	if err != nil {
		log.Fatalf("Error al inicializar el archivo de bitmap '%s.bitmap': %v", path, err)
	}
}

// INTERFAZ FILE SYSTEM (IO_FS_CREATE)
/*func (interfaz *InterfazIO, nombreArchivo string) IO_FS_CREATE() {
	// RECIBO EL NOMBRE DEL ARCHIVO A CREAR
	// MEDIANTE LA INTERFAZ SELECCIONADA SE CREE UN ARCHIVO EN EL FS, MONTADO EN DICHA INTERFAZ

}*/

// INTERFAZ FILE SYSTEM (IO_FS_DELETE)
/*func (interfaz *InterfazIO, nombreArchivo string) IO_FS_DELETE() {
	// RECIBO EL NOMBRE DEL ARCHIVO A ELIMINAR
	// MEDIANTE LA INTERFAZ SELECCIONADA SE ELIMINE UN ARCHIVO EN EL FS, MONTADO EN DICHA INTERFAZ
}*/

/*
LISTA GLOBAL DE ARCHIVOS ABIERTOS
LISTA GLOBAL DE ARCHIVOS ABIERTOS POR PROCESO
*/
