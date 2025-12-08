#!/usr/bin/env bash
# install_modules.sh - Compila y carga los módulos del kernel del proyecto

set -euo pipefail
IFS=$'\n\t'

# Directorios
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
KERNEL_MODULE_DIR="${SCRIPT_DIR}/../kernel"

# Nombres de los módulos (sin .ko) y sus archivos /proc
MODULES=( "sysinfo_so1_201801521" "continfo_so1_201801521" )

echo "🏷 Script dir:  ${SCRIPT_DIR}"
echo "📁 Kernel dir:  ${KERNEL_MODULE_DIR}"
echo "🧩 Módulos:     ${MODULES[*]}"

# 1) Verificar que exista el directorio de kernel
if [ ! -d "${KERNEL_MODULE_DIR}" ]; then
  echo "❌ Error: no se encontró el directorio del módulo: ${KERNEL_MODULE_DIR}"
  exit 2
fi

# 2) Instalar dependencias
echo "🔧 Instalando dependencias del sistema (build-essential, headers)..."
sudo apt-get update
sudo apt-get install -y build-essential linux-headers-"$(uname -r)" make gcc

# 3) Compilar los módulos
echo "📦 Compilando módulos en ${KERNEL_MODULE_DIR} ..."
pushd "${KERNEL_MODULE_DIR}" > /dev/null

if [ -f Makefile ] || [ -f makefile ]; then
  make clean || true
  make
else
  echo "❗ No se encontró Makefile en ${KERNEL_MODULE_DIR}. Ajusta el script."
  popd > /dev/null
  exit 3
fi

# 4) (Re)cargar cada módulo de la lista
for MODULE_BASE in "${MODULES[@]}"; do
  KO_FILE="${MODULE_BASE}.ko"

  if [ ! -f "${KO_FILE}" ]; then
    echo "No se encontró ${KO_FILE} después de compilar. Revisa el Makefile."
    continue
  fi

  # Si ya está cargado, descargarlo primero
  if lsmod | grep -q "^${MODULE_BASE}[[:space:]]"; then
    echo "Módulo ${MODULE_BASE} ya cargado: descargando primero..."
    sudo /sbin/modprobe -r "${MODULE_BASE}" 2>/dev/null || \
    sudo /sbin/rmmod "${MODULE_BASE}" 2>/dev/null || true
    sleep 1
  fi

  echo "Cargando módulo: ${KO_FILE}"
  sudo insmod "${KO_FILE}"

  echo "Verificando módulo cargado (lsmod):"
  if ! lsmod | grep -q "^${MODULE_BASE}[[:space:]]"; then
    echo "⚠️  ${MODULE_BASE} no aparece en lsmod. Revisa dmesg:"
    sudo dmesg | tail -n 20
  fi

  # Verificar archivo en /proc (usa el mismo nombre que MODULE_BASE)
  PROC_FILE="/proc/${MODULE_BASE}"
  echo "Verificando archivo en ${PROC_FILE}:"
  if [ -e "${PROC_FILE}" ]; then
    echo "    ${PROC_FILE} presente"
  else
    echo "   ${PROC_FILE} no encontrado. Revisa dmesg:"
    sudo dmesg | tail -n 20
  fi

  echo
done

popd > /dev/null

echo "Módulos instalados y cargados correctamente."
