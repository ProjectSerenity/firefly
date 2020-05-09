extern void kmain(void);

void _start(void);

void _start() {
	kmain();

	for (;;) {
		__asm__ ("hlt");
	}
}
