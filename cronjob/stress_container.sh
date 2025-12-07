#!/bin/bash
# Script para crear contenedores de estrés

set -e

echo "🏋️ Creando contenedores para estresar CPU y RAM..."

# Función para crear contenedor de estrés de CPU
create_cpu_stress() {
    local container_name="stress-cpu-$1"
    echo "Creando contenedor de estrés CPU: $container_name"
    
    docker run -d --name $container_name \
        --rm \
        ubuntu:20.04 \
        bash -c "
            apt-get update && apt-get install -y stress-ng;
            stress-ng --cpu 1 --timeout 300s --metrics-brief
        "
}

# Función para crear contenedor de estrés de RAM
create_ram_stress() {
    local container_name="stress-ram-$1"
    echo "Creando contenedor de estrés RAM: $container_name"
    
    docker run -d --name $container_name \
        --rm \
        ubuntu:20.04 \
        bash -c "
            apt-get update && apt-get install -y stress-ng;
            stress-ng --vm 1 --vm-bytes 256M --timeout 300s --metrics-brief
        "
}

# Crear 5 contenedores de estrés de CPU
echo "🔥 Creando contenedores de estrés de CPU..."
for i in {1..5}; do
    create_cpu_stress $i
    sleep 2
done

# Crear 5 contenedores de estrés de RAM
echo "💾 Creando contenedores de estrés de RAM..."
for i in {1..5}; do
    create_ram_stress $i
    sleep 2
done

echo "✅ Se han creado 10 contenedores de estrés"
echo "📊 Para ver los contenedores ejecutándose:"
echo "   docker ps --filter name=stress-"
echo "⏱️ Los contenedores se ejecutarán por 5 minutos"
echo "🧹 Para limpiar manualmente: docker stop \$(docker ps -q --filter name=stress-)"