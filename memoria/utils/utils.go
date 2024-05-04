package utils

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/sisoputnfrba/tp-golang/memoria/globals"
)

type PruebaMensaje struct {
	Mensaje string `json:"Prueba"`
}
type BodyRequest struct {
	Path string `json:"path"`
}

type InstructionResposne struct {
	Instruction string `json:"instruction"`
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

func ConfigurarLogger() {
	logFile, err := os.OpenFile("memoria.log", os.O_CREATE|os.O_APPEND|os.O_RDWR, 0666)
	if err != nil {
		panic(err)
	}
	mw := io.MultiWriter(os.Stdout, logFile)
	log.SetOutput(mw)
}

func ProcessSavedPathFromKernel(w http.ResponseWriter, r *http.Request) {
	// CHEQUEO CON UN LOG PARA VERIFICAR LA SOLICITUD DEL KERNEL
	log.Printf("Recibiendo solicitud de path desde el kernel")

	// CREO VARIABLE savedPath
	var savedPath BodyRequest
	err := json.NewDecoder(r.Body).Decode(&savedPath)
	if err != nil {
		http.Error(w, "Error al decodificar los datos JSON", http.StatusInternalServerError)
		return
	}

	// HAGO UN LOG SI PASO ERRORES PARA RECEPCION DEL PATH
	globalPath = savedPath.Path
	log.Printf("Path recibido desde el kernel: %s", savedPath.Path)

	// ABRO ARCHIVO DEL PATH ENVIADO POR EL KERNEL
	file, err := os.Open(savedPath.Path)
	check(err)

	// CHEQUEO
	fi, err := file.Stat()
	check(err)

	// ESTAS 4 LINEAS SON PARA LEER Y EL ARCHIVO
	sliceBytes := make([]byte, fi.Size())      //Esta línea crea un slice de bytes
	numBytesRead, err := file.Read(sliceBytes) //es el número de bytes leídos
	check(err)
	fmt.Printf("%d bytes: %s\n", numBytesRead, string(sliceBytes[:numBytesRead])) //Esta línea imprime el número de bytes leídos (numBytesRead) y el contenido del slice de bytes sliceBytes. string(sliceBytes[:numBytesRead]) convierte el slice de bytes en una cadena y solo imprime los primeros numBytesRead bytes leídos, ya que el slice puede tener una capacidad mayor que la cantidad real de bytes leídos.

	file.Close()

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(savedPath.Path))
}

func check(e error) {
	if e != nil {
		panic(e)
	}
}

var globalPath string

func ProcessSavedPCFromCPU(w http.ResponseWriter, r *http.Request) {
	// HAGO UN LOG PARA CHEQUEAR RECEPCION
	log.Printf("Recibiendo solicitud de contexto de ejecucuion desde el CPU")

	// GUARDO PCB RECIBIDO EN sendPCB
	var sendPC int
	err := json.NewDecoder(r.Body).Decode(&sendPC)
	if err != nil {
		http.Error(w, "Error al decodificar los datos JSON", http.StatusInternalServerError)
		return
	}

	// HAGO UN LOG PARA CHEQUEAR QUE PASO ERRORES
	log.Printf("PC recibido desde el CPU: %+v", sendPC)

	instruction, _ := readInstructions(globalPath, sendPC)

	reponse := InstructionResposne{
		Instruction: instruction,
	}
	jsonResponse, _ := json.Marshal(reponse)
	log.Printf("PC recibido desde el CPU: %s", jsonResponse)

	w.WriteHeader(http.StatusOK)
	w.Write(jsonResponse)
	// MANDO EL PC DIRECTAMENTE A MEMORIA
	/*if err := SendPCToMemoria(sendPCB.CpuReg.PC); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}*/
}

func readInstructions(path string, targetLine int) (string, error) {
	// Open the file for reading
	readFile, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("error opening file: %w", err)
	}
	defer readFile.Close() // Ensure file is closed even on errors

	// Create a new scanner for line-by-line reading
	fileScanner := bufio.NewScanner(readFile)
	fileScanner.Split(bufio.ScanLines)

	// Line counter for efficient access
	lineNumber := 1
	instruction := "" // Declare the variable "instruction" before the loop

	// Read lines until target line is reached or EOF
	for fileScanner.Scan() {
		if lineNumber == targetLine {
			instruction = fileScanner.Text() // Assign the scanned line to the variable instruction
			log.Printf("PC: %s y tenemos la instuction %s", fileScanner.Text(), instruction)
			return instruction, nil
		}
		log.Printf("Afuera: %s", fileScanner.Text())
		lineNumber++
	}
	return "", fmt.Errorf("target line not found")
}
