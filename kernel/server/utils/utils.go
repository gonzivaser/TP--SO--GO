package utils

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type BodyRequest struct {
	Name string `json:"name"`
}

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
