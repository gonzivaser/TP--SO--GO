package utils

import (
	"encoding/json"
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

	log.Printf("Recibiendo solicitud de path desde el kernel")
	var savedPath BodyRequest
	err := json.NewDecoder(r.Body).Decode(&savedPath)
	if err != nil {
		http.Error(w, "Error al decodificar los datos JSON", http.StatusInternalServerError)
		return
	}
	log.Printf("Path recibido desde el kernel: %s", savedPath.Path)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(savedPath.Path))
}

/*func check(e error) {
	if e != nil {
		panic(e)
	}
}*/
