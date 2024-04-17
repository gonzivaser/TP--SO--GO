package main

import (
	"net/http"

	"github.com/sisoputnfrba/tp-golang/cpu/utils"
)

func main() {
	utils.ConfigurarLogger()
	http.HandleFunc("GET /input", utils.Prueba)
	http.ListenAndServe(":8075", nil)
}
