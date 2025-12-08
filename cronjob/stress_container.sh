#!/usr/bin/env bash
# Script para crear contenedores de stress (alto y bajo consumo)

set -e

# Nombres de las imágenes
IMAGE_HIGH_CPU="stress-high-cpu:so1"
IMAGE_HIGH_RAM="stress-high-ram:so1"
IMAGE_LOW_LOAD="stress-low:so1"

echo "Construyendo imágenes de Docker para las pruebas de consumo..."

# 1) Imagen de alto consumo de CPU
echo "Construyendo imagen de ALTO consumo de CPU: ${IMAGE_HIGH_CPU}"
docker build -t "${IMAGE_HIGH_CPU}" - << 'EOF'
FROM ubuntu:20.04

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update \
    && apt-get install -y stress-ng \
    && rm -rf /var/lib/apt/lists/*

# Alto consumo de CPU: 2 workers durante 5 minutos
CMD ["bash", "-c", "echo 'Iniciando stress CPU'; stress-ng --cpu 2 --timeout 300s --metrics-brief"]
EOF

# 2) Imagen de alto consumo de RAM
echo "Construyendo imagen de ALTO consumo de RAM: ${IMAGE_HIGH_RAM}"
docker build -t "${IMAGE_HIGH_RAM}" - << 'EOF'
FROM ubuntu:20.04

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update \
    && apt-get install -y stress-ng \
    && rm -rf /var/lib/apt/lists/*

# Alto consumo de RAM: usa ~256 MB durante 5 minutos
CMD ["bash", "-c", "echo 'Iniciando stress RAM'; stress-ng --vm 1 --vm-bytes 256M --timeout 300s --metrics-brief"]
EOF

# 3) Imagen de BAJO consumo de CPU y RAM
echo "Construyendo imagen de BAJO consumo de CPU/RAM: ${IMAGE_LOW_LOAD}"
docker build -t "${IMAGE_LOW_LOAD}" - << 'EOF'
FROM ubuntu:20.04

ENV DEBIAN_FRONTEND=noninteractive

# Contenedor ligero: solo duerme en un loop infinito
CMD ["bash", "-c", "echo 'Contenedor de bajo consumo ejecutándose'; while true; do sleep 30; done"]
EOF

echo "Imágenes construidas."
echo

# Función que crea un contenedor a partir de un tipo elegido
create_random_container() {
    local idx="$1"
    local pick=$((RANDOM % 3))  # 0, 1 o 2
    local image=""
    local tipo=""

    case "${pick}" in
        0)
            image="${IMAGE_HIGH_CPU}"
            tipo="high-cpu"
            ;;
        1)
            image="${IMAGE_HIGH_RAM}"
            tipo="high-ram"
            ;;
        2)
            image="${IMAGE_LOW_LOAD}"
            tipo="low"
            ;;
    esac

    local container_name="stress-${tipo}-${idx}"
    echo "Creando contenedor #${idx}: ${container_name} (imagen: ${image})"

    docker run -d --rm --name "${container_name}" "${image}"
}

echo "Creando 10 contenedores de manera ALEATORIA usando las 3 imágenes..."
for i in $(seq 1 10); do
    create_random_container "${i}"
    sleep 1
done

echo
echo "Se han creado 10 contenedores (mezcla aleatoria de alto CPU, alto RAM y bajo consumo)."
echo "Para ver los contenedores ejecutándose:"
echo "   docker ps --filter 'name=stress-'"
echo
echo "Para limpiar manualmente todos los contenedores de prueba:"
echo "   docker stop \$(docker ps -q --filter 'name=stress-')"
