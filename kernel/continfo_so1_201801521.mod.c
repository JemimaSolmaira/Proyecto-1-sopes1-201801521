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
	{ 0x9487f592, "proc_remove" },
	{ 0xc7ffe1aa, "si_meminfo" },
	{ 0x680628e7, "ktime_get_real_ts64" },
	{ 0xf2c4f3f1, "seq_printf" },
	{ 0xd272d446, "__rcu_read_lock" },
	{ 0x1790826a, "init_task" },
	{ 0xe8d8d116, "get_task_mm" },
	{ 0xd272d446, "__rcu_read_unlock" },
	{ 0x397daafe, "mmput" },
	{ 0xb689121e, "strnstr" },
	{ 0xbc2df032, "task_cputime_adjusted" },
	{ 0x08fc6844, "seq_write" },
	{ 0x04e8afba, "access_process_vm" },
	{ 0xd272d446, "__stack_chk_fail" },
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
	0x9487f592,
	0xc7ffe1aa,
	0x680628e7,
	0xf2c4f3f1,
	0xd272d446,
	0x1790826a,
	0xe8d8d116,
	0xd272d446,
	0x397daafe,
	0xb689121e,
	0xbc2df032,
	0x08fc6844,
	0x04e8afba,
	0xd272d446,
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
	"proc_remove\0"
	"si_meminfo\0"
	"ktime_get_real_ts64\0"
	"seq_printf\0"
	"__rcu_read_lock\0"
	"init_task\0"
	"get_task_mm\0"
	"__rcu_read_unlock\0"
	"mmput\0"
	"strnstr\0"
	"task_cputime_adjusted\0"
	"seq_write\0"
	"access_process_vm\0"
	"__stack_chk_fail\0"
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


MODULE_INFO(srcversion, "9D8066D39E8392FAD2DCCC3");
