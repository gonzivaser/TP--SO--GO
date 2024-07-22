package utils

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"

	"path/filepath"

	"strings"
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

// datos unicos de cada proceso
type ProcessData struct {
	Pid             int
	LengthREG       int
	DireccionFisica []int
}

var processDataMap sync.Map

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

type MemoryRequest struct {
	PID     int    `json:"pid"`
	Address []int  `json:"address"`
	Size    int    `json:"size,omitempty"` //Si es 0, se omite (Util para creacion y terminacion de procesos)
	Data    []byte `json:"data,omitempty"` //Si es 0, se omite Util para creacion y terminacion de procesos)
	Type    string `json:"type"`           //Si es 0, se omite Util para creacion y terminacion de procesos)
	Port    int    `json:"port,omitempty"`
}

type FSstructure struct {
	FileName      string `json:"filename"`
	FSInstruction string `json:"fsinstruction"`
	FSRegTam      int    `json:"fsregtam"`
	FSRegDirec    []int  `json:"fsregdirec"`
	FSRegPuntero  int    `json:"fsregpuntero"`
}

type FileContent struct {
	InitialBlock int `json:"initial_block"`
	Size         int `json:"size"`
	FileName     string
}

type AdressFS struct {
	Address []int  `json:"address"`
	Content []byte `json:"data,omitempty"`
	Pid     int    `json:"pid"`
	Length  int    `json:"size,omitempty"`
}

type Bitmap struct {
	bits       []int
	blockCount int
	blockSize  int
}

type Block struct {
	Data []byte // Datos del bloque
}

type BlockFile struct {
	FilePath    string
	BlocksSize  int
	BlocksCount int
	FreeBlocks  []bool // Un slice para rastrear si un bloque está libre
}

/*--------------------------- ESTRUCTURA DEL METADATA -----------------------------*/
var metaDataStructure []FileContent

/*--------------------------- NOMBRE DEL ARCHIVO E INSTRUCCION -----------------------------*/
var fileName string
var fsInstruction string
var fsRegTam int
var fsRegDirec []int
var fsRegPuntero int

/*--------------------------------------------------- VAR GLOBALES ------------------------------------------------------*/

var GLOBALmemoryContent string
var config *globals.Config

/*-------------------------------------------- INICIAR CONFIGURACION ------------------------------------------------------*/

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

	processData, ok := getProcessData(payload.Pid)
	if !ok {
		log.Printf("No se encontraron datos para el PID: %d", payload.Pid)
		// Manejar el caso donde no hay datos para el PID
	}

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
		Interfaz.IO_STDIN_READ(processData.DireccionFisica, processData.LengthREG, pidExecutionProcess)
		log.Printf("Termino de leer desde la interfaz '%s'\n", Interfaz.Nombre)

	case "STDOUT":
		log.Printf("PID: %d - Operacion: IO_STDOUT_WRITE", pidExecutionProcess)
		Interfaz.IO_STDOUT_WRITE(processData.DireccionFisica, processData.LengthREG, pidExecutionProcess)
		log.Printf("Termino de escribir en la interfaz '%s'\n", Interfaz.Nombre)

	case "DialFS":
		Interfaz.FILE_SYSTEM(pidExecutionProcess)

	default:
		log.Fatalf("Tipo de interfaz desconocido: %s", Interfaz.Config.Tipo)
	}
}

/*-------------------------------------------------- ENDPOINTS ------------------------------------------------------*/

func SendPortOfInterfaceToKernel(nombreInterfaz string, config *globals.Config) error {
	kernelURL := fmt.Sprintf("http://localhost:%d/SendPortOfInterfaceToKernel", config.PuertoKernel)

	interfaceData := BodyRequestPort{
		Nombre: nombreInterfaz,
		Port:   config.Puerto,
		Type:   config.Tipo,
	}

	interfaceDataJSON, err := json.Marshal(interfaceData)
	if err != nil {
		log.Fatalf("Error al codificar el puerto a JSON: %v", err)
	}

	resp, err := http.Post(kernelURL, "application/json", bytes.NewBuffer(interfaceDataJSON))
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

// STDOUT, FS_WRITE  leer en memoria y traer lo leido con "ReceiveContentFromMemory"
func SendAdressToMemory(ReadRequest MemoryRequest) error {
	memoriaURL := fmt.Sprintf("http://localhost:%d/readMemory", config.PuertoMemoria)

	adressResponseTest, err := json.Marshal(ReadRequest)
	if err != nil {
		log.Fatalf("Error al serializar el address: %v", err)
	}

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

// STDIN, FS_READ  escribir en memoria
func SendInputToMemory(input *BodyRequestInput) error {

	memoriaURL := fmt.Sprintf("http://localhost:%d/SendInputToMemory", config.PuertoMemoria)

	inputResponseTest, err := json.Marshal(input)
	if err != nil {
		log.Fatalf("Error al serializar el Input: %v", err)
	}

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

	processData := ProcessData{
		Pid:             requestRegister.Pid,
		LengthREG:       requestRegister.Length,
		DireccionFisica: requestRegister.Address,
	}
	processDataMap.Store(requestRegister.Pid, processData)

	log.Printf("Received Register for PID %d: %v", requestRegister.Pid, processData)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf("length received: %d", requestRegister.Length)))
}

