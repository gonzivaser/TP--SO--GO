#!/bin/bash

if [ -z "$KERNEL_PORT" ]; then
    echo "No se ha definido la variable KERNEL_PORT"
    echo "Usando puerto por defecto 8080"
    KERNEL_PORT=8080
fi

if [ -z "$KERNEL_HOST" ]; then
    echo "No se ha definido la variable KERNEL_HOST"
    echo "Usando HOST por defecto localhost"
    KERNEL_PORT=localhost
fi

KERNEL_URL="http://$KERNEL_HOST:$KERNEL_PORT"

# Verificar si se pasaron los argumentos necesarios
if [ "$#" -ne 2 ]; then #Si la cantidad de argumentos es diferente de 2
    echo "Uso: $0 <PID> <PATH>"
    exit 1
fi

# Asignar los argumentos a variables
PID="$1"
PATH="$2"

# Cuerpo JSON
BODY=$(cat <<EOF
{
    "pid": $PID,
    "path": "$PATH"
}
EOF
)

# Realizar la petición POST con curl
curl -X POST "${KERNEL_URL}" -H "Content-Type: application/json" -d "$BODY"
