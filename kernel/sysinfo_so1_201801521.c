// sysinfo_so1_201801521.c  (versión corregida para evitar get_fs/set_fs y strtok_r)
#include <linux/init.h>
#include <linux/module.h>
#include <linux/kernel.h>
#include <linux/proc_fs.h>
#include <linux/seq_file.h>
#include <linux/sched/signal.h>
#include <linux/mm.h>
#include <linux/ktime.h>
#include <linux/timekeeping.h>
#include <linux/rcupdate.h>
#include <linux/slab.h>
#include <linux/fs.h>
#include <linux/uaccess.h>
#include <linux/sched/cputime.h>


#define PROC_NAME "sysinfo_so1_201801521"

MODULE_LICENSE("GPL");
MODULE_AUTHOR("201801521");
MODULE_DESCRIPTION("Modulo de monitoreo de sistema: RAM, CPU y procesos (versión corregida)");
MODULE_VERSION("1.2.1");

static struct proc_dir_entry *proc_entry;
static unsigned long prev_idle = 0;
static unsigned long prev_total = 0;

/* --- Helper: leer /proc/stat y calcular cpu usage % --- */
static void get_cpu_usage(unsigned long *usage_percent)
{
    struct file *file;
    char *buf;
    loff_t pos = 0;
    ssize_t n;
    unsigned long user = 0, nice = 0, system = 0, idle = 0, iowait = 0, irq = 0, softirq = 0, steal = 0;
    unsigned long total = 0, total_idle = 0, diff_idle = 0, diff_total = 0;

    buf = kmalloc(1024, GFP_KERNEL);
    if (!buf) {
        *usage_percent = 0;
        return;
    }

    file = filp_open("/proc/stat", O_RDONLY, 0);
    if (IS_ERR(file)) {
        kfree(buf);
        *usage_percent = 0;
        return;
    }

    /* kernel_read está disponible en kernels recientes */
    n = kernel_read(file, buf, 1023, &pos);
    if (n <= 0) {
        filp_close(file, NULL);
        kfree(buf);
        *usage_percent = 0;
        return;
    }
    buf[n] = '\0';
    filp_close(file, NULL);

    if (sscanf(buf, "cpu %lu %lu %lu %lu %lu %lu %lu %lu",
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

    kfree(buf);
}

/* --- Helper: parsear /proc/meminfo para obtener MemTotal, MemFree, MemAvailable en kB --- */
static void get_meminfo_kb(unsigned long *total_kb, unsigned long *free_kb, unsigned long *available_kb)
{
    struct file *file;
    char *buf, *p, *line;
    loff_t pos = 0;
    ssize_t n;

    *total_kb = 0;
    *free_kb = 0;
    *available_kb = 0;

    buf = kmalloc(8192, GFP_KERNEL);
    if (!buf)
        return;

    file = filp_open("/proc/meminfo", O_RDONLY, 0);
    if (IS_ERR(file)) {
        kfree(buf);
        return;
    }

    n = kernel_read(file, buf, 8191, &pos);
    if (n <= 0) {
        filp_close(file, NULL);
        kfree(buf);
        return;
    }
    buf[n] = '\0';
    filp_close(file, NULL);

    /* recorrer líneas usando strsep (disponible en kernel) */
    p = buf;
    while ((line = strsep(&p, "\n")) != NULL) {
        unsigned long val = 0;
        if (sscanf(line, "MemTotal: %lu kB", &val) == 1) {
            *total_kb = val;
        } else if (sscanf(line, "MemFree: %lu kB", &val) == 1) {
            *free_kb = val;
        } else if (sscanf(line, "MemAvailable: %lu kB", &val) == 1) {
            *available_kb = val;
        }
    }

    kfree(buf);
}

/* Helper: estado simplificado de task
   NOTA: en algunas versiones del kernel el campo `task->state` o su nombre puede cambiar.
   Para máxima portabilidad devolvemos '?' como fallback. Si en tu kernel existe task->state,
   puedes activar la lectura (ver comentarios). */
static char task_state_char(struct task_struct *task)
{
#ifdef TASK_RUNNING
    /* Intento leer state de forma segura si el miembro existe; en kernels donde no existe,
       el compilador dará error y este bloque será ignorado si TASK_RUNNING no está definido. */
    long state = 0;
    /* try to read if available - use READ_ONCE to be safe */
    /* This may still fail on some kernels; if it does, we fallback below */
#ifdef HAVE_TASK_STATE_FIELD
    state = READ_ONCE(task->state);
    if (state == TASK_RUNNING)
        return 'R';
    if (state == TASK_UNINTERRUPTIBLE)
        return 'D';
    if (state == TASK_STOPPED || state == TASK_TRACED)
        return 'T';
    if (state == EXIT_ZOMBIE || state == EXIT_DEAD)
        return 'Z';
#endif
#endif
    /* Fallback genérico: no dependemos del campo interno */
    return '?';
}

/* --- Función principal: imprime JSON con global + lista de procesos --- */
static int sysinfo_show(struct seq_file *m, void *v)
{
    unsigned long total_ram_kb = 0, free_ram_kb = 0, available_kb = 0;
    unsigned long used_ram_kb = 0;
    unsigned long cpu_usage_pct = 0;
    struct timespec64 ts;
    unsigned long long ts_ms = 0;
    struct task_struct *task;
    long total_procs = 0;
    bool first_proc = true;

    /* obtener memoria (kB) */
    get_meminfo_kb(&total_ram_kb, &free_ram_kb, &available_kb);
    if (available_kb == 0) {
        available_kb = free_ram_kb;
    }
    used_ram_kb = (total_ram_kb > available_kb) ? (total_ram_kb - available_kb) : 0;

    /* cpu usage */
    get_cpu_usage(&cpu_usage_pct);

    /* timestamp ms */
    ktime_get_real_ts64(&ts);
    ts_ms = (unsigned long long)ts.tv_sec * 1000ULL +
            (unsigned long long)(ts.tv_nsec / 1000000ULL);

    /* contar procesos en una pasada */
    rcu_read_lock();
    for_each_process(task) {
        total_procs++;
    }
    rcu_read_unlock();

    /* imprimir cabecera JSON */
    seq_printf(m, "{\n");
    seq_printf(m, "  \"total_ram_kb\": %lu,\n", total_ram_kb);
    seq_printf(m, "  \"free_ram_kb\": %lu,\n", free_ram_kb);
    seq_printf(m, "  \"available_kb\": %lu,\n", available_kb);
    seq_printf(m, "  \"ram_used_kb\": %lu,\n", used_ram_kb);
    seq_printf(m, "  \"total_procs\": %ld,\n", total_procs);
    seq_printf(m, "  \"cpu_usage_pct\": %lu,\n", cpu_usage_pct);
    seq_printf(m, "  \"ts_ms\": %llu,\n", ts_ms);

    /* iniciar array de procesos */
    seq_printf(m, "  \"procesos\": [\n");

    /* segunda pasada: iterar procesos e imprimir objetos sin coma inicial */
    rcu_read_lock();
    for_each_process(task) {
        pid_t pid = task_pid_nr(task);
        char comm[TASK_COMM_LEN];
        unsigned long rss_kb = 0;
        unsigned long vmsize_kb = 0;
        unsigned long long utime_val = 0;
unsigned long long stime_val = 0;
char state_ch = '?';

{
    u64 utime_tmp, stime_tmp;
    task_cputime_adjusted(task, &utime_tmp, &stime_tmp);
    utime_val = (unsigned long long)utime_tmp;
    stime_val = (unsigned long long)stime_tmp;
}
        struct mm_struct *mm = NULL;

        /* nombre del proceso */
        get_task_comm(comm, task);

        /* mm para rss/vsize */
        mm = get_task_mm(task);
        if (mm) {
            /* get_mm_rss devuelve páginas residentes */
            rss_kb    = (unsigned long)get_mm_rss(mm) * (PAGE_SIZE / 1024);
            /* total_vm también está en páginas */
            vmsize_kb = (unsigned long)mm->total_vm * (PAGE_SIZE / 1024);
            mmput(mm);
        } else {
            /* hilos de kernel normalmente caen aquí: sin espacio de usuario */
            rss_kb    = 0;
            vmsize_kb = 0;
        }

        /* utime/stime: por ahora se dejan en 0 hasta adaptar a tu versión de kernel */
        /* state: por ahora '?' hasta adaptar a tu versión de kernel */

        if (!first_proc)
            seq_printf(m, "    ,\n");
        else
            first_proc = false;

        seq_printf(m,
            "    { \"pid\": %d, \"comm\": \"%s\", \"rss_kb\": %lu, \"vmsize_kb\": %lu, \"state\": \"%c\", \"utime\": %llu, \"stime\": %llu, \"ts_ms\": %llu }",
            pid, comm, rss_kb, vmsize_kb, state_ch, utime_val, stime_val, ts_ms);
    }
    rcu_read_unlock();

    /* finalizar array y objeto */
    seq_printf(m, "\n  ]\n");
    seq_printf(m, "}\n");

    return 0;
}

static int sysinfo_open(struct inode *inode, struct file *file)
{
    return single_open(file, sysinfo_show, NULL);
}

static const struct proc_ops sysinfo_proc_ops = {
    .proc_open = sysinfo_open,
    .proc_read = seq_read,
    .proc_lseek = seq_lseek,
    .proc_release = single_release,
};

static int __init sysinfo_init(void)
{
    proc_entry = proc_create(PROC_NAME, 0444, NULL, &sysinfo_proc_ops);
    if (!proc_entry) {
        printk(KERN_ERR "No se pudo crear /proc/%s\n", PROC_NAME);
        return -ENOMEM;
    }
    printk(KERN_INFO "Modulo sysinfo cargado: /proc/%s\n", PROC_NAME);
    return 0;
}

static void __exit sysinfo_exit(void)
{
    proc_remove(proc_entry);
    printk(KERN_INFO "Modulo sysinfo descargado\n");
}

module_init(sysinfo_init);
module_exit(sysinfo_exit);