func RecieveFSDataFromKernel(w http.ResponseWriter, r *http.Request) {
	var fsStructure FSstructure

	err := json.NewDecoder(r.Body).Decode(&fsStructure)
	if err != nil {
		http.Error(w, "Error decoding JSON data", http.StatusInternalServerError)
		return
	}

	log.Printf("Received fileName: %s", fsStructure.FileName)
	log.Printf("Received fsInstruction: %s", fsStructure.FSInstruction)
	log.Printf("Received fsRegistroTamano: %d", fsStructure.FSRegTam)
	log.Printf("Received fsRegistroDireccion: %d", fsStructure.FSRegDirec)
	log.Printf("Received fsRegistroPuntero: %d", fsStructure.FSRegPuntero)

	fileName = fsStructure.FileName
	fsInstruction = fsStructure.FSInstruction
	fsRegTam = fsStructure.FSRegTam
	fsRegDirec = fsStructure.FSRegDirec
	fsRegPuntero = fsStructure.FSRegPuntero

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

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Content received correctly"))
}

/*----------------------------------------- FUNCIONES AUXILIARES --------------------------------------------------*/

func getProcessData(pid int) (ProcessData, bool) {
	data, ok := processDataMap.Load(pid)
	if !ok {
		return ProcessData{}, false
	}
	return data.(ProcessData), true
}

/*---------------------------------------------- INTERFACES -------------------------------------------------------*/

// INTERFAZ STDOUT (IO_STDOUT_WRITE)
func (Interfaz *InterfazIO) IO_STDOUT_WRITE(address []int, length int, pid int) {
	pathToConfig := os.Args[2]
	log.Printf("Path al archivo de configuración: %s", pathToConfig)

	config, err := LoadConfig(pathToConfig)
	if err != nil {
		log.Fatalf("Error al cargar la configuración desde '%s': %v", pathToConfig, err)
	}

	//var Bodyadress BodyAdress
	req := MemoryRequest{
		PID:     pid,
		Address: address,
		Size:    length,
		Type:    "IO",
		Port:    config.Puerto,
	}

	err1 := SendAdressToMemory(req)
	if err1 != nil {
		log.Fatalf("Error al leer desde la memoria: %v", err)
	}

	time.Sleep(time.Duration(config.UnidadDeTiempo) * time.Millisecond)
	fmt.Println(GLOBALmemoryContent)
}

// INTERFAZ STDIN (IO_STDIN_READ)
func (Interfaz *InterfazIO) IO_STDIN_READ(address []int, lengthREG int, pid int) {
	var BodyInput BodyRequestInput
	var input string

	//var inputMenorARegLongitud string

	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Ingrese por teclado: ")
	input, err := reader.ReadString('\n')
	if err != nil {
		log.Fatalf("Error al leer desde stdin: %v", err)
	}
	input = strings.TrimSpace(input)

	if len(input) > lengthREG {
		input = input[:lengthREG]
		log.Println("El texto ingresado es mayor al tamaño del registro, se truncará a: ", input)
	} else if len(input) < lengthREG {
		fmt.Print("El texto ingresado es menor, porfavor ingrese devuelta: ")

		complemento, err := reader.ReadString('\n')
		if err != nil {
			log.Fatalf("Error al leer desde stdin: %v", err)
		}
		complemento = strings.TrimSpace(complemento)
		input += complemento
		if len(input) > lengthREG {
			input = input[:lengthREG]
		}

	}

	BodyInput.Input = input
	BodyInput.Address = address
	BodyInput.Pid = pid
	log.Printf("EL PID ESSSS: %d", BodyInput.Pid)

	// Guardar el texto en la memoria en la dirección especificada
	err1 := SendInputToMemory(&BodyInput)
	if err1 != nil {
		log.Fatalf("Error al escribir en la memoria: %v", err)
	}
}

// INTERFAZ GENERICA (IO_GEN_SLEEP)
func (interfaz *InterfazIO) IO_GEN_SLEEP(n int) time.Duration {
	return time.Duration(n*interfaz.Config.UnidadDeTiempo) * time.Millisecond
}

