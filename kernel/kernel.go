package main

import (
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/sisoputnfrba/tp-golang/kernel/globals"
	"github.com/sisoputnfrba/tp-golang/kernel/utils"
)

func main() {
	utils.ConfigurarLogger()
	globals.ClientConfig = utils.IniciarConfiguracion(os.Args[1])

	if globals.ClientConfig == nil {
		log.Fatalf("No se pudo cargar la configuración")
	}

	puerto := globals.ClientConfig.Puerto

	http.HandleFunc("PUT /process", utils.InitializeProcess)
	http.HandleFunc("POST /syscall", utils.ProcessSyscallFromCPU)
	http.HandleFunc("POST /SendPortOfInterfaceToKernel", utils.RecievePortOfInterfaceFromIO)
	http.HandleFunc("POST /recieveREG", utils.RecieveREGFromCPU)
	http.HandleFunc("POST /recieveFSDATA", utils.RecieveFileNameFromCPU)
	http.HandleFunc("DELETE /process", utils.FinishProcess)
	http.HandleFunc("POST /wait", utils.RecieveWaitFromCPU)
	http.HandleFunc("POST /signal", utils.RecieveSignalFromCPU)
	http.HandleFunc("GET /process/{pid}", utils.GetProcessState)
	http.HandleFunc("PUT /plani", utils.RestopPlanification)
	http.HandleFunc("DELETE /plani", utils.StopPlanification)
	http.HandleFunc("GET /process", utils.ListProcesses)
	http.ListenAndServe(":"+strconv.Itoa(puerto), nil)
}
