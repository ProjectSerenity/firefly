.PHONY: all
all:
	$(MAKE) -C Pure64
	$(MAKE) -C kcheck
	$(MAKE) -C kernel