// INTERFAZ FILE SYSTEM
func (interfaz *InterfazIO) FILE_SYSTEM(pid int) {
	log.Printf("La interfaz '%s' es de tipo FILE SYSTEM", interfaz.Nombre)

	pathDialFS := interfaz.Config.PathDialFS
	blocksSize := interfaz.Config.TamanioBloqueDialFS
	blocksCount := interfaz.Config.CantidadBloquesDialFS
	sizeFile := blocksSize * blocksCount
	bitmapSize := blocksCount / 8
	unitWorkTimeFS := interfaz.Config.UnidadDeTiempo

	// CHEQUEO EXISTENCIA DE ARCHIVOS BLOQUES.DAT Y BITMAP.DAT, DE NO SER ASI, LOS CREO
	createMetaDataStructure()
	EnsureIfFileExists(pathDialFS, blocksSize, blocksCount, sizeFile, bitmapSize)

	switch fsInstruction {
	case "IO_FS_CREATE":
		IO_FS_CREATE(pathDialFS, fileName)
		log.Printf("PID: %d - Crear Archivo: %s", pid, fileName)

	case "IO_FS_DELETE":
		IO_FS_DELETE(pathDialFS, fileName)
		log.Printf("PID: %d - Eliminar Archivo: %s", pid, fileName)

	case "IO_FS_WRITE":
		IO_FS_WRITE(pathDialFS, fileName, fsRegDirec, fsRegTam, fsRegPuntero, pid)
		log.Printf("PID: %d - Operacion: IO_FS_WRITE - Escribir Archivo: %s - Tamaño a Escribir: %d - Puntero Archivo: %d", pid, fileName, fsRegTam, fsRegPuntero)

	case "IO_FS_TRUNCATE":
		IO_FS_TRUNCATE(pathDialFS, fileName, fsRegTam)
		log.Printf("PID: %d - Operacion: IO_FS_TRUNCATE", pid)

	case "IO_FS_READ":
		IO_FS_READ(pathDialFS, fileName, fsRegDirec, fsRegTam, fsRegPuntero, pid)
		log.Printf("PID: %d - Operacion: IO_FS_READ - Leer Archivo: %s - Tamaño a Leer: %d - Puntero Archivo: %d", pid, fileName, fsRegTam, fsRegPuntero)
	}

	log.Printf("La duración de la operación de FILE SYSTEM es de %d unidades de tiempo", unitWorkTimeFS)
	time.Sleep(time.Duration(unitWorkTimeFS) * time.Millisecond)
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

/* -------------------------------------------- FUNCIONES DE FS_CREATE ------------------------------------------------------ */

func IO_FS_CREATE(pathDialFS string, fileName string) {
	log.Printf("Creando archivo %s en %s", fileName, pathDialFS)

	filePath := pathDialFS + "/" + fileName
	file, err := os.Create(filePath)
	if err != nil {
		log.Printf("Error al crear el archivo '%s': %v", pathDialFS, err)
	}
	defer file.Close()

	// Abrir el archivo de bitmap para lectura
	bitmapFilePath := pathDialFS + "/bitmap.dat"

	bitmap := readAndCopyBitMap(bitmapFilePath)

	//Calcular el primer bit libre
	firstFreeBlock := firstBitFree(bitmap)

	if firstFreeBlock == -1 {
		log.Printf("No hay bloques libres disponibles")
	} else {
		fileSize := 0

		bitmap.Set(firstFreeBlock)

		// Mostrar el contenido del bitmap
		showBitmap(bitmap)

		updateBitMap(bitmap, bitmapFilePath)

		updateMetaDataFile(pathDialFS, fileName, firstFreeBlock, fileSize)

		fmt.Printf("Archivo '%s' creado y escrito exitosamente.\n", fileName)
	}
}

func updateMetaDataFile(pathDialFS string, fileName string, initialBlock int, fileSize int) {
	filePath := pathDialFS + "/" + fileName
	fileContent := FileContent{
		InitialBlock: initialBlock,
		Size:         fileSize,
	}
	contentBytes, err := json.Marshal(fileContent)
	if err != nil {
		log.Fatalf("Error al convertir FileContent a bytes: %v", err)
	}

	// Write FileContent to the file
	file, err := os.OpenFile(filePath, os.O_RDWR, 0644)
	if err != nil {
		log.Fatalf("Error al crear el archivo '%s': %v", pathDialFS, err)
	}
	_, err = file.Write(contentBytes)
	if err != nil {
		log.Fatalf("Error al escribir el contenido en el archivo '%s': %v", filePath, err)
	}

	file.Close()
}

func checkFilesInDirectory(pathDialFS string) bool {
	files, err := os.ReadDir(pathDialFS)
	if err != nil {
		log.Printf("Error reading directory: %v", err)
		return false
	}

	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".txt") {
			log.Printf("Found .txt file in %s", pathDialFS)
			return true
		}
	}

	log.Printf("No .txt files found in %s", pathDialFS)
	return false
}

func readFile(pathFile string) FileContent {
	readContent, err := os.ReadFile(pathFile)
	if err != nil {
		log.Fatalf("Error al leer el archivo '%s': %v", pathFile, err)
	}

	var fileContent FileContent
	err = json.Unmarshal(readContent, &fileContent)
	if err != nil {
		log.Fatalf("Error al deserializar el contenido del archivo '%s': %v", pathFile, err)
	}

	fileContent.FileName = filepath.Base(pathFile)

	return fileContent
}

