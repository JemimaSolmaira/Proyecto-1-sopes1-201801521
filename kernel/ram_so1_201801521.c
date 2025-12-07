#include <linux/init.h>
#include <linux/module.h>
#include <linux/kernel.h>
#include <linux/proc_fs.h>
#include <linux/uaccess.h>
#include <linux/seq_file.h>
#include <linux/mm.h>
#include <linux/sysinfo.h>

#define PROC_NAME "ram_so1_201801521"

MODULE_LICENSE("GPL");
MODULE_AUTHOR("201801521");
MODULE_DESCRIPTION("Modulo de monitoreo de RAM");
MODULE_VERSION("1.0");

static struct proc_dir_entry *proc_entry;

static int ram_show(struct seq_file *m, void *v)
{
    struct sysinfo si;
    unsigned long total_ram, free_ram, used_ram, porcentaje;
    
    si_meminfo(&si);
    
    // Convertir de páginas a KB
    total_ram = si.totalram * si.mem_unit / 1024;
    free_ram = si.freeram * si.mem_unit / 1024;
    used_ram = total_ram - free_ram;
    
    // Calcular porcentaje de uso
    if (total_ram > 0) {
        porcentaje = (used_ram * 100) / total_ram;
    } else {
        porcentaje = 0;
    }
    
    seq_printf(m, "{\n");
    seq_printf(m, "  \"total\": %lu,\n", total_ram);
    seq_printf(m, "  \"libre\": %lu,\n", free_ram);
    seq_printf(m, "  \"uso\": %lu,\n", used_ram);
    seq_printf(m, "  \"porcentaje\": %lu\n", porcentaje);
    seq_printf(m, "}\n");
    
    return 0;
}

static int ram_open(struct inode *inode, struct file *file)
{
    return single_open(file, ram_show, NULL);
}

static const struct proc_ops ram_proc_ops = {
    .proc_open = ram_open,
    .proc_read = seq_read,
    .proc_lseek = seq_lseek,
    .proc_release = single_release,
};

static int __init ram_so1_201801521_init(void)
{
    proc_entry = proc_create(PROC_NAME, 0444, NULL, &ram_proc_ops);
    if (!proc_entry) {
        printk(KERN_ERR "No se pudo crear /proc/%s\n", PROC_NAME);
        return -ENOMEM;
    }
    
    printk(KERN_INFO "Modulo RAM cargado: /proc/%s\n", PROC_NAME);
    return 0;
}

static void __exit ram_so1_201801521_exit(void)
{
    proc_remove(proc_entry);
    printk(KERN_INFO "Modulo RAM descargado\n");
}

module_init(ram_so1_201801521_init);
module_exit(ram_so1_201801521_exit);