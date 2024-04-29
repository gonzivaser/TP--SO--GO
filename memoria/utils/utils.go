package utils

import (
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
	//globals.ClientConfig = IniciarConfiguracion("config.json")
	log.Printf("Recibiendo solicitud de path desde el kernel")
	var savedPath BodyRequest
	err := json.NewDecoder(r.Body).Decode(&savedPath)
	if err != nil {
		http.Error(w, "Error al decodificar los datos JSON", http.StatusInternalServerError)
		return
	}
	log.Printf("Path recibido desde el kernel: %s", savedPath.Path)

	file, err := os.Open(savedPath.Path)
	check(err)

	fi, err := file.Stat()
	check(err)

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
