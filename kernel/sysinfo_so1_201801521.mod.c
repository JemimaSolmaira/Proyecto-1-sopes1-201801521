#include <linux/module.h>
#include <linux/export-internal.h>
#include <linux/compiler.h>

MODULE_INFO(name, KBUILD_MODNAME);

__visible struct module __this_module
__section(".gnu.linkonce.this_module") = {
	.name = KBUILD_MODNAME,
	.init = init_module,
#ifdef CONFIG_MODULE_UNLOAD
	.exit = cleanup_module,
#endif
	.arch = MODULE_ARCH_INIT,
};



static const struct modversion_info ____versions[]
__used __section("__versions") = {
	{ 0x003b23f9, "single_open" },
	{ 0xbd03ed67, "random_kmalloc_seed" },
	{ 0xfed1e3bc, "kmalloc_caches" },
	{ 0x70db3fe4, "__kmalloc_cache_noprof" },
	{ 0x318e45ff, "filp_open" },
	{ 0x4cd313ad, "kernel_read" },
	{ 0xdb9a5310, "filp_close" },
	{ 0x173ec8da, "sscanf" },
	{ 0xcb8b6ec6, "kfree" },
	{ 0xd272d446, "__stack_chk_fail" },
	{ 0x9487f592, "proc_remove" },
	{ 0xa5c7582d, "strsep" },
	{ 0x680628e7, "ktime_get_real_ts64" },
	{ 0xd272d446, "__rcu_read_lock" },
	{ 0x1790826a, "init_task" },
	{ 0xd272d446, "__rcu_read_unlock" },
	{ 0xf2c4f3f1, "seq_printf" },
	{ 0x397daafe, "mmput" },
	{ 0xbc2df032, "task_cputime_adjusted" },
	{ 0x9479a1e8, "strnlen" },
	{ 0xd70733be, "sized_strscpy" },
	{ 0xe8d8d116, "get_task_mm" },
	{ 0xe54e0a6b, "__fortify_panic" },
	{ 0xbd4e501f, "seq_read" },
	{ 0xfc8fa4ce, "seq_lseek" },
	{ 0xcb077514, "single_release" },
	{ 0xd272d446, "__fentry__" },
	{ 0x82c6f73b, "proc_create" },
	{ 0xe8213e80, "_printk" },
	{ 0xd272d446, "__x86_return_thunk" },
	{ 0xba157484, "module_layout" },
};

static const u32 ____version_ext_crcs[]
__used __section("__version_ext_crcs") = {
	0x003b23f9,
	0xbd03ed67,
	0xfed1e3bc,
	0x70db3fe4,
	0x318e45ff,
	0x4cd313ad,
	0xdb9a5310,
	0x173ec8da,
	0xcb8b6ec6,
	0xd272d446,
	0x9487f592,
	0xa5c7582d,
	0x680628e7,
	0xd272d446,
	0x1790826a,
	0xd272d446,
	0xf2c4f3f1,
	0x397daafe,
	0xbc2df032,
	0x9479a1e8,
	0xd70733be,
	0xe8d8d116,
	0xe54e0a6b,
	0xbd4e501f,
	0xfc8fa4ce,
	0xcb077514,
	0xd272d446,
	0x82c6f73b,
	0xe8213e80,
	0xd272d446,
	0xba157484,
};
static const char ____version_ext_names[]
__used __section("__version_ext_names") =
	"single_open\0"
	"random_kmalloc_seed\0"
	"kmalloc_caches\0"
	"__kmalloc_cache_noprof\0"
	"filp_open\0"
	"kernel_read\0"
	"filp_close\0"
	"sscanf\0"
	"kfree\0"
	"__stack_chk_fail\0"
	"proc_remove\0"
	"strsep\0"
	"ktime_get_real_ts64\0"
	"__rcu_read_lock\0"
	"init_task\0"
	"__rcu_read_unlock\0"
	"seq_printf\0"
	"mmput\0"
	"task_cputime_adjusted\0"
	"strnlen\0"
	"sized_strscpy\0"
	"get_task_mm\0"
	"__fortify_panic\0"
	"seq_read\0"
	"seq_lseek\0"
	"single_release\0"
	"__fentry__\0"
	"proc_create\0"
	"_printk\0"
	"__x86_return_thunk\0"
	"module_layout\0"
;

MODULE_INFO(depends, "");


MODULE_INFO(srcversion, "B3552633EADB938E72B43BF");
