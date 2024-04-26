package utils

import (
	"encoding/json"
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

	if r.Method != http.MethodGet {
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}
	path := r.PathValue("path")

	searchFileInMemoria(path, "nombreDeArchivo")

	// Hacer algo con el savedPath recibido
	log.Printf("SavedPath recibido desde el kernel: %+v", path)

	// Responder al kernel si es necesario
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("SavedPath recibido exitosamente"))
}

func searchFileInMemoria(rootPath string, targetFileName string) (string, error) { //Probable desencadenacion de la ruta en path y nombre de archivo
	log.Printf("Buscando archivo en Memoria con path: %s", rootPath)
	var targetPath string

	// Recorre todos los archivos y directorios dentro de rootPath
	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		// Si hay un error al acceder al archivo o directorio, regresa el error
		if err != nil {
			return err
		}

		// Si el nombre del archivo coincide con el objetivo, establece targetPath
		if info.Name() == targetFileName {
			targetPath = path
			return nil // Detiene la búsqueda
		}

		return nil
	})

	if err != nil {
		return "", err
	}

	if targetPath == "" {
		log.Fatalf("archivo no encontrado: %s", targetFileName)
	}

	return targetPath, nil
}