func readFilesInDirectory(directoryPath string) []FileContent {
	var filesContent []FileContent

	files, err := os.ReadDir(directoryPath)
	if err != nil {
		log.Fatalf("Error al leer el directorio '%s': %v", directoryPath, err)
	}

	for _, file := range files {
		if !file.IsDir() && filepath.Ext(file.Name()) == ".txt" {
			filePath := filepath.Join(directoryPath, file.Name())
			fileContent := readFile(filePath)
			filesContent = append(filesContent, fileContent)
		}
	}

	return filesContent
}

func createMetaDataStructure() {
	if checkFilesInDirectory(config.PathDialFS) {
		// Example usage of readFilesInDirectory
		metaDataStructure = readFilesInDirectory(config.PathDialFS)

		// Display filesContent
		for i, fileContent := range metaDataStructure {
			fmt.Printf("MetaStructure %d: FileName %s InitialBlock: %d, Size: %d\n", i, fileContent.FileName, fileContent.InitialBlock, fileContent.Size)
		}
	}
}

func firstBitFree(bitmap *Bitmap) int {
	for i := 0; i < config.CantidadBloquesDialFS; i++ {
		isFree := !bitmap.Get(i)
		if isFree {
			fmt.Printf("Found free bit at index %d\n", i)
			return i
		}

	}
	fmt.Println("No free bits found")
	return -1
}

/* ------------------------------------------- FUNCIONES DE FS_DELETE ------------------------------------------------------ */

func IO_FS_DELETE(pathDialFS string, fileName string) {
	log.Printf("Eliminando el archivo %s en %s", fileName, pathDialFS)

	filePath := pathDialFS + "/" + fileName

	// PRIMERO CHEQUEO QUE EL ARCHIVO EXISTE
	/*if !verificarExistenciaDeArchivo(pathDialFS, fileName) {
		return
	}*/

	err := os.Remove(filePath)
	if err != nil {
		log.Fatalf("Error al eliminar el archivo '%s': %v", pathDialFS, err)
	}

	// UNA VEZ REMOVIDO EL ARCHIVO, TENGO QUE ACTUALIZAR BITMAP Y ARCHIVO DE BLOQUES
	// Abrir el archivo de bitmap para lectura
	var fileData FileContent
	fileData, err = dataFileInMetaDataStructure(fileName)
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	var blocksToDelete int

	if fileData.Size > 0 {
		blocksToDelete = fileData.Size / config.TamanioBloqueDialFS
	} else {
		blocksToDelete = 1
	}

	bitmapFilePath := pathDialFS + "/bitmap.dat"
	bitmap := readAndCopyBitMap(bitmapFilePath)

	removeBlocks(bitmap, fileData.InitialBlock, blocksToDelete, blocksToDelete)

	showBitmap(bitmap)

	updateBitMap(bitmap, bitmapFilePath)

	deleteInMetaDataStructure(fileName)
}

func searchInMetaDataStructure(fileName string) int {
	for _, fileContent := range metaDataStructure {
		if fileContent.FileName == fileName {
			return fileContent.InitialBlock
		}
	}
	return -1
}

func deleteInMetaDataStructure(fileName string) {
	for i, fileContent := range metaDataStructure {
		if fileContent.FileName == fileName {
			metaDataStructure = append(metaDataStructure[:i], metaDataStructure[i+1:]...)
			break
		}
	}
}

/* ----------------------------------------- FUNCIONES DE FS_TRUNCATE ------------------------------------------------------ */

