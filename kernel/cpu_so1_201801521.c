#include <linux/init.h>
#include <linux/module.h>
#include <linux/kernel.h>
#include <linux/proc_fs.h>
#include <linux/uaccess.h>
#include <linux/seq_file.h>
#include <linux/sched.h>
#include <linux/sched/signal.h>
#include <linux/mm.h>
#include <linux/timer.h>
#include <linux/jiffies.h>

#define PROC_NAME "cpu_so1_201801521"

MODULE_LICENSE("GPL");
MODULE_AUTHOR("201801521");
MODULE_DESCRIPTION("Modulo de monitoreo de CPU");
MODULE_VERSION("1.0");

static struct proc_dir_entry *proc_entry;
static unsigned long prev_idle = 0;
static unsigned long prev_total = 0;

static void get_cpu_usage(unsigned long *usage_percent)
{
    struct file *file;
    char buffer[256];
    unsigned long user, nice, system, idle, iowait, irq, softirq, steal;
    unsigned long total, total_idle, diff_idle, diff_total;
    loff_t pos = 0;
    
    // Leer /proc/stat
    file = filp_open("/proc/stat", O_RDONLY, 0);
    if (IS_ERR(file)) {
        *usage_percent = 0;
        return;
    }
     
    
    kernel_read(file, buffer, sizeof(buffer) - 1, &pos);
    buffer[255] = '\0';
    filp_close(file, NULL);
    
    // Parsear la primera línea (cpu general)
    if (sscanf(buffer, "cpu %lu %lu %lu %lu %lu %lu %lu %lu",
               &user, &nice, &system, &idle, &iowait, &irq, &softirq, &steal) == 8) {
        
        total_idle = idle + iowait;
        total = user + nice + system + idle + iowait + irq + softirq + steal;
        
        diff_idle = total_idle - prev_idle;
        diff_total = total - prev_total;
        
        if (diff_total != 0) {
            *usage_percent = (1000 * (diff_total - diff_idle) / diff_total + 5) / 10;
        } else {
            *usage_percent = 0;
        }
        
        prev_idle = total_idle;
        prev_total = total;
    } else {
        *usage_percent = 0;
    }
}

static int cpu_show(struct seq_file *m, void *v)
{
    unsigned long cpu_usage = 0;
    
    get_cpu_usage(&cpu_usage);
    
    seq_printf(m, "{\n");
    seq_printf(m, "  \"porcentajeUso\": %lu\n", cpu_usage);
    seq_printf(m, "}\n");
    
    return 0;
}

static int cpu_open(struct inode *inode, struct file *file)
{
    return single_open(file, cpu_show, NULL);
}

static const struct proc_ops cpu_proc_ops = {
    .proc_open = cpu_open,
    .proc_read = seq_read,
    .proc_lseek = seq_lseek,
    .proc_release = single_release,
};

static int __init cpu_so1_201801521_init(void)
{
    proc_entry = proc_create(PROC_NAME, 0444, NULL, &cpu_proc_ops);
    if (!proc_entry) {
        printk(KERN_ERR "No se pudo crear /proc/%s\n", PROC_NAME);
        return -ENOMEM;
    }
    
    printk(KERN_INFO "Modulo CPU cargado: /proc/%s\n", PROC_NAME);
    return 0;
}

static void __exit cpu_so1_201801521_exit(void)
{
    proc_remove(proc_entry);
    printk(KERN_INFO "Modulo CPU descargado\n");
}

module_init(cpu_so1_201801521_init);
module_exit(cpu_so1_201801521_exit);