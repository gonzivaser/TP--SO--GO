package main

import (
	"net/http"

	"github.com/sisoputnfrba/tp-golang/memoria/utils"
)

func main() {
	utils.ConfigurarLogger()
	http.HandleFunc("GET /savedPath/{path}", utils.ProcessSavedPathFromKernel)
	http.ListenAndServe(":8085", nil)
}
