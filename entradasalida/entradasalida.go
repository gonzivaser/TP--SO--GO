package main

import (
	"log"
	"net/http"
	"os"

	// "os"
	"strconv"

	"github.com/sisoputnfrba/tp-golang/entradasalida/globals"
	"github.com/sisoputnfrba/tp-golang/entradasalida/utils"
)

func main() {
	utils.ConfigurarLogger()
	globals.ClientConfig = utils.IniciarConfiguracion("config.json")

	if globals.ClientConfig == nil {
		log.Fatalf("No se pudo cargar la configuración")
	}

	puerto := globals.ClientConfig.Puerto

	// configFile := "config.json"
	// config, err := utils.CargarConfiguracion(configFile)
	// if err != nil {
	// 	log.Fatalf("Error al cargar la configuración: %v", err)
	// }

	// Crear la interfaz y pasar la configuración cargada
	// interfaz := utils.InterfazIO{
	// 	Nombre: "Generica",
	// 	Config: config,
	// }

	http.HandleFunc("GET /input", utils.Prueba)
	http.HandleFunc("GET /interfaz", utils.Iniciar)

	// Cargar la configuración desde el archivo

	interfaceName := os.Args[1]
	log.Printf("Nombre de la interfaz: %s", interfaceName)
	pathToConfig := os.Args[2]
	log.Printf("Path al archivo de configuración: %s", pathToConfig)

	file, err := os.Open(pathToConfig)
	check(err)

	fi, err := file.Stat()
	check(err)

	sliceBytes := make([]byte, fi.Size())      //Esta línea crea un slice de bytes
	numBytesRead, err := file.Read(sliceBytes) //es el número de bytes leídos
	check(err)
	log.Printf("%d bytes: %s\n", numBytesRead, string(sliceBytes[:numBytesRead])) 
	file.Close()

	// Iniciar la interfaz
	//interfaz.Iniciar()
	http.ListenAndServe(":"+strconv.Itoa(puerto), nil)
}

func check(e error) {
	if e != nil {
		panic(e)
	}
}