func IO_FS_TRUNCATE(pathDialFS string, fileName string, length int) {
	log.Printf("Truncando el archivo %s en %s", fileName, pathDialFS)

	// VERIFICO EXISTENCIA DE ARCHIVO
	verificarExistenciaDeArchivo(pathDialFS, fileName)

	// SACO LA CANTIDAD DE BLOQUES NECESARIOS
	var fileData FileContent
	fileData, err := dataFileInMetaDataStructure(fileName)
	if err != nil {
		// Handle the error, e.g., log it or return it
		log.Printf("Error: %v", err)
		return
	}
	bitmapFilePath := pathDialFS + "/bitmap.dat"
	bitmap := readAndCopyBitMap(bitmapFilePath)
	cantBloques := length / config.TamanioBloqueDialFS
	totalFreeBlocks := getTotalFreeBlocks(bitmap)

	if length > fileData.Size {
		log.Printf("El tamaño a truncar es mayor al tamaño actual del archivo")
		areFree := lookForContiguousBlocks(cantBloques, fileData.InitialBlock, pathDialFS)
		if areFree {
			log.Printf("Los bloques solicitados están libres")
			assignBlocks(bitmap, fileData.InitialBlock, cantBloques)
			showBitmap(bitmap)
			updateBitMap(bitmap, bitmapFilePath)
			updateMetaDataFile(pathDialFS, fileName, fileData.InitialBlock, length)
		} else {
			log.Printf("Los bloques solicitados no están libres")
			truncateBitmap(bitmap, fileData.InitialBlock, cantBloques, bitmapFilePath, pathDialFS, fileName, fileData.Size)
			firstFreeBlock := firstBitFree(bitmap)
			assignBlocks(bitmap, firstFreeBlock, cantBloques)
			showBitmap(bitmap)
			updateBitMap(bitmap, bitmapFilePath)
			log.Printf("firstFreeBlock: %d", firstFreeBlock)
			updateMetaDataFile(pathDialFS, fileName, firstFreeBlock, length)
		}
	} else if length < fileData.Size {
		log.Printf("El tamaño a truncar es menor al tamaño actual del archivo")
		totalBlocks := fileData.Size / config.TamanioBloqueDialFS
		removeBlocks(bitmap, fileData.InitialBlock, totalBlocks, cantBloques)
		showBitmap(bitmap)
		updateBitMap(bitmap, bitmapFilePath)
		updateMetaDataFile(pathDialFS, fileName, fileData.InitialBlock, length)
	} else if length == fileData.Size {
		log.Printf("El tamaño a truncar es igual al tamaño actual del archivo")
	} else {
		log.Printf("Error al truncar el archivo, bloques disponibles %d", totalFreeBlocks)

	}
}

func assignBlocks(bitmap *Bitmap, initialBlock int, cantBloques int) {
	for i := initialBlock; i < initialBlock+cantBloques; i++ {
		bitmap.Set(i)
	}
}

func getTotalFreeBlocks(bitmap *Bitmap) int {
	totalFreeBlocks := 0
	for i := 0; i < config.CantidadBloquesDialFS; i++ {
		if !bitmap.Get(i) {
			totalFreeBlocks++
		}
	}
	return totalFreeBlocks
}

func removeBlocks(bitmap *Bitmap, initialBlock int, totalBlocks int, blocksToRemove int) {
	if blocksToRemove >= totalBlocks {
		// Remove all blocks if we're removing all or more than total
		for i := initialBlock; i < initialBlock+totalBlocks; i++ {
			bitmap.Remove(i)
		}
	} else {
		// Remove only the last 'blocksToRemove' blocks
		for i := initialBlock + totalBlocks - blocksToRemove; i < initialBlock+totalBlocks; i++ {
			bitmap.Remove(i)
		}
	}
}

func dataFileInMetaDataStructure(fileName string) (FileContent, error) {
	for _, fileContent := range metaDataStructure {
		if fileContent.FileName == fileName {
			return fileContent, nil
		}
	}
	return FileContent{}, fmt.Errorf("file '%s' not found in metadata structure", fileName)
}

func lookForContiguousBlocks(cantBloques int, initialBlock int, pathDialFS string) bool {
	log.Printf("Buscando %d bloques contiguos desde el bloque %d", cantBloques, initialBlock)
	// Abrir el archivo de bitmap para lectura
	bitmapFilePath := pathDialFS + "/bitmap.dat"

	// LEER EL CONTENIDO DEL ARCHIVO DE BITMAP
	bitmapBytes, err := os.ReadFile(bitmapFilePath)
	if err != nil {
		log.Fatalf("Error al leer el archivo de bitmap '%s': %v", bitmapFilePath, err)
	}

	// Crear un nuevo Bitmap y llenarlo con los datos leídos
	bitmap := NewBitmap()
	err = bitmap.FromBytes(bitmapBytes)
	if err != nil {
		log.Fatalf("Error al convertir bytes a bitmap: %v", err)
	}

	// Verificar si el rango está dentro de los límites del bitmap
	if initialBlock+cantBloques > config.CantidadBloquesDialFS {
		fmt.Printf("El rango solicitado excede el tamaño del bitmap\n")
		return false
	}

	// Verificar si todos los bloques en el rango están libres
	for i := initialBlock + 1; i < initialBlock+cantBloques; i++ {
		if bitmap.Get(i) {
			fmt.Printf("Bloque %d está ocupado\n", i)
			return false
		}
	}

	fmt.Printf("Todos los bloques desde %d hasta %d están libres\n", initialBlock+1, initialBlock+cantBloques)
	return true
}

func truncateBitmap(bitmap *Bitmap, initialBlock int, cantBloques int, bitmapFilePath string, pathDialFS string, fileName string, fileSize int) {
	//eliminamos los bloques que tiene asignado el archivo

	var blocksToDelete int
	if fileSize > 0 {
		blocksToDelete = fileSize / config.TamanioBloqueDialFS
	} else {
		blocksToDelete = 1
	}
	removeBlocks(bitmap, initialBlock, blocksToDelete, blocksToDelete)
	deleteInMetaDataStructure(fileName)

	for _, fileContent := range metaDataStructure {
		blocksPerFile := getBlocksFile(fileContent.FileName)
		fmt.Printf("File: %s, tiene estos bloques: %d\n", fileContent.FileName, blocksPerFile)
		newInitialBlock := moveZeros(bitmap, fileContent.InitialBlock, blocksPerFile, bitmapFilePath)
		updateMetaDataFile(pathDialFS, fileContent.FileName, newInitialBlock, fileContent.Size)
	}

}

