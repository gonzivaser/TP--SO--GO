package main

import (
	"log"
	"net/http"
	"strconv"

	"github.com/sisoputnfrba/tp-golang/memoria/globals"
	"github.com/sisoputnfrba/tp-golang/memoria/utils"
)

func main() {
	utils.ConfigurarLogger()
	globals.ClientConfig = utils.IniciarConfiguracion("config.json")

	if globals.ClientConfig == nil {
		log.Fatalf("No se pudo cargar la configuración")
	}

	puerto := globals.ClientConfig.Puerto

	http.HandleFunc("POST /setInstructionFromFileToMap", utils.SetInstructionsFromFileToMap)
	http.HandleFunc("GET /getInstructionFromPid", utils.GetInstruction)
	http.HandleFunc("POST /SendInputSTDINToMemory", utils.RecieveInputSTDINFromIO)
	http.HandleFunc("POST /SendAdressSTDOUTToMemory", utils.RecieveAdressSTDOUTFromIO)

	http.ListenAndServe(":"+strconv.Itoa(puerto), nil)
}
