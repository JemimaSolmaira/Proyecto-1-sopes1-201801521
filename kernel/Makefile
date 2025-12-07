obj-m += sysinfo_so1_201801521.o
obj-m += continfo_so1_201801521.o


KERNEL_DIR = /lib/modules/$(shell uname -r)/build
PWD = $(shell pwd)

all:
	make -C $(KERNEL_DIR) M=$(PWD) modules

clean:
	make -C $(KERNEL_DIR) M=$(PWD) clean

install:
	sudo insmod sysinfo_so1_201801521.ko
	sudo insmod continfo_so1_201801521.ko

uninstall:
	sudo rmmod sysinfo_so1_201801521
	sudo rmmod continfo_so1_201801521

test:
	cat /proc/sysinfo_so1_201801521
	cat /proc/continfo_so1_201801521

.PHONY: all clean install uninstall test



