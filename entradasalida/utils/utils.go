package utils

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
)

type PruebaMensaje struct {
	Mensaje string `json:"Prueba"`
}

func ConfigurarLogger() {
	logFile, err := os.OpenFile("entradasalida.log", os.O_CREATE|os.O_APPEND|os.O_RDWR, 0666)
	if err != nil {
		panic(err)
	}
	mw := io.MultiWriter(os.Stdout, logFile)
	log.SetOutput(mw)
}

func Prueba(w http.ResponseWriter, r *http.Request) {

	Prueba := PruebaMensaje{
		Mensaje: "Todo OK IO",
	}

	pruebaResponse, _ := json.Marshal(Prueba)

	w.WriteHeader(http.StatusOK)
	w.Write(pruebaResponse)
}