func moveZeros(bitmap *Bitmap, initialBlock int, cantBloques int, bitmapFilePath string) int {
	snakeSize := 0
	newInitialBlock := initialBlock
	for i := 0; i < initialBlock+cantBloques; i++ {
		if !bitmap.Get(i) {
			snakeSize++
		} else {
			if snakeSize > 0 {
				bitmap.Remove(i)
				bitmap.Set(i - snakeSize)
				if i-snakeSize < newInitialBlock {
					newInitialBlock = i - snakeSize
				}
			}
		}
	}
	log.Printf("snakeSize: %d", snakeSize)
	updateBitMap(bitmap, bitmapFilePath)

	return newInitialBlock
}

func getBlocksFile(fileName string) int {
	for _, fileContent := range metaDataStructure {
		if fileContent.FileName == fileName {
			return fileContent.InitialBlock + (fileContent.Size / config.TamanioBloqueDialFS) - 1
		}
	}
	return -1
}

/* ----------------------------------------- FUNCIONES DE FS_WRITE ------------------------------------------------------ */

func IO_FS_WRITE(pathDialFS string, fileName string, adress []int, length int, regPuntero int, pid int) {
	log.Printf("Escribiendo en el archivo %s en %s", fileName, pathDialFS)

	// VERIFICO EXISTENCIA DE ARCHIVO
	verificarExistenciaDeArchivo(pathDialFS, fileName)

	req := MemoryRequest{
		PID:     pid,
		Address: adress,
		Size:    length,
		Type:    "IO",
		Port:    config.Puerto,
	}

	err := SendAdressToMemory(req)
	if err != nil {
		log.Fatalf("Error al leer desde la memoria: %v", err)
	}

	// VERIFICO EXISTENCIA DE ARCHIVO
	verificarExistenciaDeArchivo(pathDialFS, fileName)

	// TENGO QUE ABRIR EL ARCHIVO DE BLOQUES.DAT
	blocksFilePath := filepath.Join(pathDialFS, "bloques.dat")
	blocksFile, err := os.OpenFile(blocksFilePath, os.O_RDWR, 0644)
	if err != nil {
		log.Fatalf("Error al abrir el archivo de bloques '%s': %v", blocksFilePath, err)
	}
	defer blocksFile.Close()

	// BLOQUE A ESCRIBIR
	bloqueInicialDelArchivo := searchInMetaDataStructure(fileName)
	//fileData := dataFileInMetaDataStructure(fileName)

	posicionInicialDeEscritura := (bloqueInicialDelArchivo * config.TamanioBloqueDialFS) + regPuntero

	// ME MUEVO A LA POSICION INICIAL DE ESCRITURA
	_, err = blocksFile.Seek(int64(posicionInicialDeEscritura), 0)
	if err != nil {
		log.Fatalf("Error al mover el cursor del archivo de bloques '%s': %v", blocksFilePath, err)
	}

	// ESCRIBIR EN EL ARCHIVO DE BLOQUES
	// Aquí deberías escribir los datos reales en lugar de "hola"
	// Por ejemplo, podrías usar los datos de 'adress' y 'length'
	dataToWrite := []byte(GLOBALmemoryContent) // Reemplazar esto con los datos reales--->GLOBALmemoryContent
	_, err = blocksFile.Write(dataToWrite)
	if err != nil {
		log.Fatalf("Error al escribir en el archivo de bloques '%s': %v", blocksFilePath, err)
	}

	// Mover el cursor al inicio del bloque para leer el contenido
	_, err = blocksFile.Seek(int64(bloqueInicialDelArchivo*config.TamanioBloqueDialFS), 0)
	if err != nil {
		log.Fatalf("Error al mover el cursor para leer: %v", err)
	}

	// Leer el contenido del archivo
	fileContent := make([]byte, config.TamanioBloqueDialFS*config.CantidadBloquesDialFS) // Asumiendo que el archivo ocupa un bloque
	bytesRead, err := blocksFile.Read(fileContent)
	if err != nil && err != io.EOF {
		log.Fatalf("Error al leer el contenido del archivo: %v", err)
	}

	// Mostrar el contenido del archivo
	fmt.Printf("Contenido del archivo %s después de la escritura:\n", fileName)
	fmt.Println(string(fileContent[:bytesRead]))

	log.Printf("Archivo %s escrito exitosamente", fileName)
}

/* ------------------------------------------ FUNCIONES DE FS_READ ------------------------------------------------------ */

