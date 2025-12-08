#!/usr/bin/env bash
# Script para crear contenedores de stress (alto y bajo consumo) CADA 1 MINUTO

set -e

# ==============================
# Nombres de las imágenes
# ==============================
IMAGE_HIGH_CPU="stress-high-cpu:so1"
IMAGE_HIGH_RAM="stress-high-ram:so1"
IMAGE_LOW_LOAD="stress-low:so1"

echo "Construyendo imágenes de Docker para las pruebas de consumo..."

# ==============================
# 1) Imagen de ALTO consumo CPU
# ==============================
echo "Construyendo imagen de ALTO consumo de CPU: ${IMAGE_HIGH_CPU}"
docker build -t "${IMAGE_HIGH_CPU}" - << 'EOF'
FROM ubuntu:20.04

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update \
    && apt-get install -y stress-ng \
    && rm -rf /var/lib/apt/lists/*

CMD ["bash", "-c", "echo 'Iniciando stress CPU'; stress-ng --cpu 2 --timeout 300s --metrics-brief"]
EOF

# ==============================
# 2) Imagen de ALTO consumo RAM
# ==============================
echo "Construyendo imagen de ALTO consumo de RAM: ${IMAGE_HIGH_RAM}"
docker build -t "${IMAGE_HIGH_RAM}" - << 'EOF'
FROM ubuntu:20.04

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update \
    && apt-get install -y stress-ng \
    && rm -rf /var/lib/apt/lists/*

CMD ["bash", "-c", "echo 'Iniciando stress RAM'; stress-ng --vm 1 --vm-bytes 256M --timeout 300s --metrics-brief"]
EOF

# ==============================
# 3) Imagen de BAJO consumo
# ==============================
echo "Construyendo imagen de BAJO consumo de CPU/RAM: ${IMAGE_LOW_LOAD}"
docker build -t "${IMAGE_LOW_LOAD}" - << 'EOF'
FROM ubuntu:20.04

ENV DEBIAN_FRONTEND=noninteractive

RUN printf '#!/bin/bash\nwhile true; do sleep 30; done\n' \
    > /usr/local/bin/stress-low \
    && chmod +x /usr/local/bin/stress-low

CMD ["stress-low"]
EOF

echo "✅ Imágenes construidas."
echo

# ==============================
# Función que crea UN contenedor
# ==============================
create_random_container() {
    local idx="$1"
    local pick=$((RANDOM % 3))
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
    echo "🧪 Creando contenedor: ${container_name}"

    docker run -d --rm --name "${container_name}" "${image}"
}

# ==============================
# LOOP INFINITO CADA 1 MINUTO ✅
# ==============================
echo "⏱️ Iniciando generador automático de contenedores (cada 60 segundos)..."
echo "👉 Presiona CTRL + C para detenerlo"
echo

while true; do
    echo "🚀 Generando 10 contenedores nuevos: $(date)"

    for i in $(seq 1 10); do
        create_random_container "${i}"
        sleep 1
    done

    echo "✅ Lote de 10 contenedores creado."
    echo "⏳ Esperando 60 segundos..."
    echo

    sleep 60
done
