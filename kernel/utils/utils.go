package utils

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type BodyRequest struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

var pid int = 0

func HelloWorld(w http.ResponseWriter, r *http.Request) {
	var request BodyRequest
	// MANDO LA REQUEST Y DECODIFICO
	err := json.NewDecoder(r.Body).Decode(&request)
	// ESTE IF ESTA POR SI NO PUDO PEGAR A LA API
	if err != nil {
		http.Error(w, fmt.Sprintf("Rompi todo %s", err.Error()), http.StatusBadRequest)
		return
	}
	// EN CASO DE QUE PASA ESE IF, IMPRIMO HOLA (NOMBRE) COMO ANDAS
	respuesta, err := json.Marshal(fmt.Sprintf("Hola %s! Como andas?", request.Name))

	// ESTE IF ESTA EN CASO DE QUE POR EJEMPLO EL NOMBRE ESTE MAL Y NO PUEDA DECODIFICARLO
	if err != nil {
		http.Error(w, "Error al codificar los datos como JSON", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(respuesta)
}

//Listar Procesos: Se encargará de mostrar por consola y retornar por la api el listado de procesos que se encuentran en el sistema con su respectivo
//estado dentro de cada uno de ellos.

func IniciarProceso(w http.ResponseWriter, r *http.Request) {

	respuesta, err := json.Marshal(fmt.Sprintf("pid: %d", pid))
	if err != nil {
		http.Error(w, "Error al codificar los datos como JSON", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(respuesta)
}
