package main

import (
	"net/http"
	"server/utils"
)

func main() {
	http.HandleFunc("POST /helloworld", utils.HelloWorld)
	http.ListenAndServe(":8080", nil)
}
