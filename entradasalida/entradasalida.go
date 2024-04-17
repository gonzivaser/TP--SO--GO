package main

import (
	"net/http"

	"github.com/sisoputnfrba/tp-golang/entradasalida/utils"
)

func main() {
	utils.ConfigurarLogger()
	http.HandleFunc("GET /input", utils.Prueba)
	http.ListenAndServe(":8090", nil)
}
