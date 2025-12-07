savedcmd_sysinfo_so1_201801521.mod := printf '%s\n'   sysinfo_so1_201801521.o | awk '!x[$$0]++ { print("./"$$0) }' > sysinfo_so1_201801521.mod
