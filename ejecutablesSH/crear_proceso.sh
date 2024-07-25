#!/bin/bash

# Verificar si se ha definido la variable KERNEL_PORT
if [ -z "$KERNEL_PORT" ]; then
    echo "No se ha definido la variable KERNEL_PORT"
    echo "Usando puerto por defecto 8080"
    KERNEL_PORT=8080
fi

# Verificar si se ha definido la variable KERNEL_HOST
if [ -z "$KERNEL_HOST" ]; then
    echo "No se ha definido la variable KERNEL_HOST"
    echo "Usando HOST por defecto localhost"
    KERNEL_HOST=localhost
fi

KERNEL_URL="http://$KERNEL_HOST:$KERNEL_PORT"

# Verificar si se pasaron los argumentos necesarios
if [ "$#" -ne 2 ]; then
    echo "Uso: $0 <PID> <PATH>"
    exit 1
fi

PID="$1"
PATH="$2"

BODY="{\"pid\": $PID, \"path\": \"$PATH\"}"

echo "URL: $KERNEL_URL"
echo "Cuerpo JSON: $BODY"

# Realizar la petición POST con curl
curl -X POST "$KERNEL_URL" -H "Content-Type: application/json" -d "$BODY"
