package main

import (
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/sisoputnfrba/tp-golang/cpu/globals"
	"github.com/sisoputnfrba/tp-golang/cpu/utils"
)

func main() {
	utils.ConfigurarLogger()
	globals.ClientConfig = utils.InitializeConfiguracion(os.Args[1])

	if globals.ClientConfig == nil {
		log.Fatalf("No se pudo cargar la configuración")
	}
	puerto := globals.ClientConfig.Puerto

	http.HandleFunc("/receivePCB", utils.ReceiveContextFromKernel)
	http.HandleFunc("POST /receiveDataFromMemory", utils.RecieveMOV_INFromMemory)
	http.HandleFunc("/interrupt", utils.CheckinterruptsFromKernel)
	http.HandleFunc("/recievePageTam", utils.ReceiveTamPageFromMemory)
	http.HandleFunc("POST /recieveFrame", utils.RecieveFramefromMemory)
	http.ListenAndServe(":"+strconv.Itoa(puerto), nil)
}
