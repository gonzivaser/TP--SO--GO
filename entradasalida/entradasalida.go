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
	interfaceName := os.Args[1]
	log.Printf("Nombre de la interfaz: %s", interfaceName)
	pathToConfig := os.Args[2]
	log.Printf("Path al archivo de configuración: %s", pathToConfig)

	//http.HandleFunc("GET /input", utils.Prueba)
	http.HandleFunc("GET /interfaz", utils.Iniciar)

	// Cargar la configuración desde el archivo

	http.ListenAndServe(":"+strconv.Itoa(puerto), nil)
}

// unidades := 5
// duración := gi.IO_GEN_SLEEP(unidades)
// fmt.Printf("La espera por %d unidades para la interfaz '%s' es de %v\n", unidades, gi.Nombre, duración)

/*func readConfigFile(path string) []byte {
	file, err := os.Open(path)
	check(err)

	fi, err := file.Stat()
	check(err)

	sliceBytes := make([]byte, fi.Size())      //Esta línea crea un slice de bytes
	numBytesRead, err := file.Read(sliceBytes) //es el número de bytes leídos
	check(err)
	log.Printf("%d bytes: %s\n", numBytesRead, string(sliceBytes[:numBytesRead]))
	file.Close()
	return sliceBytes

}*/