func IO_FS_READ(pathDialFS string, fileName string, address []int, length int, regPuntero int, pid int) {
	log.Printf("Leyendo el archivo %s en %s", fileName, pathDialFS)

	// VERIFICO EXISTENCIA DE ARCHIVO
	verificarExistenciaDeArchivo(pathDialFS, fileName)

	// TENGO QUE ABRIR EL ARCHIVO DE BLOQUES.DAT
	blocksFilePath := pathDialFS + "/bloques.dat"
	blocksFile, err := os.Open(blocksFilePath)
	if err != nil {
		log.Fatalf("Error al abrir el archivo de bloques '%s': %v", blocksFilePath, err)
	}
	defer blocksFile.Close()

	// BLOQUE A LEER
	bloqueInicialDelArchivo := searchInMetaDataStructure(fileName)
	posicionInicialDeLectura := (bloqueInicialDelArchivo * config.TamanioBloqueDialFS) + regPuntero

	// ME MUEVO A LA POSICION INICIAL DE LECTURA
	_, err = blocksFile.Seek(int64(posicionInicialDeLectura), 0)
	if err != nil {
		log.Fatalf("Error al mover el cursor del archivo de bloques '%s': %v", blocksFilePath, err)
	}

	// LEO LA CANTIDAD DE BYTES INDICADA POR LENGTH Y CREO UN SLICE CON UN TAMAÑO DEFINIDO PARA ALMACERNARLO
	contenidoLeidoDeArchivo := make([]byte, length)
	_, err = blocksFile.Read(contenidoLeidoDeArchivo)
	if err != nil {
		log.Fatalf("Error al leer el archivo de bloques '%s': %v", blocksFilePath, err)
	}

	// TENGO QUE ESCRIBIR EL CONTENIDO LEIDO EN MEMORIA A PARTIR DE LA DIRECCION FISICA INDICADA EN ADDRESS
	// Llamo a endpoint para escribir el contenido en memoria
	var BodyAdressFSRead BodyRequestInput
	BodyAdressFSRead.Pid = pid
	BodyAdressFSRead.Input = string(contenidoLeidoDeArchivo)
	BodyAdressFSRead.Address = address

	err1 := SendInputToMemory(&BodyAdressFSRead)
	if err1 != nil {
		log.Fatalf("Error al leer desde la memoria: %v", err)
	}
}

func verificarExistenciaDeArchivo(path string, fileName string) {
	filePath := path + "/" + fileName
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		log.Fatalf("El archivo '%s' no existe", fileName)
	}
}

/* ------------------------------------- CREAR ARCHIVOS DE BLOQUES Y BITMAP ------------------------------------------------------ */

func CreateBlockFile(path string, blocksSize int, blocksCount int, sizeFile int) (*BlockFile, error) {

	filePath := path + "/bloques.dat"

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

	return &BlockFile{
		FilePath:    filePath,
		BlocksSize:  blocksSize,
		BlocksCount: blocksCount,
		FreeBlocks:  make([]bool, blocksCount),
	}, nil
}

func CreateBitmapFile(path string, blocksCount int, bitmapSize int) {
	filePath := path + "/bitmap.dat"

	bitmapFile, err := os.Create(filePath)
	if err != nil {
		log.Fatalf("Error al crear el archivo de bitmap '%s': %v", filePath, err)
	}
	defer bitmapFile.Close()

	bitmap := NewBitmap()

	bitmapBytes := bitmap.ToBytes()
	_, err = bitmapFile.Write(bitmapBytes)
	if err != nil {
		log.Fatalf("Error al inicializar el archivo de bitmap '%s': %v", filePath, err)
	}

	// flushear si hubo error
	if err := bitmapFile.Sync(); err != nil {
		log.Fatalf("Error al forzar la escritura del archivo de bitmap '%s': %v", filePath, err)
	}
}

/* --------------------------------------------- METODOS DEL BITMAP ------------------------------------------------------ */

func NewBitmap() *Bitmap {
	return &Bitmap{
		bits:       make([]int, config.CantidadBloquesDialFS),
		blockCount: config.CantidadBloquesDialFS,
		blockSize:  config.TamanioBloqueDialFS,
	}
}

func (b *Bitmap) FromBytes(bytes []byte) error {
	expectedBytes := (b.blockCount * b.blockSize) / 8
	if len(bytes) != expectedBytes {
		return fmt.Errorf("invalid byte slice length: expected %d bytes, got %d", expectedBytes, len(bytes))
	}

	b.bits = make([]int, b.blockCount)
	for i := 0; i < b.blockCount; i++ {
		byteIndex := (i * b.blockSize) / 8
		bitOffset := (i * b.blockSize) % 8
		if bytes[byteIndex]&(1<<bitOffset) != 0 {
			b.bits[i] = 1
		}
	}
	return nil
}

