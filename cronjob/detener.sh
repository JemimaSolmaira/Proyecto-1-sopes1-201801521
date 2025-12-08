#!/usr/bin/env bash
# DETENER CONTENEDORES DE ESTRÉS (stress-*)

set -e

echo "Deteniendo TODOS los contenedores de stress"

CONTAINERS=$(docker ps -q --filter "name=stress-")

if [ -z "$CONTAINERS" ]; then
    echo " No hay contenedores de stress en ejecución."
    exit 0
fi

echo "🧾 Contenedores a detener:"
docker ps --filter "id=${CONTAINERS}" --format "  - {{.Names}} ({{.ID}})"

docker stop $CONTAINERS

echo "Todos los contenedores de stress han sido detenidos correctamente."