#!/bin/bash

# Verificar si se pasaron los argumentos necesarios
if [ "$#" -ne 2 ]; then
    echo "Uso: $0 <PID> <PATH>"
    exit 1
fi

# Asignar los argumentos a variables
PID="$1"
FILE_PATH="$2"

# URL del servidor
URL="http://192.168.0.91:8080/process"

# Cuerpo JSON
BODY="{\"pid\": $PID, \"path\": \"$FILE_PATH\"}"

# Imprimir la URL y el cuerpo JSON para depuración
echo "URL: $URL"
echo "Cuerpo JSON: $BODY"

# Realizar la petición PUT con curl
curl -X PUT "$URL" -H "Content-Type: application/json" -d "$BODY"