func (b *Bitmap) ToBytes() []byte {
	bytes := make([]byte, (b.blockCount*b.blockSize)/8)
	for i := 0; i < b.blockCount; i++ {
		if b.bits[i] == 1 {
			byteIndex := (i * b.blockSize) / 8
			bitOffset := (i * b.blockSize) % 8
			bytes[byteIndex] |= 1 << bitOffset
		}
	}
	return bytes
}

func (b *Bitmap) Get(pos int) bool {
	if pos < 0 || pos >= b.blockCount {
		return false
	}
	return b.bits[pos] == 1
}

func (b *Bitmap) Set(pos int) {
	if pos < 0 || pos >= b.blockCount {
		return
	}
	b.bits[pos] = 1
}

func (b *Bitmap) Remove(pos int) {
	if pos < 0 || pos >= b.blockCount {
		return
	}
	b.bits[pos] = 0
}

/* ------------------------------------- METODOS DE BLOQUES ------------------------------------------------------ */

func showBitmap(bitmap *Bitmap) {
	fmt.Println("Bitmap:")
	for i := 0; i < config.CantidadBloquesDialFS; i++ {
		if bitmap.Get(i) {
			fmt.Print("1")
		} else {
			fmt.Print("0")
		}
		if (i+1)%64 == 0 {
			fmt.Println() // New line every 64 bits for readability
		}
	}
}

func readAndCopyBitMap(bitmapFilePath string) *Bitmap {
	bitmapBytes, err := os.ReadFile(bitmapFilePath)
	if err != nil {
		log.Fatalf("Error al leer el archivo de bitmap '%s': %v", bitmapFilePath, err)
	}

	bitmap := NewBitmap()
	err = bitmap.FromBytes(bitmapBytes)
	if err != nil {
		log.Fatalf("Error al convertir bytes a bitmap: %v", err)
	}

	return bitmap
}

func updateBitMap(bitmap *Bitmap, bitmapFilePath string) {
	modifiedBitmapBytes := bitmap.ToBytes()

	err := os.WriteFile(bitmapFilePath, modifiedBitmapBytes, 0644)
	if err != nil {
		log.Fatalf("Error al escribir el archivo de bitmap modificado '%s': %v", bitmapFilePath, err)
	}

	fmt.Println("Bitmap file updated successfully.")
}

//fs pide posicion a memoria, si lo agarra y lo guarda en el archivo de bloques.dat
// bloques basados por tamaños de byte, ej 4 bytes por bloque y si pongo hola que ocupa 7 bytes, ocupa un bloque
// cuando hago create lo que voy a hacer es meterlo al filesystem, notas.txt ya existe en tu ruta de dialfs_path y lo que hago es escribir en bloques.dat
// como existe en esa ruta voy a acceder y sacar la metdata

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

/*currentBlocks := global.GetCurrentBlocks(global.Estructura_truncate.FileName)
  freeContiguousBlocks := global.GetFreeContiguousBlocks(global.Estructura_truncate.FileName)
  neededBlocks := global.GetNeededBlocks(global.Estructura_truncate)
  totalFreeBlocks := global.GetTotalFreeBlocks()

  if currentBlocks == neededBlocks {
      global.UpdateSize(global.Estructura_truncate.FileName, global.Estructura_truncate.Tamanio, neededBlocks)
      global.Logger.Log(fmt.Sprintf("No es necesario truncar pero actualicé el size: %+v", global.Estructura_truncate), log.DEBUG)

  } else if !(totalFreeBlocks >= neededBlocks-currentBlocks) {
      global.Logger.Log(fmt.Sprintf("No es posible agrandar el archivo: %+v", global.Estructura_truncate), log.ERROR)

  } else if currentBlocks > neededBlocks {
      global.Logger.Log(fmt.Sprintf("Trunco a menos %+v", global.Estructura_truncate), log.DEBUG)

      global.UpdateSize(global.Estructura_truncate.FileName, global.Estructura_truncate.Tamanio, neededBlocks)
      global.PrintBitmap()
      global.UpdateBitmap(0, filestruct.Initial_block+neededBlocks, currentBlocks-neededBlocks)
      global.PrintBitmap()

  } else if neededBlocks-currentBlocks <= freeContiguousBlocks {
      global.Logger.Log(fmt.Sprintf("Trunco a más %+v", global.Estructura_truncate), log.DEBUG)

      global.UpdateSize(global.Estructura_truncate.FileName, global.Estructura_truncate.Tamanio, neededBlocks)
      global.PrintBitmap()
      global.UpdateBitmap(1, filestruct.Initial_block+currentBlocks, neededBlocks-currentBlocks)
      global.PrintBitmap()

  } else {
      global.Logger.Log(fmt.Sprintf("Es necesario compactar: %+v", global.Estructura_truncate), log.DEBUG)

      // actualizar bitamp y archivos metadata
      compactar(global.Estructura_truncate.FileName, totalFreeBlocks)

      global.PrintBloques()

  }*/
