package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"

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
	globals.ClientConfig = IniciarConfiguracion("config.json")

	if r.Method != http.MethodGet {
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}
	//path := r.PathValue("path")

	//Buscar el path en el filesystem y asignar instrucciones a cada proceso
	p := filepath.Join(globals.ClientConfig.InstructionsPath, "instru.txt")
	f, err := os.Open(p)
	check(err)

	fi, err := f.Stat()
	check(err)

	b1 := make([]byte, fi.Size())
	n1, err := f.Read(b1)
	check(err)
	fmt.Printf("%d bytes: %s\n", n1, string(b1[:n1]))

	f.Close()

	// Hacer algo con el savedPath recibido
	log.Printf("SavedPath recibido desde el kernel: %+v", p)

	// Responder al kernel si es necesario
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("SavedPath recibido exitosamente"))
}

func check(e error) {
	if e != nil {
		panic(e)
	}
}
