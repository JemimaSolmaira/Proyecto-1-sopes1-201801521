// proc_procs_containers.c
#include <linux/init.h>
#include <linux/module.h>
#include <linux/kernel.h>
#include <linux/seq_file.h>
#include <linux/proc_fs.h>
#include <linux/sched/signal.h>
#include <linux/mm.h>
#include <linux/sched.h>
#include <linux/uaccess.h>
#include <linux/slab.h>
#include <linux/sched/signal.h>
#include <linux/sizes.h>
#include <linux/mm_types.h>
#include <linux/swap.h>
#include <linux/sysinfo.h>
#include <linux/timekeeping.h>
#include <linux/sched/cputime.h>
#include <linux/string.h>


#define PROC_NAME "continfo_so1_201801521"
#define CMDLINE_MAX 1024

MODULE_LICENSE("GPL");
MODULE_AUTHOR("201801521");
MODULE_DESCRIPTION("Listado de procesos de contenedores");
MODULE_VERSION("1.0");

static struct proc_dir_entry *proc_entry;

/* Trata de leer la cmdline de task->mm (si existe). Retorna 0 si se leyó algo. */
static int read_task_cmdline(struct task_struct *task, char *buf, int bufsize)
{
    struct mm_struct *mm;
    int ret = 0;
    unsigned long len;
    if (!task)
        return -1;

    rcu_read_lock();
    mm = get_task_mm(task); 
    rcu_read_unlock();

    if (!mm)
        return -1;

    /* mm->arg_start/arg_end indican la región de cmdline */
    if (mm->arg_end > mm->arg_start) {
        len = mm->arg_end - mm->arg_start;
        if (len > (unsigned long)(bufsize - 1))
            len = bufsize - 1;

        /* access_process_vm permite leer la memoria de otro proceso desde kernel */
        ret = access_process_vm(task, mm->arg_start, buf, len, 0);
        if (ret > 0) {
            /* cmdline tiene '\0' entre argumentos; reemplazarlos por espacios */
            int i;
            for (i = 0; i < ret; ++i)
                if (buf[i] == '\0')
                    buf[i] = ' ';
            buf[ret] = '\0';
        } else {
            buf[0] = '\0';
            ret = -1;
        }
    } else {
        buf[0] = '\0';
        ret = -1;
    }

    mmput(mm); 
    return ret;
}



static int continfo_so1_201801521_show(struct seq_file *m, void *v)
{
    struct task_struct *task;
    struct sysinfo si;
    unsigned long total_ram_kb, free_ram_kb, used_ram_kb;
    struct timespec64 ts;
    unsigned long long ts_ms;

    si_meminfo(&si);
    total_ram_kb = (si.totalram * si.mem_unit) / 1024;
    free_ram_kb  = (si.freeram * si.mem_unit) / 1024;
    used_ram_kb  = (total_ram_kb > free_ram_kb) ? (total_ram_kb - free_ram_kb) : 0;

    ktime_get_real_ts64(&ts);
    ts_ms = (unsigned long long)ts.tv_sec * 1000ULL +
            (unsigned long long)(ts.tv_nsec / 1000000ULL);

    seq_printf(m, "{\n");
    seq_printf(m, "  \"total_ram_kb\": %lu,\n", total_ram_kb);
    seq_printf(m, "  \"free_ram_kb\": %lu,\n", free_ram_kb);
    seq_printf(m, "  \"used_ram_kb\": %lu,\n", used_ram_kb);
    seq_printf(m, "  \"ts_ms\": %llu,\n", ts_ms);
    seq_printf(m, "  \"procesos\": [\n");

    rcu_read_lock();
    {
        bool first = true;

        for_each_process(task) {
    char cmdline[CMDLINE_MAX];
    unsigned long vsz_kb = 0;
    unsigned long rss_kb = 0;
    unsigned long mem_percent = 0;
    const char *state_s = "U";   
    int is_container_related = 0;
    u64 utime_ns = 0, stime_ns = 0, cpu_time_ns = 0;

    if (read_task_cmdline(task, cmdline, sizeof(cmdline)) < 0)
        cmdline[0] = '\0';

    if (strnstr(cmdline, "docker", sizeof(cmdline)) ||
        strnstr(cmdline, "containerd", sizeof(cmdline)) ||
        strnstr(cmdline, "runc", sizeof(cmdline)) ||
        strnstr(cmdline, "podman", sizeof(cmdline)) ||
        strnstr(cmdline, "kubepods", sizeof(cmdline))) {
        is_container_related = 1;
    }

    if (task->mm) {
        struct mm_struct *mm = task->mm;
        vsz_kb = (mm->total_vm * PAGE_SIZE) / 1024;
        {
            long rss_pages = get_mm_rss(mm);
            if (rss_pages > 0)
                rss_kb = (rss_pages * PAGE_SIZE) / 1024;
        }
    }

    if (total_ram_kb > 0)
        mem_percent = (rss_kb * 100) / total_ram_kb;

    task_cputime_adjusted(task, &utime_ns, &stime_ns);
    cpu_time_ns = utime_ns + stime_ns;

    if (!first)
        seq_puts(m, ",\n");
    else
        first = false;

    seq_printf(m,
        "    { \"pid\": %d, "
        "\"nombre\": \"%s\", "
        "\"cmdline_or_container_id\": \"%s\", "
        "\"vsz_kb\": %lu, "
        "\"rss_kb\": %lu, "
        "\"mem_percent\": %lu, "
        "\"cpu_time_ns\": %llu, "
        "\"estado\": \"%s\", "
        "\"container_related\": \"%s\" }",
        task->pid,
        task->comm,
        cmdline,
        vsz_kb,
        rss_kb,
        mem_percent,
        cpu_time_ns,
        state_s,
        is_container_related ? "yes" : "no");
}

    }
    rcu_read_unlock();

    seq_printf(m, "\n  ]\n");
    seq_printf(m, "}\n");
    return 0;
}


static int continfo_so1_201801521_open(struct inode *inode, struct file *file)
{
    return single_open(file, continfo_so1_201801521_show, NULL);
}

static const struct proc_ops continfo_so1_201801521_proc_ops = {
    .proc_open    = continfo_so1_201801521_open,
    .proc_read    = seq_read,
    .proc_lseek   = seq_lseek,
    .proc_release = single_release,
};


static int __init continfo_so1_201801521_init(void)
{
    proc_entry = proc_create(PROC_NAME, 0444, NULL, &continfo_so1_201801521_proc_ops);
    if (!proc_entry) {
        pr_err("No se pudo crear /proc/%s\n", PROC_NAME);
        return -ENOMEM;
    }
    pr_info("Modulo procesos cargado: /proc/%s\n", PROC_NAME);
    return 0;
}

static void __exit continfo_so1_201801521_exit(void)
{
    proc_remove(proc_entry);
    pr_info("Modulo procesos descargado\n");
}

module_init(continfo_so1_201801521_init);
module_exit(continfo_so1_201801521_exit);