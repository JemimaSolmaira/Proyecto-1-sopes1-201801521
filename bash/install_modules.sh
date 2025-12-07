#!/usr/bin/env bash
# install_modules.sh - Compila y carga el módulo del kernel del proyecto
# Ejecutar desde cualquier ruta: ./install_modules.sh

set -euo pipefail
IFS=$'\n\t'

# Configurables
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# Si tu repo está un nivel arriba en "kernel/", ajusta:
KERNEL_MODULE_DIR="${SCRIPT_DIR}/../kernel"
# Nombre base del módulo (sin .ko). Cámbialo si tu Makefile genera otro nombre.
MODULE_BASE="sysinfo_so1_201801521"
KO_FILE="${MODULE_BASE}.ko"

echo "🏷 Script dir: ${SCRIPT_DIR}"
echo "📁 Módulo dir: ${KERNEL_MODULE_DIR}"

# 1) Comprobar que estamos donde se espera
if [ ! -d "${KERNEL_MODULE_DIR}" ]; then
  echo "❌ Error: no se encontró el directorio del módulo: ${KERNEL_MODULE_DIR}"
  exit 2
fi

# 2) Instalar dependencias (requiere internet y permisos sudo)
echo "🔧 Instalando dependencias del sistema (build-essential, headers)..."
sudo apt-get update
sudo apt-get install -y build-essential linux-headers-"$(uname -r)" make gcc

# 3) Compilar el módulo
echo "📦 Compilando módulo en ${KERNEL_MODULE_DIR} ..."
pushd "${KERNEL_MODULE_DIR}" > /dev/null

# Limpieza y build (asume Makefile correcto en kernel/)
if [ -f Makefile ] || [ -f makefile ]; then
  make clean || true
  make
else
  echo "❗ No se encontró Makefile en ${KERNEL_MODULE_DIR}. Ajusta el script."
  popd > /dev/null
  exit 3
fi

# Verificar que se generó el .ko
if [ ! -f "${KO_FILE}" ]; then
  # Buscar cualquier .ko generado y usar el primero
  altko=$(ls -1 *.ko 2>/dev/null | head -n1 || true)
  if [ -n "${altko}" ]; then
    KO_FILE="${altko}"
    MODULE_BASE="${KO_FILE%.ko}"
    echo "ℹ️ Usando módulo generado: ${KO_FILE}"
  else
    echo "❌ No se encontró ningún .ko en ${KERNEL_MODULE_DIR}"
    popd > /dev/null
    exit 4
  fi
fi

# 4) Si el módulo ya está cargado, descargarlo primero (para recargar)
if lsmod | grep -q "^${MODULE_BASE}[[:space:]]"; then
  echo "🔁 Módulo ${MODULE_BASE} ya cargado: descargando primero..."
  sudo /sbin/modprobe -r "${MODULE_BASE}" || sudo /sbin/rmmod "${MODULE_BASE}" || true
  sleep 1
fi

# 5) Cargar el módulo (usar modprobe si lo instalaste en /lib/modules, sino insmod)
echo "🚀 Cargando módulo: ${KO_FILE}"
sudo insmod "${KO_FILE}"

# 6) Verificar carga
echo "✅ Verificando módulo cargado (lsmod):"
lsmod | grep -E "^${MODULE_BASE}[[:space:]]" || {
  echo "⚠️  No aparece en lsmod. Revisa dmesg:"
  sudo dmesg | tail -n 20
  popd > /dev/null
  exit 5
}

# 7) Verificar archivo en /proc
echo "📁 Verificando archivo en /proc:"
if ls -la /proc/ | grep -q "${MODULE_BASE}"; then
  echo "✅ Archivo /proc/${MODULE_BASE} presente"
else
  echo "⚠️ Archivo /proc/${MODULE_BASE} no encontrado. Revisa dmesg:"
  sudo dmesg | tail -n 20
fi

popd > /dev/null

echo "🎉 Módulo instalado y cargado correctamente."

