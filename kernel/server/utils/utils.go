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
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	respuesta, err := json.Marshal(fmt.Sprintf("Hola %s! Como andas?", request.Name))
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
	var request BodyRequest
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	respuesta, err := json.Marshal(fmt.Sprintf("pid: %d", pid))
	if err != nil {
		http.Error(w, "Error al codificar los datos como JSON", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(respuesta)
}

func ListarProcesos(w http.ResponseWriter, r *http.Request) {

}
