package utils

import (
	"io"
	"log"
	"net/http"
	"os"
)

type PruebaMensaje struct {
	Mensaje string `json:"Prueba"`
}

type BodyRequest struct {
	Path string `json:"path"`
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
	log.Print(path)

	/*
		err := json.NewDecoder(r.Body).Decode(&savedPath)
		if err != nil {
			http.Error(w, "Error al decodificar los datos JSON", http.StatusInternalServerError)
			return
		}
	*/

	// Hacer algo con el savedPath recibido
	log.Printf("SavedPath recibido desde el kernel: %+v", path)

	// Responder al kernel si es necesario
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("SavedPath recibido exitosamente"))
}

/*
------OTRA FORMA DE HACER EL PROCESSSAVEDPATHFROMKERNEL------

func ProcessSavedPathFromKernel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}

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

*/
